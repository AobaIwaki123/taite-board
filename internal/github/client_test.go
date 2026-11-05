package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestClientUpdateFile(t *testing.T) {
	var gotAuthorization string
	var gotRequest putRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthorization = r.Header.Get("Authorization")
		if r.Method == http.MethodGet {
			if r.URL.Path != "/repos/owner/repo/contents/data/groups/ribonclock/schedule.json" {
				t.Fatalf("unexpected GET path: %s", r.URL.Path)
			}
			content := base64.StdEncoding.EncodeToString([]byte("old\n"))
			_ = json.NewEncoder(w).Encode(contentsResponse{
				Path:     "data/groups/ribonclock/schedule.json",
				SHA:      "old-sha",
				Content:  content,
				Encoding: "base64",
			})
			return
		}
		if r.Method != http.MethodPut {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(body, &gotRequest); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"content":{"path":"data/groups/ribonclock/schedule.json","sha":"new-file-sha"},"commit":{"sha":"commit-sha","html_url":"https://github.com/owner/repo/commit/commit-sha"}}`)
	}))
	defer server.Close()

	client := NewClient(Config{
		Token:  "secret-token",
		Owner:  "owner",
		Repo:   "repo",
		APIURL: server.URL,
	})
	result, err := client.UpdateFile(context.Background(), "main", "data/groups/ribonclock/schedule.json", "test commit", func(current []byte) ([]byte, error) {
		return append(current, []byte("new\n")...), nil
	})
	if err != nil {
		t.Fatalf("UpdateFile failed: %v", err)
	}
	if gotAuthorization != "Bearer secret-token" {
		t.Errorf("unexpected authorization header: %q", gotAuthorization)
	}
	if gotRequest.Message != "test commit" || gotRequest.Branch != "main" || gotRequest.SHA != "old-sha" {
		t.Errorf("unexpected update request: %+v", gotRequest)
	}
	decoded, err := base64.StdEncoding.DecodeString(gotRequest.Content)
	if err != nil {
		t.Fatal(err)
	}
	if string(decoded) != "old\nnew\n" {
		t.Errorf("unexpected content: %q", decoded)
	}
	if result.SHA != "commit-sha" || result.URL == "" {
		t.Errorf("unexpected commit result: %+v", result)
	}
}

func TestClientUpdateFileRetriesConflictWithFreshContent(t *testing.T) {
	var gets atomic.Int32
	var puts atomic.Int32
	var seenSHA []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			count := gets.Add(1)
			content := "first\n"
			sha := "sha-first"
			if count > 1 {
				content = "concurrent\n"
				sha = "sha-concurrent"
			}
			_ = json.NewEncoder(w).Encode(contentsResponse{
				Path:     "schedule.json",
				SHA:      sha,
				Content:  base64.StdEncoding.EncodeToString([]byte(content)),
				Encoding: "base64",
			})
		case http.MethodPut:
			count := puts.Add(1)
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			var request putRequest
			if err := json.Unmarshal(body, &request); err != nil {
				t.Fatal(err)
			}
			seenSHA = append(seenSHA, request.SHA)
			if count == 1 {
				w.WriteHeader(http.StatusConflict)
				_, _ = io.WriteString(w, `{"message":"sha conflict"}`)
				return
			}
			_, _ = io.WriteString(w, `{"content":{"path":"schedule.json","sha":"new-sha"},"commit":{"sha":"commit-sha"}}`)
		default:
			t.Fatalf("unexpected method: %s", r.Method)
		}
	}))
	defer server.Close()

	var mutations []string
	client := NewClient(Config{Token: "token", Owner: "owner", Repo: "repo", APIURL: server.URL})
	_, err := client.UpdateFile(context.Background(), "main", "schedule.json", "retry", func(current []byte) ([]byte, error) {
		mutations = append(mutations, string(current))
		return append(current, []byte("updated\n")...), nil
	})
	if err != nil {
		t.Fatalf("UpdateFile failed: %v", err)
	}
	if gets.Load() != 2 || puts.Load() != 2 {
		t.Fatalf("expected two GETs and PUTs, got %d GETs and %d PUTs", gets.Load(), puts.Load())
	}
	if strings.Join(mutations, "|") != "first\n|concurrent\n" {
		t.Errorf("mutation did not receive fresh content: %q", mutations)
	}
	if strings.Join(seenSHA, "|") != "sha-first|sha-concurrent" {
		t.Errorf("unexpected optimistic-lock SHAs: %q", seenSHA)
	}
}

func TestClientUpdateFileNotFoundIsReported(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"message":"Not Found"}`)
	}))
	defer server.Close()

	client := NewClient(Config{Token: "token", Owner: "owner", Repo: "repo", APIURL: server.URL})
	_, err := client.UpdateFile(context.Background(), "main", "schedule.json", "create", func(current []byte) ([]byte, error) {
		if current != nil {
			t.Errorf("expected nil content for a missing file, got %q", current)
		}
		return []byte("new"), nil
	})
	if err == nil || !IsNotFound(err) {
		t.Fatalf("expected not found error, got %v", err)
	}
}
