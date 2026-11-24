package gcs

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func generateTestServiceAccountJSON(t *testing.T) []byte {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating rsa key: %v", err)
	}
	der := x509.MarshalPKCS1PrivateKey(priv)
	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: der,
	})
	keyJSON, err := json.Marshal(serviceAccountKey{
		ClientEmail: "taite-storage@taite-board.iam.gserviceaccount.com",
		PrivateKey:  string(pemBytes),
		TokenURI:    "https://oauth2.test/token",
	})
	if err != nil {
		t.Fatalf("marshaling key json: %v", err)
	}
	return keyJSON
}

func TestClient_GetPutListAndTokenCache(t *testing.T) {
	keyJSON := generateTestServiceAccountJSON(t)
	client, err := NewClientFromJSON(keyJSON)
	if err != nil {
		t.Fatalf("NewClientFromJSON failed: %v", err)
	}

	tokenCalls := 0
	var lastPutBody string
	var lastPutCT string

	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch {
			case r.URL.String() == "https://oauth2.test/token":
				tokenCalls++
				body, _ := io.ReadAll(r.Body)
				if !strings.Contains(string(body), "assertion=") {
					t.Errorf("expected jwt assertion in token body, got %s", string(body))
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"access_token":"tok-123","expires_in":3600}`)),
					Header:     make(http.Header),
				}, nil

			case strings.Contains(r.URL.Path, "/storage/v1/b/taite-board-data/o/data%2Fgroups%2Fribonclock%2Fschedule.json") ||
				strings.Contains(r.URL.RawPath, "data%2Fgroups%2Fribonclock%2Fschedule.json") ||
				strings.HasSuffix(r.URL.Path, "/o/data/groups/ribonclock/schedule.json"):
				if r.Header.Get("Authorization") != "Bearer tok-123" {
					t.Errorf("unexpected auth header: %s", r.Header.Get("Authorization"))
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"sessions":[]}`)),
					Header:     make(http.Header),
				}, nil

			case strings.Contains(r.URL.Path, "/o/missing.json"):
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Body:       io.NopCloser(strings.NewReader(`{"error":{"code":404}}`)),
					Header:     make(http.Header),
				}, nil

			case strings.Contains(r.URL.Path, "/upload/storage/v1/b/taite-board-data/o"):
				lastPutCT = r.Header.Get("Content-Type")
				b, _ := io.ReadAll(r.Body)
				lastPutBody = string(b)
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"name":"schedule.json"}`)),
					Header:     make(http.Header),
				}, nil

			case strings.HasSuffix(r.URL.Path, "/storage/v1/b/taite-board-data/o"):
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"items":[{"name":"data/groups/ribonclock/schedule.json"},{"name":"schedule.json"}]}`)),
					Header:     make(http.Header),
				}, nil

			default:
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
				return nil, nil
			}
		}),
	}

	now := time.Unix(1700000000, 0)
	client.nowFunc = func() time.Time { return now }

	ctx := context.Background()

	got, err := client.GetObject(ctx, "taite-board-data", "data/groups/ribonclock/schedule.json")
	if err != nil {
		t.Fatalf("GetObject failed: %v", err)
	}
	if string(got) != `{"sessions":[]}` {
		t.Errorf("unexpected object body: %s", string(got))
	}

	_, err = client.GetObject(ctx, "taite-board-data", "missing.json")
	if !errors.Is(err, ErrObjectNotFound) {
		t.Errorf("expected ErrObjectNotFound, got %v", err)
	}

	err = client.PutObject(ctx, "taite-board-data", "schedule.json", "application/json; charset=utf-8", "no-cache, max-age=0", []byte(`{"ok":true}`))
	if err != nil {
		t.Fatalf("PutObject failed: %v", err)
	}
	if !strings.HasPrefix(lastPutCT, "multipart/related; boundary=") {
		t.Errorf("expected multipart/related content-type, got %s", lastPutCT)
	}
	if !strings.Contains(lastPutBody, `"cacheControl":"no-cache, max-age=0"`) {
		t.Errorf("expected cacheControl metadata in multipart body, got %s", lastPutBody)
	}
	if !strings.Contains(lastPutBody, `{"ok":true}`) {
		t.Errorf("expected payload in multipart body, got %s", lastPutBody)
	}

	items, err := client.ListObjects(ctx, "taite-board-data", "data/")
	if err != nil {
		t.Fatalf("ListObjects failed: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %v", items)
	}

	// Token should have been cached across all 4 calls
	if tokenCalls != 1 {
		t.Errorf("expected 1 token call due to caching, got %d", tokenCalls)
	}

	// Advance time past expiry margin (56 minutes later) -> should refresh token
	now = now.Add(56 * time.Minute)
	_, err = client.GetObject(ctx, "taite-board-data", "data/groups/ribonclock/schedule.json")
	if err != nil {
		t.Fatalf("GetObject after expiry failed: %v", err)
	}
	if tokenCalls != 2 {
		t.Errorf("expected token refresh after expiry, got %d calls", tokenCalls)
	}
}

func TestNewClientFromEnv_Unset(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "")
	t.Setenv("GCS_CREDENTIALS_FILE", "")
	t.Setenv("GCS_CREDENTIALS_JSON", "")

	c, err := NewClientFromEnv()
	if err != nil {
		t.Fatalf("expected nil error when unset, got %v", err)
	}
	if c != nil {
		t.Fatalf("expected nil client when unset, got %+v", c)
	}

	// Verify GCS_CREDENTIALS_JSON works
	keyJSON := generateTestServiceAccountJSON(t)
	t.Setenv("GCS_CREDENTIALS_JSON", string(bytes.TrimSpace(keyJSON)))
	c, err = NewClientFromEnv()
	if err != nil {
		t.Fatalf("expected valid client from GCS_CREDENTIALS_JSON, got %v", err)
	}
	if c == nil || c.clientEmail != "taite-storage@taite-board.iam.gserviceaccount.com" {
		t.Errorf("unexpected client: %+v", c)
	}
}

func TestClient_MetadataServerAndCustomScope(t *testing.T) {
	calls := 0
	metaClient := NewClientFromMetadata("http://metadata.test/token")
	metaClient.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			calls++
			if r.Header.Get("Metadata-Flavor") != "Google" {
				t.Errorf("expected Metadata-Flavor: Google header, got %q", r.Header.Get("Metadata-Flavor"))
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"access_token":"meta-tok-999","expires_in":3600}`)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	tok1, err := metaClient.TokenForScope(context.Background(), defaultScope)
	if err != nil {
		t.Fatalf("TokenForScope metadata failed: %v", err)
	}
	tok2, err := metaClient.TokenForScope(context.Background(), "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		t.Fatalf("TokenForScope cached metadata failed: %v", err)
	}
	if tok1 != "meta-tok-999" || tok2 != "meta-tok-999" {
		t.Errorf("unexpected tokens: %q, %q", tok1, tok2)
	}
	if calls != 1 {
		t.Errorf("expected cached metadata token (1 call), got %d", calls)
	}

	// Verify SA JSON client acquires distinct cached tokens per scope
	keyJSON := generateTestServiceAccountJSON(t)
	saClient, err := NewClientFromJSON(keyJSON)
	if err != nil {
		t.Fatalf("NewClientFromJSON failed: %v", err)
	}
	scopeCalls := 0
	saClient.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			scopeCalls++
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"access_token":"sa-tok-` + string(rune('0'+scopeCalls)) + `","expires_in":3600}`)),
				Header:     make(http.Header),
			}, nil
		}),
	}
	s1, err := saClient.TokenForScope(context.Background(), defaultScope)
	if err != nil {
		t.Fatalf("default scope token failed: %v", err)
	}
	s2, err := saClient.TokenForScope(context.Background(), "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		t.Fatalf("cloud-platform scope token failed: %v", err)
	}
	if s1 == s2 || scopeCalls != 2 {
		t.Errorf("expected distinct token per scope (2 calls), got (%q, %q) calls=%d", s1, s2, scopeCalls)
	}
}
