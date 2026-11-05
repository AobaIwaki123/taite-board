package monitor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	githubapi "github.com/AobaIwaki123/taite-board/internal/github"
)

type recordingWriteback struct {
	files   map[string][]byte
	calls   []string
	commits int
}

func newRecordingWriteback(initial []byte) *recordingWriteback {
	files := make(map[string][]byte)
	for _, path := range scheduleWritebackPaths("ribonclock") {
		files[path] = append([]byte(nil), initial...)
	}
	return &recordingWriteback{files: files}
}

func (w *recordingWriteback) UpdateFile(_ context.Context, _ string, path string, _ string, mutate githubapi.FileMutation) (githubapi.CommitResult, error) {
	current, ok := w.files[path]
	if !ok {
		return githubapi.CommitResult{}, fmt.Errorf("missing test file %s", path)
	}
	updated, err := mutate(current)
	if err != nil {
		return githubapi.CommitResult{}, err
	}
	w.files[path] = updated
	w.calls = append(w.calls, path)
	w.commits++
	return githubapi.CommitResult{
		Path: path,
		SHA:  "commit-sha",
		URL:  "https://github.com/AobaIwaki123/taite-board/commit/commit-sha",
	}, nil
}

func TestServer_Intake_CommitPromoteUsesGitHubWriteback(t *testing.T) {
	initial := []byte(`{
  "schema_version": 1,
  "generated_at": "2026-11-05T00:00:00Z",
  "timezone": "Asia/Tokyo",
  "target_group": {"name": "Test"},
  "sessions": []
}`)
	writeback := newRecordingWriteback(initial)
	collector := NewCollector("test-v1")
	handler := NewHandlerWithDependenciesAndWriteback(collector, nil, nil, writeback)

	body, _ := json.Marshal(map[string]any{
		"action":        "promote",
		"group":         "ribonclock",
		"source_url":    "https://x.com/Ribbon_Clock/status/1234567890",
		"title":         "Test Live",
		"date":          "2026-11-21",
		"venue":         "渋谷RING",
		"live_start":    "20:20",
		"live_end":      "20:40",
		"benefit_start": "21:10",
		"benefit_end":   "22:10",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/intake/commit", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var response commitResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if response.SessionID != "20261121-1" {
		t.Errorf("expected session ID 20261121-1, got %s", response.SessionID)
	}
	if response.CommitURL == "" || len(response.CommitPaths) != 5 {
		t.Errorf("expected commit metadata for five paths, got %+v", response)
	}
	if writeback.commits != 5 {
		t.Fatalf("expected five GitHub file commits, got %d", writeback.commits)
	}
	if got := collector.Snapshot().Pipeline.PromotedSessions; got != 1 {
		t.Fatalf("expected one promoted session in collector, got %d", got)
	}
	for path, content := range writeback.files {
		var schedule struct {
			Sessions []struct {
				ID string `json:"id"`
			} `json:"sessions"`
		}
		if err := json.Unmarshal(content, &schedule); err != nil {
			t.Fatalf("parsing committed %s: %v", path, err)
		}
		if len(schedule.Sessions) != 1 || schedule.Sessions[0].ID != "20261121-1" {
			t.Errorf("unexpected committed schedule at %s: %+v", path, schedule.Sessions)
		}
	}
}

func TestServer_Intake_CommitPatchUsesGitHubWriteback(t *testing.T) {
	initial := []byte(`{
  "schema_version": 1,
  "generated_at": "2026-11-05T00:00:00Z",
  "timezone": "Asia/Tokyo",
  "target_group": {"name": "Test"},
  "sessions": [{
    "id": "20261121-1",
    "title": "Test Live",
    "date": "2026-11-21",
    "venue": {"name": "渋谷RING"},
    "live": {"start_time": "20:20", "end_time": "20:40", "stage": null},
    "benefit": {"type": "並行", "start_time": "21:10", "end_time": "22:10", "booth": "A"},
    "photo": null,
    "ticket": {"platform": "公式アナウンス待ち", "url": null, "prices": []}
  }]
}`)
	writeback := newRecordingWriteback(initial)
	collector := NewCollector("test-v1")
	handler := NewHandlerWithDependenciesAndWriteback(collector, nil, nil, writeback)

	body, _ := json.Marshal(map[string]any{
		"action":          "patch",
		"group":           "ribonclock",
		"session_id":      "20261121-1",
		"photo_song":      "未来へのファンファーレ",
		"photo_device":    "video_ok",
		"admission_bonus": "指名入場特典：宿題チェキ券",
		"source_url":      "https://x.com/Ribbon_Clock/status/1234567890",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/intake/commit", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	for path, content := range writeback.files {
		var schedule struct {
			Sessions []struct {
				AdmissionBonus string `json:"admission_bonus"`
				Photo          struct {
					AllowedSongs []string `json:"allowed_songs"`
					Device       string   `json:"device"`
				} `json:"photo"`
			} `json:"sessions"`
		}
		if err := json.Unmarshal(content, &schedule); err != nil {
			t.Fatalf("parsing committed %s: %v", path, err)
		}
		if schedule.Sessions[0].AdmissionBonus != "指名入場特典：宿題チェキ券" {
			t.Errorf("admission bonus not updated in %s", path)
		}
		if len(schedule.Sessions[0].Photo.AllowedSongs) != 1 || schedule.Sessions[0].Photo.Device != "video_ok" {
			t.Errorf("photo patch not updated in %s: %+v", path, schedule.Sessions[0].Photo)
		}
	}
	if got := collector.Snapshot().Pipeline.PromotedSessions; got != 0 {
		t.Fatalf("expected patch to leave promoted-session count unchanged, got %d", got)
	}
}

func TestServer_Intake_CommitRequiresWritebackConfiguration(t *testing.T) {
	handler := NewHandlerWithDependenciesAndWriteback(NewCollector("test-v1"), nil, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/intake/commit", bytes.NewReader([]byte(`{"action":"promote"}`)))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d: %s", rec.Code, rec.Body.String())
	}
}
