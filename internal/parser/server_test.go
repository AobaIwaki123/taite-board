package parser

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AobaIwaki123/taite-board/internal/candidate"
)

type mockVisionParser struct {
	resp *ParseResponse
	err  error
}

func (m *mockVisionParser) Parse(ctx context.Context, req *ParseRequest) (*ParseResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.resp, nil
}

func TestServer_Healthz(t *testing.T) {
	srv := NewServer(&mockVisionParser{}, "")
	handler := srv.Handler()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != "ok\n" {
		t.Fatalf("expected 'ok\n', got %q", rec.Body.String())
	}
}

func TestServer_Auth(t *testing.T) {
	srv := NewServer(&mockVisionParser{}, "secret-token-123")
	handler := srv.Handler()

	// Missing token -> 401
	body, _ := json.Marshal(ParseRequest{Text: "test"})
	req := httptest.NewRequest(http.MethodPost, "/parse", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 unauthorized, got %d", rec.Code)
	}

	// Valid Bearer token
	reqAuth := httptest.NewRequest(http.MethodPost, "/parse", bytes.NewReader(body))
	reqAuth.Header.Set("Authorization", "Bearer secret-token-123")
	recAuth := httptest.NewRecorder()
	handler.ServeHTTP(recAuth, reqAuth)

	if recAuth.Code == http.StatusUnauthorized {
		t.Fatalf("expected authorization to succeed with valid token")
	}
}

func TestServer_Parse_Success(t *testing.T) {
	mockParser := &mockVisionParser{
		resp: &ParseResponse{
			RequestID: "req-1",
			Status:    candidate.StatusCompleted,
			Candidate: candidate.EventCandidate{
				Date:         "2026-11-07",
				VenueRaw:     "渋谷RING",
				LiveStartRaw: "18:30",
				LiveEndRaw:   "19:00",
			},
			Evidence: []candidate.Evidence{
				{
					Field:      "live_start_raw",
					Source:     "image",
					Quote:      "18:30",
					Confidence: 0.98,
				},
			},
		},
	}

	srv := NewServer(mockParser, "")
	handler := srv.Handler()

	body, _ := json.Marshal(ParseRequest{
		RequestID: "req-1",
		Text:      "11/7 渋谷RING",
		Media: []MediaItem{
			{InputURL: "https://example.com/timetable.jpg"},
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/parse", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp ParseResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Candidate.LiveStartRaw != "18:30" {
		t.Errorf("expected live_start_raw 18:30, got %s", resp.Candidate.LiveStartRaw)
	}
	if len(resp.Evidence) != 1 {
		t.Errorf("expected 1 evidence item, got %d", len(resp.Evidence))
	}
}

func TestServer_Parse_Error(t *testing.T) {
	mockParser := &mockVisionParser{
		err: errors.New("gemini rate limit exceeded"),
	}

	srv := NewServer(mockParser, "")
	handler := srv.Handler()

	body, _ := json.Marshal(ParseRequest{
		RequestID: "req-err",
	})
	req := httptest.NewRequest(http.MethodPost, "/parse", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}

	var resp ParseResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Status != candidate.StatusFailedRetryable {
		t.Errorf("expected failed_retryable status, got %s", resp.Status)
	}
}
