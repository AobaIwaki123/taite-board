package gcs

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	defaultAPIBaseURL = "https://storage.googleapis.com"
	defaultScope      = "https://www.googleapis.com/auth/devstorage.read_write"
	defaultTokenURI   = "https://oauth2.googleapis.com/token"
)

// ErrObjectNotFound is returned when the requested object does not exist in GCS.
var ErrObjectNotFound = errors.New("gcs object not found")

// ObjectStore abstracts GCS object operations for production and testability.
type ObjectStore interface {
	GetObject(ctx context.Context, bucket, object string) ([]byte, error)
	PutObject(ctx context.Context, bucket, object, contentType, cacheControl string, data []byte) error
	ListObjects(ctx context.Context, bucket, prefix string) ([]string, error)
}

type serviceAccountKey struct {
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
	TokenURI    string `json:"token_uri"`
}

// Client is a lightweight Google Cloud Storage JSON API client using only the Go standard library.
type Client struct {
	clientEmail string
	privateKey  *rsa.PrivateKey
	tokenURI    string
	apiBaseURL  string
	httpClient  *http.Client
	nowFunc     func() time.Time

	mu          sync.Mutex
	cachedToken string
	tokenExpiry time.Time
}

// NewClientFromEnv creates a GCS client from environment variables:
// GOOGLE_APPLICATION_CREDENTIALS, GCS_CREDENTIALS_FILE, or GCS_CREDENTIALS_JSON.
// Returns (nil, nil) when no credentials are configured.
func NewClientFromEnv() (*Client, error) {
	if rawJSON := strings.TrimSpace(os.Getenv("GCS_CREDENTIALS_JSON")); rawJSON != "" {
		return NewClientFromJSON([]byte(rawJSON))
	}

	credFile := strings.TrimSpace(os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"))
	if credFile == "" {
		credFile = strings.TrimSpace(os.Getenv("GCS_CREDENTIALS_FILE"))
	}
	if credFile == "" {
		return nil, nil
	}

	raw, err := os.ReadFile(credFile)
	if err != nil {
		return nil, fmt.Errorf("reading GCS credentials file %s: %w", credFile, err)
	}
	return NewClientFromJSON(raw)
}

// NewClientFromJSON parses a GCP service account JSON key and constructs a Client.
func NewClientFromJSON(keyJSON []byte) (*Client, error) {
	var key serviceAccountKey
	if err := json.Unmarshal(keyJSON, &key); err != nil {
		return nil, fmt.Errorf("parsing service account json: %w", err)
	}
	if key.ClientEmail == "" || key.PrivateKey == "" {
		return nil, fmt.Errorf("service account json missing client_email or private_key")
	}

	privKey, err := parseRSAPrivateKey(key.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("parsing private_key: %w", err)
	}

	tokenURI := key.TokenURI
	if tokenURI == "" {
		tokenURI = defaultTokenURI
	}

	return &Client{
		clientEmail: key.ClientEmail,
		privateKey:  privKey,
		tokenURI:    tokenURI,
		apiBaseURL:  defaultAPIBaseURL,
		httpClient:  &http.Client{Timeout: 20 * time.Second},
		nowFunc:     time.Now,
	}, nil
}

func parseRSAPrivateKey(pemStr string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("invalid PEM block")
	}
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("private key is not RSA")
		}
		return rsaKey, nil
	}
	if rsaKey, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return rsaKey, nil
	}
	return nil, fmt.Errorf("unsupported private key format")
}

// GetObject downloads an object from the specified GCS bucket.
func (c *Client) GetObject(ctx context.Context, bucket, object string) ([]byte, error) {
	if bucket == "" || object == "" {
		return nil, fmt.Errorf("bucket and object are required")
	}
	token, err := c.accessToken(ctx)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("%s/storage/v1/b/%s/o/%s?alt=media",
		strings.TrimRight(c.apiBaseURL, "/"),
		url.PathEscape(bucket),
		url.PathEscape(object),
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("creating get request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing get object: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading get response: %w", err)
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrObjectNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("gcs get %s/%s failed (%d): %s", bucket, object, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, nil
}

// PutObject uploads data and metadata (Content-Type, Cache-Control) via GCS multipart upload.
func (c *Client) PutObject(ctx context.Context, bucket, object, contentType, cacheControl string, data []byte) error {
	if bucket == "" || object == "" {
		return fmt.Errorf("bucket and object are required")
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	token, err := c.accessToken(ctx)
	if err != nil {
		return err
	}

	meta := map[string]string{
		"name":        object,
		"contentType": contentType,
	}
	if cacheControl != "" {
		meta["cacheControl"] = cacheControl
	}
	metaBytes, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshaling object metadata: %w", err)
	}

	var bodyBuf bytes.Buffer
	mw := multipart.NewWriter(&bodyBuf)

	metaHeader := make(textproto.MIMEHeader)
	metaHeader.Set("Content-Type", "application/json; charset=UTF-8")
	metaPart, err := mw.CreatePart(metaHeader)
	if err != nil {
		return fmt.Errorf("creating metadata part: %w", err)
	}
	if _, err := metaPart.Write(metaBytes); err != nil {
		return fmt.Errorf("writing metadata part: %w", err)
	}

	mediaHeader := make(textproto.MIMEHeader)
	mediaHeader.Set("Content-Type", contentType)
	mediaPart, err := mw.CreatePart(mediaHeader)
	if err != nil {
		return fmt.Errorf("creating media part: %w", err)
	}
	if _, err := mediaPart.Write(data); err != nil {
		return fmt.Errorf("writing media part: %w", err)
	}
	if err := mw.Close(); err != nil {
		return fmt.Errorf("closing multipart writer: %w", err)
	}

	endpoint := fmt.Sprintf("%s/upload/storage/v1/b/%s/o?uploadType=multipart",
		strings.TrimRight(c.apiBaseURL, "/"),
		url.PathEscape(bucket),
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &bodyBuf)
	if err != nil {
		return fmt.Errorf("creating put request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "multipart/related; boundary="+mw.Boundary())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing put object: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("gcs put %s/%s failed (%d): %s", bucket, object, resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}

// ListObjects lists object names matching prefix in the specified GCS bucket.
func (c *Client) ListObjects(ctx context.Context, bucket, prefix string) ([]string, error) {
	if bucket == "" {
		return nil, fmt.Errorf("bucket is required")
	}
	token, err := c.accessToken(ctx)
	if err != nil {
		return nil, err
	}

	var names []string
	pageToken := ""
	for {
		u := fmt.Sprintf("%s/storage/v1/b/%s/o", strings.TrimRight(c.apiBaseURL, "/"), url.PathEscape(bucket))
		q := url.Values{}
		if prefix != "" {
			q.Set("prefix", prefix)
		}
		if pageToken != "" {
			q.Set("pageToken", pageToken)
		}
		if encoded := q.Encode(); encoded != "" {
			u += "?" + encoded
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, fmt.Errorf("creating list request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("executing list objects: %w", err)
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("reading list response: %w", readErr)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("gcs list %s failed (%d): %s", bucket, resp.StatusCode, strings.TrimSpace(string(body)))
		}

		var parsed struct {
			Items []struct {
				Name string `json:"name"`
			} `json:"items"`
			NextPageToken string `json:"nextPageToken"`
		}
		if err := json.Unmarshal(body, &parsed); err != nil {
			return nil, fmt.Errorf("parsing list response: %w", err)
		}
		for _, item := range parsed.Items {
			names = append(names, item.Name)
		}
		if parsed.NextPageToken == "" {
			break
		}
		pageToken = parsed.NextPageToken
	}

	return names, nil
}

func (c *Client) accessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.nowFunc()
	if c.cachedToken != "" && now.Add(5*time.Minute).Before(c.tokenExpiry) {
		return c.cachedToken, nil
	}

	assertion, err := c.signJWT(now)
	if err != nil {
		return "", fmt.Errorf("signing jwt assertion: %w", err)
	}

	form := url.Values{}
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:jwt-bearer")
	form.Set("assertion", assertion)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURI, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("creating token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("requesting oauth token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading token response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("oauth token request failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("decoding token response: %w", err)
	}
	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("empty access_token returned from oauth server")
	}

	ttl := time.Duration(tokenResp.ExpiresIn) * time.Second
	if ttl <= 0 {
		ttl = time.Hour
	}
	c.cachedToken = tokenResp.AccessToken
	c.tokenExpiry = now.Add(ttl)
	return c.cachedToken, nil
}

func (c *Client) signJWT(now time.Time) (string, error) {
	headerBytes, err := json.Marshal(map[string]string{
		"alg": "RS256",
		"typ": "JWT",
	})
	if err != nil {
		return "", err
	}

	claimsBytes, err := json.Marshal(map[string]any{
		"iss":   c.clientEmail,
		"scope": defaultScope,
		"aud":   c.tokenURI,
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	})
	if err != nil {
		return "", err
	}

	unsigned := base64.RawURLEncoding.EncodeToString(headerBytes) + "." + base64.RawURLEncoding.EncodeToString(claimsBytes)
	digest := sha256.Sum256([]byte(unsigned))
	sig, err := rsa.SignPKCS1v15(rand.Reader, c.privateKey, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}
