package schedule

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSortSchedule(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "schedule.json")
	syncCopy := filepath.Join(tmpDir, "copy.json")

	initial := rootDoc{
		SchemaVersion: 1,
		GeneratedAt:   "2026-10-01T00:00:00Z",
		Timezone:      "Asia/Tokyo",
		Sessions: []json.RawMessage{
			json.RawMessage(`{"id":"20261102-1","date":"2026-11-02","live":{"start_time":"12:00"}}`),
			json.RawMessage(`{"id":"20261101-2","date":"2026-11-01","live":{"start_time":"18:00"}}`),
			json.RawMessage(`{"id":"20261101-1","date":"2026-11-01","live":{"start_time":"12:00"}}`),
		},
	}
	bytes, _ := json.Marshal(initial)
	_ = os.WriteFile(filePath, bytes, 0644)

	count, err := SortSchedule(filePath, syncCopy)
	if err != nil {
		t.Fatalf("SortSchedule failed: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 sessions, got %d", count)
	}

	raw, _ := os.ReadFile(filePath)
	var doc rootDoc
	_ = json.Unmarshal(raw, &doc)

	var s0, s1, s2 struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(doc.Sessions[0], &s0)
	_ = json.Unmarshal(doc.Sessions[1], &s1)
	_ = json.Unmarshal(doc.Sessions[2], &s2)

	if s0.ID != "20261101-1" || s1.ID != "20261101-2" || s2.ID != "20261102-1" {
		t.Errorf("incorrect sort order: [%s, %s, %s]", s0.ID, s1.ID, s2.ID)
	}
}

func TestAddSession(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "schedule.json")

	initial := rootDoc{
		SchemaVersion: 1,
		GeneratedAt:   "2026-10-01T00:00:00Z",
		Timezone:      "Asia/Tokyo",
		Sessions: []json.RawMessage{
			json.RawMessage(`{"id":"20261101-1","date":"2026-11-01","live":{"start_time":"12:00"}}`),
		},
	}
	bytes, _ := json.Marshal(initial)
	_ = os.WriteFile(filePath, bytes, 0644)

	id, err := AddSession(AddSessionOptions{
		Date:          "2026-11-01",
		Title:         "Live Part 2",
		VenueName:     "Test Venue",
		LiveStartTime: "18:00",
		FilePath:      filePath,
	})
	if err != nil {
		t.Fatalf("AddSession failed: %v", err)
	}
	if id != "20261101-2" {
		t.Errorf("expected auto ID 20261101-2, got %s", id)
	}

	raw, _ := os.ReadFile(filePath)
	var doc rootDoc
	_ = json.Unmarshal(raw, &doc)
	if len(doc.Sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(doc.Sessions))
	}
}
