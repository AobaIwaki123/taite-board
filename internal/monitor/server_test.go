package monitor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/AobaIwaki123/taite-board/internal/candidate"
	"github.com/AobaIwaki123/taite-board/internal/gcs"
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
	handler := NewHandler(collector)

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
	if res["action"] != "created" {
		t.Errorf("expected created, got %v", res["action"])
	}
	if res["domain"] != "ribonclock.iorin.net" {
		t.Errorf("expected domain=ribonclock.iorin.net, got %v", res["domain"])
	}

	post := func(data map[string]any) *httptest.ResponseRecorder {
		t.Helper()
		b, err := json.Marshal(data)
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRecorder()
		handler.ServeHTTP(r, httptest.NewRequest(http.MethodPost, "/api/intake/promote", bytes.NewReader(b)))
		return r
	}
	merged := post(map[string]any{"date": "2026-11-21", "photo_rule": "撮影可能"})
	var mergedResponse map[string]any
	if err := json.Unmarshal(merged.Body.Bytes(), &mergedResponse); err != nil {
		t.Fatal(err)
	}
	if merged.Code != http.StatusOK || mergedResponse["action"] != "merged" || mergedResponse["session_id"] != res["session_id"] {
		t.Fatalf("unexpected merge: %d %s", merged.Code, merged.Body.String())
	}
	second := post(map[string]any{"date": "2026-11-21", "title": "別公演", "venue": "渋谷RING", "live_start": "23:00"})
	if second.Code != http.StatusOK {
		t.Fatalf("second event: %d %s", second.Code, second.Body.String())
	}
	conflict := post(map[string]any{"date": "2026-11-21", "photo_rule": "撮影可能"})
	if conflict.Code != http.StatusConflict {
		t.Fatalf("expected 409: %d %s", conflict.Code, conflict.Body.String())
	}
	patchBody, _ := json.Marshal(map[string]any{"date": "2026-11-21", "photo_rule": "撮影可能"})
	patchResponse := httptest.NewRecorder()
	handler.ServeHTTP(patchResponse, httptest.NewRequest(http.MethodPost, "/api/intake/patch", bytes.NewReader(patchBody)))
	if patchResponse.Code != http.StatusConflict {
		t.Fatalf("patch should require a target: %d %s", patchResponse.Code, patchResponse.Body.String())
	}
	targeted := post(map[string]any{"date": "2026-11-21", "target_session_id": res["session_id"], "photo_rule": "撮影可能"})
	if targeted.Code != http.StatusOK {
		t.Fatalf("targeted merge: %d %s", targeted.Code, targeted.Body.String())
	}
}

func TestServer_Intake_Patch(t *testing.T) {
	collector := NewCollector("test-v1")
	handler := NewHandler(collector)

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
				"benefit": {"start_time": "21:10"},
				"ticket": {"platform": "", "prices": []}
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

type memoryObjectStore struct {
	mu      sync.Mutex
	objects map[string][]byte
}

func newMemoryObjectStore() *memoryObjectStore {
	return &memoryObjectStore{objects: make(map[string][]byte)}
}

func (m *memoryObjectStore) key(bucket, object string) string {
	return bucket + "/" + object
}

func (m *memoryObjectStore) GetObject(_ context.Context, bucket, object string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.objects[m.key(bucket, object)]
	if !ok {
		return nil, gcs.ErrObjectNotFound
	}
	cp := make([]byte, len(b))
	copy(cp, b)
	return cp, nil
}

func (m *memoryObjectStore) PutObject(_ context.Context, bucket, object, _, _ string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]byte, len(data))
	copy(cp, data)
	m.objects[m.key(bucket, object)] = cp
	return nil
}

func (m *memoryObjectStore) ListObjects(_ context.Context, bucket, prefix string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []string
	fullPrefix := bucket + "/" + prefix
	for k := range m.objects {
		if strings.HasPrefix(k, fullPrefix) {
			out = append(out, strings.TrimPrefix(k, bucket+"/"))
		}
	}
	return out, nil
}

func TestServer_GCS_PromotePatchAndSessions(t *testing.T) {
	store := newMemoryObjectStore()
	initial := `{
		"schema_version": 1,
		"generated_at": "2026-11-20T00:00:00Z",
		"timezone": "Asia/Tokyo",
		"target_group": {"name": "リボンクロック"},
		"sessions": []
	}`
	_ = store.PutObject(context.Background(), "taite-board-data", "data/groups/ribonclock/schedule.json", "", "", []byte(initial))

	collector := NewCollector("test-v1")
	collector.SetCloudStorage(store, "taite-board-data", "taite-board-state")
	handler := NewHandlerWithConfig(collector, &mockFetcher{}, &mockResolver{}, HandlerConfig{
		Store:       store,
		DataBucket:  "taite-board-data",
		StateBucket: "taite-board-state",
	})

	// 1. Promote new session -> writes to GCS primary + mirrors
	promoteBody, _ := json.Marshal(map[string]any{
		"group":         "ribonclock",
		"source_id":     "990011",
		"source_url":    "https://x.com/Ribbon_Clock/status/990011",
		"title":         "クラウドテスト公演",
		"date":          "2026-11-25",
		"venue":         "渋谷RING",
		"live_start":    "19:00",
		"live_end":      "19:20",
		"benefit_start": "19:30",
		"benefit_end":   "20:30",
		"booth":         "並行A",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/intake/promote", bytes.NewReader(promoteBody))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("promote failed (%d): %s", rec.Code, rec.Body.String())
	}

	var promoteRes map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &promoteRes)
	if promoteRes["storage"] != "gcs" {
		t.Errorf("expected storage=gcs, got %v", promoteRes["storage"])
	}
	sessionID, _ := promoteRes["session_id"].(string)
	if sessionID != "20261125-1" {
		t.Errorf("expected session_id=20261125-1, got %s", sessionID)
	}

	// Verify mirrors were updated in GCS
	for _, obj := range []string{"data/groups/ribonclock/schedule.json", "schedule.json", "mock_schedule.json"} {
		b, err := store.GetObject(context.Background(), "taite-board-data", obj)
		if err != nil {
			t.Fatalf("expected mirror %s in GCS: %v", obj, err)
		}
		if !strings.Contains(string(b), "クラウドテスト公演") {
			t.Errorf("mirror %s missing promoted session", obj)
		}
	}

	// Verify pipeline_state.json was persisted to state bucket
	stateBytes, err := store.GetObject(context.Background(), "taite-board-state", "pipeline_state.json")
	if err != nil {
		t.Fatalf("expected pipeline_state.json in state bucket: %v", err)
	}
	if !strings.Contains(string(stateBytes), `"promoted_sessions": 1`) {
		t.Errorf("expected promoted_sessions=1 in state bucket, got %s", string(stateBytes))
	}

	// 2. List sessions via /api/sessions
	req = httptest.NewRequest(http.MethodGet, "/api/sessions?group=ribonclock", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("sessions list failed (%d): %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "20261125-1") {
		t.Errorf("expected session 20261125-1 in /api/sessions output: %s", rec.Body.String())
	}

	// 3. Patch session in GCS
	patchBody, _ := json.Marshal(map[string]any{
		"group":               "ribonclock",
		"session_id":          "20261125-1",
		"admission_bonus":     "サイン入りチェキ券",
		"photo_song":          "75秒の沈黙",
		"source_url":          "https://x.com/Ribbon_Clock/status/990022",
		"update_official_url": true,
	})
	req = httptest.NewRequest(http.MethodPost, "/api/intake/patch", bytes.NewReader(patchBody))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch failed (%d): %s", rec.Code, rec.Body.String())
	}

	updatedBytes, _ := store.GetObject(context.Background(), "taite-board-data", "schedule.json")
	if !strings.Contains(string(updatedBytes), "サイン入りチェキ券") || !strings.Contains(string(updatedBytes), "75秒の沈黙") {
		t.Errorf("expected patched bonus and photo song in GCS schedule.json, got %s", string(updatedBytes))
	}

	// 4. Invalid schedule (end_time <= start_time) should return 422 and not overwrite GCS
	badPromoteBody, _ := json.Marshal(map[string]any{
		"group":      "ribonclock",
		"title":      "不正時刻テスト",
		"date":       "2026-11-26",
		"venue":      "渋谷RING",
		"live_start": "20:00",
		"live_end":   "19:30",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/intake/promote", bytes.NewReader(badPromoteBody))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for invalid live times, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestServer_UISplit_And_DiscordNotification(t *testing.T) {
	store := newMemoryObjectStore()
	initial := `{
		"schema_version": 1,
		"generated_at": "2026-11-20T00:00:00Z",
		"timezone": "Asia/Tokyo",
		"target_group": {"name": "リボンクロック"},
		"sessions": []
	}`
	_ = store.PutObject(context.Background(), "taite-board-data", "data/groups/ribonclock/schedule.json", "", "", []byte(initial))

	var mu sync.Mutex
	var webhookPayloads []string
	webhookSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		webhookPayloads = append(webhookPayloads, string(b))
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer webhookSrv.Close()

	collector := NewCollector("test-v1")
	collector.SetCloudStorage(store, "taite-board-data", "taite-board-state")
	handler := NewHandlerWithConfig(collector, &mockFetcher{}, &mockResolver{}, HandlerConfig{
		Store:            store,
		DataBucket:       "taite-board-data",
		StateBucket:      "taite-board-state",
		WebhookURL:       webhookSrv.URL,
		NotifyHTTPClient: webhookSrv.Client(),
		NotifySync:       true,
	})

	// 1. UI Split: / serves dashboard, /register and /admin serve admin registration UI
	dashRec := httptest.NewRecorder()
	handler.ServeHTTP(dashRec, httptest.NewRequest(http.MethodGet, "/", nil))
	if dashRec.Code != http.StatusOK || strings.Contains(dashRec.Body.String(), "batch-intake-form") || !strings.Contains(dashRec.Body.String(), "Ribbon Clock Access Monitor") {
		t.Errorf("expected clean observability dashboard on /, got code=%d", dashRec.Code)
	}

	for _, path := range []string{"/register", "/admin"} {
		regRec := httptest.NewRecorder()
		handler.ServeHTTP(regRec, httptest.NewRequest(http.MethodGet, path, nil))
		if regRec.Code != http.StatusOK || !strings.Contains(regRec.Body.String(), "batch-intake-form") || !strings.Contains(regRec.Body.String(), "Ribbon Clock Admin") {
			t.Errorf("expected registration UI on %s, got code=%d", path, regRec.Code)
		}
	}

	// 2. Promote with ticket_url triggers Discord webhook and records activity
	promoteBody, _ := json.Marshal(map[string]any{
		"group":         "ribonclock",
		"source_id":     "990088",
		"source_url":    "https://x.com/Ribbon_Clock/status/990088",
		"title":         "通知テスト公演",
		"date":          "2026-11-28",
		"venue":         "Spotify O-EAST",
		"live_start":    "18:00",
		"live_end":      "18:25",
		"benefit_start": "18:45",
		"benefit_end":   "19:45",
		"ticket_url":    "https://t.livepocket.jp/e/test990088",
	})
	promRec := httptest.NewRecorder()
	handler.ServeHTTP(promRec, httptest.NewRequest(http.MethodPost, "/api/intake/promote", bytes.NewReader(promoteBody)))
	if promRec.Code != http.StatusOK {
		t.Fatalf("promote failed (%d): %s", promRec.Code, promRec.Body.String())
	}

	schedBytes, _ := store.GetObject(context.Background(), "taite-board-data", "schedule.json")
	if !strings.Contains(string(schedBytes), "https://t.livepocket.jp/e/test990088") {
		t.Errorf("expected ticket_url persisted in schedule.json, got %s", string(schedBytes))
	}

	// 3. Patch triggers Discord webhook and records patch activity
	patchBody, _ := json.Marshal(map[string]any{
		"group":           "ribonclock",
		"session_id":      "20261128-1",
		"admission_bonus": "ランダムチェキ",
		"source_url":      "https://x.com/Ribbon_Clock/status/990089",
	})
	patchRec := httptest.NewRecorder()
	handler.ServeHTTP(patchRec, httptest.NewRequest(http.MethodPost, "/api/intake/patch", bytes.NewReader(patchBody)))
	if patchRec.Code != http.StatusOK {
		t.Fatalf("patch failed (%d): %s", patchRec.Code, patchRec.Body.String())
	}

	// 4. Stats snapshot returns both recent_activities and discord_notify_enabled=true
	statsRec := httptest.NewRecorder()
	handler.ServeHTTP(statsRec, httptest.NewRequest(http.MethodGet, "/api/stats", nil))
	if statsRec.Code != http.StatusOK {
		t.Fatalf("stats failed (%d)", statsRec.Code)
	}
	var snap StatsSnapshot
	if err := json.Unmarshal(statsRec.Body.Bytes(), &snap); err != nil {
		t.Fatalf("decode stats: %v", err)
	}
	if !snap.DiscordNotifyEnabled {
		t.Errorf("expected discord_notify_enabled=true")
	}
	if len(snap.RecentActivities) != 2 {
		t.Fatalf("expected 2 recent activities, got %d", len(snap.RecentActivities))
	}
	if snap.RecentActivities[0].Action != "patch" || snap.RecentActivities[0].NotificationStatus != "sent" {
		t.Errorf("expected latest activity=patch/sent, got %+v", snap.RecentActivities[0])
	}
	if snap.RecentActivities[1].Action != "promote" || snap.RecentActivities[1].NotificationStatus != "sent" {
		t.Errorf("expected older activity=promote/sent, got %+v", snap.RecentActivities[1])
	}

	// 5. Verify activities.json was persisted to state bucket
	actBytes, err := store.GetObject(context.Background(), "taite-board-state", "activities.json")
	if err != nil || !strings.Contains(string(actBytes), "通知テスト公演") {
		t.Errorf("expected activities.json in state bucket, err=%v", err)
	}

	// 6. Test notification endpoint (configured -> 200, unconfigured -> 400)
	testRec := httptest.NewRecorder()
	handler.ServeHTTP(testRec, httptest.NewRequest(http.MethodPost, "/api/notify/test", nil))
	if testRec.Code != http.StatusOK {
		t.Errorf("expected 200 on /api/notify/test, got %d: %s", testRec.Code, testRec.Body.String())
	}

	mu.Lock()
	payloadCount := len(webhookPayloads)
	var firstPayload, secondPayload string
	if payloadCount >= 2 {
		firstPayload = webhookPayloads[0]
		secondPayload = webhookPayloads[1]
	}
	mu.Unlock()
	if payloadCount != 3 {
		t.Fatalf("expected 3 webhook calls (promote, patch, test), got %d", payloadCount)
	}
	if !strings.Contains(firstPayload, "新規ライブ登録") || !strings.Contains(firstPayload, "通知テスト公演") {
		t.Errorf("unexpected promote webhook payload: %s", firstPayload)
	}
	if !strings.Contains(secondPayload, "ライブ情報更新") || !strings.Contains(secondPayload, "ランダムチェキ") {
		t.Errorf("unexpected patch webhook payload: %s", secondPayload)
	}

	unconfHandler := NewHandlerWithDependencies(NewCollector("test-v1"), &mockFetcher{}, &mockResolver{})
	unconfRec := httptest.NewRecorder()
	unconfHandler.ServeHTTP(unconfRec, httptest.NewRequest(http.MethodPost, "/api/notify/test", nil))
	if unconfRec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 when DISCORD_WEBHOOK_URL is unset, got %d", unconfRec.Code)
	}
}

func TestDomainRoutingFetcher_And_TimeTreePromote(t *testing.T) {
	router := &DomainRoutingFetcher{
		X: &mockFetcher{
			post: &raw.Post{Source: "x", SourceID: "111", URL: "https://x.com/Ribbon_Clock/status/111", Author: "X Author", Text: "11/28 渋谷RING LIVE 18:00-18:20"},
		},
		TimeTree: &mockFetcher{
			post: &raw.Post{Source: "timetree", SourceID: "3093691568935820727", URL: "https://timetreeapp.com/public_calendars/ribbon_clock/events/3093691568935820727", Author: "リボンクロック 公式LIVE予定", Text: "11/28(土) 会場: LUMINE 0 出演: 10:45〜11:00"},
		},
	}

	xPost, err := router.FetchPost(context.Background(), "https://x.com/Ribbon_Clock/status/111")
	if err != nil || xPost.Source != "x" {
		t.Fatalf("expected x source, got post=%+v err=%v", xPost, err)
	}

	ttPost, err := router.FetchPost(context.Background(), "https://timetreeapp.com/public_calendars/ribbon_clock/events/3093691568935820727")
	if err != nil || ttPost.Source != "timetree" {
		t.Fatalf("expected timetree source, got post=%+v err=%v", ttPost, err)
	}

	ttShortPost, err := router.FetchPost(context.Background(), "https://timetr.ee/p/ribbon_clock/3093691568935820727")
	if err != nil || ttShortPost.Source != "timetree" {
		t.Fatalf("expected timetree source for timetr.ee, got post=%+v err=%v", ttShortPost, err)
	}

	if _, err := router.FetchPost(context.Background(), "https://example.com/live/123"); err == nil {
		t.Errorf("expected unsupported domain error for example.com")
	}

	// Verify promoting a TimeTree event passes schedule schema validation (source_tweet_url stays empty, source_refs records timetree_event)
	store := newMemoryObjectStore()
	_ = store.PutObject(context.Background(), "taite-board-data", "data/groups/ribonclock/schedule.json", "", "", []byte(`{
		"schema_version": 1, "generated_at": "2026-11-20T00:00:00Z", "timezone": "Asia/Tokyo",
		"target_group": {"name": "リボンクロック"}, "sessions": []
	}`))

	handler := NewHandlerWithConfig(NewCollector("test-v1"), router, &mockResolver{}, HandlerConfig{
		Store:       store,
		DataBucket:  "taite-board-data",
		StateBucket: "taite-board-state",
	})

	promBody, _ := json.Marshal(map[string]any{
		"group":      "ribonclock",
		"source_id":  "3093691568935820727",
		"source_url": "https://timetreeapp.com/public_calendars/ribbon_clock/events/3093691568935820727",
		"title":      "アイコレ",
		"date":       "2026-11-28",
		"venue":      "LUMINE 0",
		"live_start": "10:45",
		"live_end":   "11:00",
	})
	promRec := httptest.NewRecorder()
	handler.ServeHTTP(promRec, httptest.NewRequest(http.MethodPost, "/api/intake/promote", bytes.NewReader(promBody)))
	if promRec.Code != http.StatusOK {
		t.Fatalf("expected 200 promoting timetree event, got %d: %s", promRec.Code, promRec.Body.String())
	}
	saved, _ := store.GetObject(context.Background(), "taite-board-data", "schedule.json")
	if !strings.Contains(string(saved), `"type": "timetree_event"`) || !strings.Contains(string(saved), `"source_tweet_url": ""`) {
		t.Errorf("expected timetree_event source_ref and empty source_tweet_url, got: %s", string(saved))
	}
}

func TestServer_FailureNotifications_IntakeAndPromote(t *testing.T) {
	store := newMemoryObjectStore()
	_ = store.PutObject(context.Background(), "taite-board-data", "data/groups/ribonclock/schedule.json", "", "", []byte(`{
		"schema_version": 1, "generated_at": "2026-11-20T00:00:00Z", "timezone": "Asia/Tokyo",
		"target_group": {"name": "リボンクロック"}, "sessions": []
	}`))

	var mu sync.Mutex
	var payloads []string
	webhookSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		payloads = append(payloads, string(b))
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer webhookSrv.Close()

	collector := NewCollector("test-v1")
	handler := NewHandlerWithConfig(collector, &mockFetcher{err: errors.New("fetch failed 404")}, &mockResolver{}, HandlerConfig{
		Store:            store,
		DataBucket:       "taite-board-data",
		StateBucket:      "taite-board-state",
		WebhookURL:       webhookSrv.URL,
		NotifyHTTPClient: webhookSrv.Client(),
		NotifySync:       true,
	})

	// 1. Intake fetch error -> 422 + Discord failure notification
	intakeReq, _ := json.Marshal(map[string]string{"url": "https://x.com/Ribbon_Clock/status/999"})
	intakeRec := httptest.NewRecorder()
	handler.ServeHTTP(intakeRec, httptest.NewRequest(http.MethodPost, "/api/intake", bytes.NewReader(intakeReq)))
	if intakeRec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 on intake failure, got %d", intakeRec.Code)
	}

	// 2. Promote validation error -> 422 + Discord failure notification
	badPromote, _ := json.Marshal(map[string]any{
		"group":      "ribonclock",
		"title":      "失敗テスト公演",
		"date":       "2026-11-28",
		"venue":      "渋谷RING",
		"live_start": "20:00",
		"live_end":   "19:00",
	})
	promRec := httptest.NewRecorder()
	handler.ServeHTTP(promRec, httptest.NewRequest(http.MethodPost, "/api/intake/promote", bytes.NewReader(badPromote)))
	if promRec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 on promote validation failure, got %d", promRec.Code)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(payloads) != 2 {
		t.Fatalf("expected 2 failure webhook notifications, got %d", len(payloads))
	}
	if !strings.Contains(payloads[0], "リンク取得・解析エラー") || !strings.Contains(payloads[0], "fetch failed 404") {
		t.Errorf("unexpected intake failure payload: %s", payloads[0])
	}
	if !strings.Contains(payloads[1], "新規ライブ登録エラー") || !strings.Contains(payloads[1], "失敗テスト公演") {
		t.Errorf("unexpected promote failure payload: %s", payloads[1])
	}

	acts := collector.RecentActivities()
	if len(acts) != 2 || acts[0].Action != "promote_error" || acts[1].Action != "intake_error" {
		t.Errorf("unexpected recorded failure activities: %+v", acts)
	}
}

func TestServer_NotifyStart(t *testing.T) {
	var mu sync.Mutex
	var payloads []string
	webhookSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		payloads = append(payloads, string(b))
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer webhookSrv.Close()

	collector := NewCollector("test-v1")
	handler := NewHandlerWithConfig(collector, &mockFetcher{}, &mockResolver{}, HandlerConfig{
		WebhookURL:       webhookSrv.URL,
		NotifyHTTPClient: webhookSrv.Client(),
		NotifySync:       true,
	})

	body, _ := json.Marshal(map[string]string{
		"action":     "intake_start",
		"group":      "ribonclock",
		"title":      "一括リンク取り込み",
		"summary":    "4件のURLを取得・解析開始",
		"source_url": "https://x.com/Ribbon_Clock/status/123",
	})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/notify/start", bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	mu.Lock()
	defer mu.Unlock()
	if len(payloads) != 1 || !strings.Contains(payloads[0], "リンク取得・解析開始") || !strings.Contains(payloads[0], "4件のURLを取得・解析開始") {
		t.Fatalf("unexpected start notification payload: %v", payloads)
	}
}

func TestServer_ReconcileEndpoint_CloudRunAndFallback(t *testing.T) {
	var mu sync.Mutex
	var discordPayloads []string
	webhookSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		discordPayloads = append(discordPayloads, string(b))
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer webhookSrv.Close()

	var triggeredGroup string
	var triggeredDryRun bool
	collector := NewCollector("test-v1")
	handler := NewHandlerWithConfig(collector, &mockFetcher{}, &mockResolver{}, HandlerConfig{
		WebhookURL:       webhookSrv.URL,
		NotifyHTTPClient: webhookSrv.Client(),
		NotifySync:       true,
		CloudRunJob:      "projects/taite-board/locations/asia-northeast1/jobs/taite-reconcile",
		TriggerReconcileJob: func(_ context.Context, group string, dryRun bool) (string, error) {
			triggeredGroup = group
			triggeredDryRun = dryRun
			return "projects/taite-board/locations/asia-northeast1/jobs/taite-reconcile/executions/exec-123", nil
		},
	})

	reqBody, _ := json.Marshal(map[string]any{"group": "all", "dry_run": true})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/reconcile", bytes.NewReader(reqBody)))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202 Accepted, got %d: %s", rec.Code, rec.Body.String())
	}
	if triggeredGroup != "all" || !triggeredDryRun {
		t.Errorf("unexpected trigger args: group=%s dryRun=%v", triggeredGroup, triggeredDryRun)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["mode"] != "cloud_run_job" || !strings.Contains(resp["execution"].(string), "exec-123") {
		t.Errorf("unexpected response payload: %+v", resp)
	}

	mu.Lock()
	if len(discordPayloads) != 2 {
		t.Errorf("expected 2 discord notifications (start + triggered), got %d", len(discordPayloads))
	}
	mu.Unlock()

	// Fallback inline mode when CloudRunJob is unset
	mem := newMemoryObjectStore()
	_ = mem.PutObject(context.Background(), "taite-board-data", "data/groups/ribonclock/schedule.json", "application/json", "", []byte(`{
  "schema_version": 1,
  "generated_at": "2026-11-23T00:00:00+09:00",
  "timezone": "Asia/Tokyo",
  "target_group": { "name": "リボンクロック", "canonical_id": "group:ribbon-clock" },
  "sessions": [
    {
      "id": "20261123-1",
      "title": "🎀ギガ盛りMAX🎀",
      "date": "2026-11-23",
      "day_of_week": "月祝",
      "venue": { "canonical_id": "", "name": "ZEAL THEATER TOKYO" },
      "live": { "start_time": "--:--", "end_time": "", "stage": null },
      "benefit": { "type": "並行", "start_time": "--:--", "end_time": "", "booth": "ロビ-B" },
      "ticket": { "platform": "", "url": "https://livepocket.jp/e/x8-ap", "prices": [] },
      "source_tweet_url": "https://x.com/Ribbon_Clock/status/111"
    }
  ]
}`))
	inlineHandler := NewHandlerWithConfig(NewCollector("test-v1"), &mockFetcher{}, &mockResolver{}, HandlerConfig{
		Store:      mem,
		DataBucket: "taite-board-data",
	})
	rec2 := httptest.NewRecorder()
	inlineHandler.ServeHTTP(rec2, httptest.NewRequest(http.MethodPost, "/api/reconcile", bytes.NewReader([]byte(`{"group":"ribonclock","dry_run":false}`))))
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 on inline reconcile, got %d: %s", rec2.Code, rec2.Body.String())
	}
	updatedBytes, err := mem.GetObject(context.Background(), "taite-board-data", "schedule.json")
	if err != nil {
		t.Fatalf("expected schedule.json mirrored in GCS store: %v", err)
	}
	updatedStr := string(updatedBytes)
	if !strings.Contains(updatedStr, "ZEAL THEATER 新橋") || !strings.Contains(updatedStr, "ロビーB") {
		t.Errorf("expected reconciled schedule mirrored to schedule.json, got %s", updatedStr)
	}
}
