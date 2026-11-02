package promote

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPromoteCandidate_Success(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "promote-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create initial schedule file
	initialSchedule := map[string]interface{}{
		"schema_version": 1,
		"generated_at":   "2026-11-01T00:00:00+09:00",
		"timezone":       "Asia/Tokyo",
		"target_group": map[string]interface{}{
			"canonical_id": "group:ribbon-clock",
			"name":         "リボンクロック",
		},
		"sessions": []map[string]interface{}{
			{
				"id":          "20261105-1",
				"date":        "2026-11-05",
				"day_of_week": "木",
				"title":       "既存定期公演",
				"venue": map[string]interface{}{
					"canonical_id": "venue:spotify-o-nest",
					"name":         "Spotify O-nest",
					"map_query":    "Spotify O-nest",
					"map_url":      "https://maps.google.com/?q=Spotify+O-nest",
				},
				"live": map[string]interface{}{
					"start_time": "19:00",
					"end_time":   "19:30",
					"stage":      nil,
				},
				"ticket": map[string]interface{}{
					"platform": nil,
					"url":      nil,
					"prices":   []interface{}{},
				},
				"source_tweet_url": "https://x.com/Ribbon_Clock/status/1111111111",
				"source_refs": []map[string]interface{}{
					{
						"type":      "x_post",
						"source_id": "1111111111",
						"url":       "https://x.com/Ribbon_Clock/status/1111111111",
					},
				},
			},
		},
	}
	schedData, _ := json.Marshal(initialSchedule)
	schedPath := filepath.Join(tempDir, "schedule.json")
	if err := os.WriteFile(schedPath, schedData, 0o644); err != nil {
		t.Fatalf("failed to write initial schedule: %v", err)
	}

	// Create candidate intake contract file
	candidateDoc := map[string]interface{}{
		"contract":   "taite-board.candidate-intake/v1",
		"run_id":     "20261102T120000+0900",
		"created_at": "2026-11-02T12:00:00Z",
		"status":     "ready",
		"sessions": []map[string]interface{}{
			{
				"id":    "candidate-20261110-1",
				"title": "MARQUEE祭 mini Vol.360",
				"date":  "2026-11-10",
				"venue": map[string]interface{}{
					"name": "Spotify O-nest",
				},
				"live": map[string]interface{}{
					"start_time": "12:00",
					"end_time":   "12:25",
				},
				"source_refs": []map[string]interface{}{
					{
						"type":      "x_post",
						"source_id": "2222222222",
						"url":       "https://x.com/Ribbon_Clock/status/2222222222",
					},
				},
			},
		},
	}
	candData, _ := json.Marshal(candidateDoc)
	candPath := filepath.Join(tempDir, "candidate.json")
	if err := os.WriteFile(candPath, candData, 0o644); err != nil {
		t.Fatalf("failed to write candidate file: %v", err)
	}

	opts := PromoteOptions{
		CandidateFile:      candPath,
		TargetScheduleFile: schedPath,
		NoSync:             true,
		DryRun:             false,
	}

	res, err := PromoteCandidate(opts)
	if err != nil {
		t.Fatalf("PromoteCandidate failed: %v", err)
	}

	if res.TotalAdded != 1 {
		t.Errorf("expected 1 session added, got %d", res.TotalAdded)
	}
	if len(res.Sessions) != 1 {
		t.Fatalf("expected 1 session summary, got %d", len(res.Sessions))
	}
	if res.Sessions[0].ID != "20261110-1" {
		t.Errorf("expected auto-derived ID 20261110-1, got %s", res.Sessions[0].ID)
	}
	if res.Sessions[0].Action != "added" {
		t.Errorf("expected action 'added', got %s", res.Sessions[0].Action)
	}

	// Verify written file
	savedBytes, _ := os.ReadFile(schedPath)
	var savedDoc struct {
		Sessions []struct {
			ID        string `json:"id"`
			DayOfWeek string `json:"day_of_week"`
			Title     string `json:"title"`
			Venue     struct {
				CanonicalID string `json:"canonical_id"`
				Name        string `json:"name"`
			} `json:"venue"`
		} `json:"sessions"`
	}
	if err := json.Unmarshal(savedBytes, &savedDoc); err != nil {
		t.Fatalf("failed to unmarshal saved schedule: %v", err)
	}

	if len(savedDoc.Sessions) != 2 {
		t.Fatalf("expected 2 sessions in schedule, got %d", len(savedDoc.Sessions))
	}
	// Session 2 should have been sorted after 2026-11-05
	s2 := savedDoc.Sessions[1]
	if s2.ID != "20261110-1" {
		t.Errorf("expected second session ID to be 20261110-1, got %s", s2.ID)
	}
	if s2.DayOfWeek != "火" {
		t.Errorf("expected DayOfWeek '火', got %s", s2.DayOfWeek)
	}
	if s2.Venue.CanonicalID != "venue:spotify-o-nest" {
		t.Errorf("expected canonical venue ID 'venue:spotify-o-nest', got %s", s2.Venue.CanonicalID)
	}
}

func TestPromoteCandidate_UpdateExisting(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "promote-update-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	initialSchedule := map[string]interface{}{
		"schema_version": 1,
		"generated_at":   "2026-11-01T00:00:00+09:00",
		"timezone":       "Asia/Tokyo",
		"target_group": map[string]interface{}{
			"canonical_id": "group:ribbon-clock",
			"name":         "リボンクロック",
		},
		"sessions": []map[string]interface{}{
			{
				"id":          "20261112-1",
				"date":        "2026-11-12",
				"day_of_week": "木",
				"title":       "単独無銭ライブ",
				"venue": map[string]interface{}{
					"canonical_id": "venue:club-asia",
					"name":         "clubasia",
					"map_query":    "clubasia",
					"map_url":      "https://maps.google.com/?q=clubasia",
				},
				"live": map[string]interface{}{
					"start_time": "--:--",
					"stage":      nil,
				},
				"ticket": map[string]interface{}{
					"platform": nil,
					"url":      nil,
					"prices":   []interface{}{},
				},
				"source_tweet_url": "https://x.com/Ribbon_Clock/status/3333333333",
				"source_refs": []map[string]interface{}{
					{
						"type":      "x_post",
						"source_id": "3333333333",
						"url":       "https://x.com/Ribbon_Clock/status/3333333333",
					},
				},
			},
		},
	}
	schedData, _ := json.Marshal(initialSchedule)
	schedPath := filepath.Join(tempDir, "schedule.json")
	if err := os.WriteFile(schedPath, schedData, 0o644); err != nil {
		t.Fatalf("failed to write initial schedule: %v", err)
	}

	// Updated candidate for the same event with confirmed live time
	candData := []byte(`{
		"contract": "taite-board.candidate-intake/v1",
		"sessions": [
			{
				"id": "20261112-1",
				"date": "2026-11-12",
				"title": "単独無銭ライブ",
				"venue": { "name": "clubasia" },
				"live": { "start_time": "19:15", "end_time": "20:00" },
				"source_tweet_url": "https://x.com/Ribbon_Clock/status/3333333333"
			}
		]
	}`)
	candPath := filepath.Join(tempDir, "cand_update.json")
	if err := os.WriteFile(candPath, candData, 0o644); err != nil {
		t.Fatalf("failed to write candidate file: %v", err)
	}

	res, err := PromoteCandidate(PromoteOptions{
		CandidateFile:      candPath,
		TargetScheduleFile: schedPath,
		NoSync:             true,
	})
	if err != nil {
		t.Fatalf("PromoteCandidate failed: %v", err)
	}

	if res.TotalUpdated != 1 {
		t.Errorf("expected 1 session updated, got %d", res.TotalUpdated)
	}
	if res.TotalAdded != 0 {
		t.Errorf("expected 0 sessions added, got %d", res.TotalAdded)
	}

	// Verify updated field
	savedBytes, _ := os.ReadFile(schedPath)
	var savedDoc struct {
		Sessions []struct {
			ID   string `json:"id"`
			Live struct {
				StartTime string `json:"start_time"`
				EndTime   string `json:"end_time"`
			} `json:"live"`
		} `json:"sessions"`
	}
	_ = json.Unmarshal(savedBytes, &savedDoc)
	if savedDoc.Sessions[0].Live.StartTime != "19:15" {
		t.Errorf("expected live start_time 19:15, got %s", savedDoc.Sessions[0].Live.StartTime)
	}
}

func TestPromoteCandidate_DryRun(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "promote-dryrun-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	initialSchedule := map[string]interface{}{
		"schema_version": 1,
		"generated_at":   "2026-11-01T00:00:00+09:00",
		"timezone":       "Asia/Tokyo",
		"target_group": map[string]interface{}{
			"canonical_id": "group:ribbon-clock",
			"name":         "リボンクロック",
		},
		"sessions": []map[string]interface{}{},
	}
	schedData, _ := json.Marshal(initialSchedule)
	schedPath := filepath.Join(tempDir, "schedule.json")
	_ = os.WriteFile(schedPath, schedData, 0o644)

	candData := []byte(`{
		"contract": "taite-board.candidate-intake/v1",
		"sessions": [
			{
				"date": "2026-11-20",
				"title": "新フェス出演",
				"venue": { "name": "豊洲PIT" },
				"source_tweet_url": "https://x.com/Ribbon_Clock/status/4444444444"
			}
		]
	}`)
	candPath := filepath.Join(tempDir, "cand_dry.json")
	_ = os.WriteFile(candPath, candData, 0o644)

	res, err := PromoteCandidate(PromoteOptions{
		CandidateFile:      candPath,
		TargetScheduleFile: schedPath,
		DryRun:             true,
	})
	if err != nil {
		t.Fatalf("DryRun failed: %v", err)
	}

	if res.TotalAdded != 1 {
		t.Errorf("expected 1 session reported added in DryRun, got %d", res.TotalAdded)
	}

	// Ensure file on disk was NOT modified
	savedBytes, _ := os.ReadFile(schedPath)
	var savedDoc struct {
		Sessions []interface{} `json:"sessions"`
	}
	_ = json.Unmarshal(savedBytes, &savedDoc)
	if len(savedDoc.Sessions) != 0 {
		t.Errorf("schedule file must remain unchanged in DryRun, got %d sessions", len(savedDoc.Sessions))
	}
}
