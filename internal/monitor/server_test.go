package monitor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AobaIwaki123/taite-board/internal/candidate"
	"github.com/AobaIwaki123/taite-board/internal/parser"
	"github.com/AobaIwaki123/taite-board/internal/raw"
)

type mockFetcher struct {
	post *raw.Post
	err  error
}

func (m *mockFetcher) FetchPost(ctx context.Context, postURL string) (*raw.Post, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.post, nil
}

type mockResolver struct {
	cand      candidate.EventCandidate
	evidences []candidate.Evidence
	needsAI   bool
}

func (m *mockResolver) Resolve(text string, images []string) (candidate.EventCandidate, []candidate.Evidence, bool) {
	return m.cand, m.evidences, m.needsAI
}

func TestServer_Healthz(t *testing.T) {
	collector := NewCollector("test-v1")
	handler := NewHandler(collector)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if rec.Body.String() != "ok\n" {
		t.Fatalf("expected 'ok\n', got %q", rec.Body.String())
	}
}

func TestServer_Intake_Success(t *testing.T) {
	collector := NewCollector("test-v1")
	fetcher := &mockFetcher{
		post: &raw.Post{
			Source:   "x",
			SourceID: "99887766",
			URL:      "https://x.com/Ribbon_Clock/status/99887766",
			Author:   "Ribbon Clock",
			Text:     "11/07 渋谷RING LIVE 18:30-19:00",
			Images:   []string{"https://pbs.twimg.com/media/test.jpg"},
		},
	}
	resolver := &mockResolver{
		cand: candidate.EventCandidate{
			Date:             "2026-11-07",
			VenueRaw:         "渋谷RING",
			VenueCanonicalID: "venue:shibuya-ring",
			GroupCanonicalID: "group:ribbon-clock",
			LiveStartRaw:     "18:30",
			LiveEndRaw:       "19:00",
		},
		needsAI: false,
	}

	handler := NewHandlerWithDependencies(collector, fetcher, resolver)

	body, _ := json.Marshal(map[string]string{
		"url": "https://x.com/Ribbon_Clock/status/99887766",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/intake", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var res candidate.IntakeResult
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if res.SourceID != "99887766" {
		t.Errorf("expected source_id 99887766, got %s", res.SourceID)
	}
	if res.NeedsAI != false {
		t.Errorf("expected needs_ai=false, got true")
	}
	if res.Candidate.VenueCanonicalID != "venue:shibuya-ring" {
		t.Errorf("expected venue:shibuya-ring, got %s", res.Candidate.VenueCanonicalID)
	}
	if len(res.RawFingerprint) != 64 {
		t.Errorf("expected 64-char fingerprint, got %s", res.RawFingerprint)
	}

	// Verify intake was recorded in collector
	recent := collector.RecentIntakes()
	if len(recent) != 1 {
		t.Fatalf("expected 1 recent intake in collector, got %d", len(recent))
	}
	if recent[0].SourceID != "99887766" {
		t.Errorf("expected recorded source_id 99887766, got %s", recent[0].SourceID)
	}
}

func TestServer_Intake_FetchError(t *testing.T) {
	collector := NewCollector("test-v1")
	fetcher := &mockFetcher{
		err: errors.New("oembed returned 404"),
	}
	resolver := &mockResolver{}

	handler := NewHandlerWithDependencies(collector, fetcher, resolver)

	body, _ := json.Marshal(map[string]string{
		"url": "https://x.com/invalid/status/123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/intake", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected status 422, got %d", rec.Code)
	}
}

func TestServer_AIParse_Mock(t *testing.T) {
	collector := NewCollector("test-v1")
	handler := NewHandler(collector)

	body, _ := json.Marshal(map[string]any{
		"source_id": "12345",
		"text":      "タイテ画像です",
		"images":    []string{"https://pbs.twimg.com/media/test.jpg"},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/intake/ai-parse", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var res parser.ParseResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if res.Candidate.LiveStartRaw != "18:30" {
		t.Errorf("expected 18:30, got %s", res.Candidate.LiveStartRaw)
	}
	if res.Status != candidate.StatusCompleted {
		t.Errorf("expected completed, got %s", res.Status)
	}
}
