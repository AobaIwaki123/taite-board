package parser

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestGeminiClient_Parse_AIStudioMode(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "key=test-api-key") {
			t.Errorf("expected query param key=test-api-key, got %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(geminiResponse{
			Candidates: []struct {
				Content struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"content"`
			}{
				{
					Content: struct {
						Parts []struct {
							Text string `json:"text"`
						} `json:"parts"`
					}{
						Parts: []struct {
							Text string `json:"text"`
						}{
							{Text: `{"date":"2026-11-04","venue_raw":"渋谷VIDENT","live_start_raw":"18:35","live_end_raw":"18:55"}`},
						},
					},
				},
			},
		})
	}))
	defer mockServer.Close()

	client := NewGeminiClient("test-api-key", 5*time.Second)
	client.Endpoint = mockServer.URL

	resp, err := client.Parse(context.Background(), &ParseRequest{
		RequestID: "req-1",
		Text:      "11/4 渋谷VIDENT",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Candidate.Date != "2026-11-04" {
		t.Errorf("expected date 2026-11-04, got %s", resp.Candidate.Date)
	}
	if resp.Candidate.LiveStartRaw != "18:35" {
		t.Errorf("expected live_start_raw 18:35, got %s", resp.Candidate.LiveStartRaw)
	}
}

func TestGeminiClient_Parse_VertexAIKeylessMode(t *testing.T) {
	var capturedAuth string
	var capturedProject string

	mockVertexServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		capturedProject = r.Header.Get("X-Goog-User-Project")

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(geminiResponse{
			Candidates: []struct {
				Content struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"content"`
			}{
				{
					Content: struct {
						Parts []struct {
							Text string `json:"text"`
						} `json:"parts"`
					}{
						Parts: []struct {
							Text string `json:"text"`
						}{
							{Text: `{"date":"2026-11-04","venue_raw":"渋谷VIDENT","live_start_raw":"18:35","live_end_raw":"18:55"}`},
						},
					},
				},
			},
		})
	}))
	defer mockVertexServer.Close()

	os.Setenv("GOOGLE_OAUTH_ACCESS_TOKEN", "mock-vertex-access-token")
	defer os.Unsetenv("GOOGLE_OAUTH_ACCESS_TOKEN")

	client := NewGeminiClient("", 5*time.Second)
	client.Endpoint = mockVertexServer.URL
	client.ProjectID = "test-project-123"

	// Point client HTTPClient transport to mockVertexServer to intercept the Vertex AI request
	client.HTTPClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		req.URL.Scheme = "http"
		req.URL.Host = mockVertexServer.Listener.Addr().String()
		return http.DefaultTransport.RoundTrip(req)
	})

	resp, err := client.Parse(context.Background(), &ParseRequest{
		RequestID: "req-keyless",
		Text:      "11/4 渋谷VIDENT",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedAuth != "Bearer mock-vertex-access-token" {
		t.Errorf("expected Bearer mock-vertex-access-token, got %s", capturedAuth)
	}
	if capturedProject != "test-project-123" {
		t.Errorf("expected X-Goog-User-Project test-project-123, got %s", capturedProject)
	}
	if resp.Candidate.VenueRaw != "渋谷VIDENT" {
		t.Errorf("expected venue 渋谷VIDENT, got %s", resp.Candidate.VenueRaw)
	}
}

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestCleanJSONText(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"{\"key\": \"val\"}", "{\"key\": \"val\"}"},
		{"```json\n{\"key\": \"val\"}\n```", "{\"key\": \"val\"}"},
		{"```\n{\"key\": \"val\"}\n```", "{\"key\": \"val\"}"},
	}
	for _, tc := range cases {
		got := cleanJSONText(tc.input)
		if got != tc.expected {
			t.Errorf("cleanJSONText(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}
