package monitor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
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
	// Pre-populate an intake item waiting for AI
	collector.AddIntake(candidate.IntakeResult{
		Source:   "x",
		SourceID: "12345",
		Text:     "タイテ画像です",
		NeedsAI:  true,
		Status:   candidate.StatusNeedsReview,
	})

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

	// Verify intake in collector was updated with AI extracted parameters
	recent := collector.RecentIntakes()
	if len(recent) != 1 {
		t.Fatalf("expected 1 recent intake, got %d", len(recent))
	}
	if recent[0].Candidate.LiveStartRaw != "18:30" {
		t.Errorf("expected updated live start 18:30 in collector, got %s", recent[0].Candidate.LiveStartRaw)
	}
	if recent[0].NeedsAI != false {
		t.Errorf("expected needs_ai=false after AI parse")
	}
	if recent[0].Status != candidate.StatusCompleted {
		t.Errorf("expected status=completed after AI parse, got %s", recent[0].Status)
	}

	// Verify pipeline AI dispatched count
	snap := collector.Snapshot()
	if snap.Pipeline.AIDispatched != 1 {
		t.Errorf("expected AIDispatched=1 in snapshot pipeline, got %d", snap.Pipeline.AIDispatched)
	}
}

func TestServer_Intake_Promote(t *testing.T) {
	collector := NewCollector("test-v1")
	handler := NewHandlerWithDependencies(collector, nil, nil)

	// Create temp schedule file
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origWd) }()

	_ = os.MkdirAll("data/generated", 0755)
	_ = os.WriteFile("data/generated/schedule.json", []byte(`{"target_group":{"name":"test"},"sessions":[]}`), 0644)

	body, _ := json.Marshal(map[string]any{
		"group":         "ribonclock",
		"source_id":     "11223344",
		"source_url":    "https://x.com/test/status/11223344",
		"title":         "Girls Face in RING",
		"date":          "2026-11-21",
		"venue":         "渋谷RING",
		"live_start":    "20:20",
		"live_end":      "20:40",
		"benefit_start": "21:10",
		"benefit_end":   "22:10",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/intake/promote", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var res map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if res["status"] != "success" {
		t.Errorf("expected status=success, got %v", res["status"])
	}
	if res["domain"] != "ribonclock.iorin.net" {
		t.Errorf("expected domain=ribonclock.iorin.net, got %v", res["domain"])
	}
}

func TestServer_Intake_Patch(t *testing.T) {
	collector := NewCollector("test-v1")
	handler := NewHandlerWithDependencies(collector, nil, nil)

	// Create temp schedule file
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origWd) }()

	_ = os.MkdirAll("data/generated", 0755)
	initialSchedule := `{
		"target_group": {"name": "test"},
		"current_next_id": "session-test-01",
		"sessions": [
			{
				"id": "session-test-01",
				"date": "2026-11-21",
				"title": "Girls Face in RING",
				"venue": {"name": "渋谷RING"},
				"live": {"start_time": "20:20"},
				"benefit": {"start_time": "21:10"}
			}
		]
	}`
	_ = os.WriteFile("data/generated/schedule.json", []byte(initialSchedule), 0644)

	body, _ := json.Marshal(map[string]any{
		"group":           "ribonclock",
		"date":            "2026-11-21",
		"source_url":      "https://x.com/Ribbon_Clock/status/999888",
		"admission_bonus": "指名入場特典：宿題チェキ券",
		"photo_song":      "未来へのファンファーレ",
		"photo_device":    "video_ok",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/intake/patch", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var res map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if res["status"] != "success" {
		t.Errorf("expected status=success, got %v", res["status"])
	}
	if res["session_id"] != "session-test-01" {
		t.Errorf("expected session_id=session-test-01, got %v", res["session_id"])
	}

	// Verify updated schedule contents
	updatedBytes, err := os.ReadFile("data/generated/schedule.json")
	if err != nil {
		t.Fatalf("failed to read updated schedule: %v", err)
	}
	var doc struct {
		Sessions []struct {
			ID             string `json:"id"`
			AdmissionBonus string `json:"admission_bonus"`
			Photo          struct {
				AllowedSongs []string `json:"allowed_songs"`
				Device       string   `json:"device"`
			} `json:"photo"`
		} `json:"sessions"`
	}
	if err := json.Unmarshal(updatedBytes, &doc); err != nil {
		t.Fatalf("failed to parse updated json: %v", err)
	}
	if len(doc.Sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(doc.Sessions))
	}
	sess := doc.Sessions[0]
	if sess.AdmissionBonus != "指名入場特典：宿題チェキ券" {
		t.Errorf("expected admission bonus '指名入場特典：宿題チェキ券', got %q", sess.AdmissionBonus)
	}
	if len(sess.Photo.AllowedSongs) == 0 || sess.Photo.AllowedSongs[0] != "未来へのファンファーレ" {
		t.Errorf("expected allowed song '未来へのファンファーレ', got %v", sess.Photo.AllowedSongs)
	}
	if sess.Photo.Device != "video_ok" {
		t.Errorf("expected photo device 'video_ok', got %q", sess.Photo.Device)
	}
}
