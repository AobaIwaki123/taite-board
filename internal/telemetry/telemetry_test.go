package telemetry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AobaIwaki123/taite-board/internal/candidate"
	"github.com/AobaIwaki123/taite-board/internal/raw"
)

func TestCollectPipelineStats(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Create a crawl run with 2 posts:
	//    - Post 1: Valid timetable with image -> should PASS
	//    - Post 2: Chatter without image -> should be FILTERED (media)
	run1Dir := filepath.Join(tempDir, "raw", "runs", "20261030T100000+0900")
	posts1Dir := filepath.Join(run1Dir, "posts")
	if err := os.MkdirAll(posts1Dir, 0755); err != nil {
		t.Fatalf("failed to create dirs: %v", err)
	}

	manifest1 := raw.RunManifest{
		SchemaVersion: 1,
		RunID:         "20261030T100000+0900",
		Source:        "x",
		FetchedAt:     time.Now().Format(time.RFC3339),
		InputCount:    2,
		SuccessCount:  2,
	}
	mBytes, _ := json.Marshal(manifest1)
	_ = os.WriteFile(filepath.Join(run1Dir, "manifest.json"), mBytes, 0644)

	post1 := raw.Post{
		Source:    "x",
		SourceID:  "1001",
		Text:      "🎀 リボンクロック 次回ライブ情報！\n2026年11月10日 SHIBUYA DIVE\nタイテ公開です！",
		Images:    []string{"https://example.com/timetable.jpg"},
		FetchedAt: time.Now().Format(time.RFC3339),
	}
	p1Bytes, _ := json.Marshal(post1)
	_ = os.WriteFile(filepath.Join(posts1Dir, "1001.json"), p1Bytes, 0644)

	post2 := raw.Post{
		Source:    "x",
		SourceID:  "1002",
		Text:      "おはようございます！今日も一日がんばりましょう！",
		Images:    nil, // No images -> should filter by media
		FetchedAt: time.Now().Format(time.RFC3339),
	}
	p2Bytes, _ := json.Marshal(post2)
	_ = os.WriteFile(filepath.Join(posts1Dir, "1002.json"), p2Bytes, 0644)

	// 2. Create Candidate with cache hit
	candDir := filepath.Join(tempDir, "candidate")
	if err := os.MkdirAll(candDir, 0755); err != nil {
		t.Fatalf("failed to create candidate dir: %v", err)
	}
	cand1 := candidate.IntakeResult{
		SourceID: "1003",
		Status:   candidate.StatusSkippedCache,
	}
	cBytes, _ := json.Marshal(cand1)
	_ = os.WriteFile(filepath.Join(candDir, "cand1003.json"), cBytes, 0644)

	// 3. Create schedule.json with 2 sessions
	genDir := filepath.Join(tempDir, "generated")
	if err := os.MkdirAll(genDir, 0755); err != nil {
		t.Fatalf("failed to create generated dir: %v", err)
	}
	schedJSON := `{"sessions":[{"id":"sess-1","date":"2026-11-10"},{"id":"sess-2","date":"2026-11-11"}]}`
	_ = os.WriteFile(filepath.Join(genDir, "schedule.json"), []byte(schedJSON), 0644)

	// Execute CollectPipelineStats
	stats, err := CollectPipelineStats(tempDir)
	if err != nil {
		t.Fatalf("CollectPipelineStats failed: %v", err)
	}

	if stats.TotalRuns != 1 {
		t.Errorf("expected 1 total run, got %d", stats.TotalRuns)
	}
	if stats.DetectedPosts != 2 {
		t.Errorf("expected 2 detected posts, got %d", stats.DetectedPosts)
	}
	if stats.FilterPassed != 1 {
		t.Errorf("expected 1 filter passed, got %d", stats.FilterPassed)
	}
	if stats.FilteredPosts != 1 {
		t.Errorf("expected 1 filtered post, got %d", stats.FilteredPosts)
	}
	if stats.FilterBreakdown.Media != 1 {
		t.Errorf("expected 1 media filtered, got %d", stats.FilterBreakdown.Media)
	}
	if stats.AICacheHits != 1 {
		t.Errorf("expected 1 AI cache hit, got %d", stats.AICacheHits)
	}
	if stats.PromotedSessions != 2 {
		t.Errorf("expected 2 promoted sessions, got %d", stats.PromotedSessions)
	}
	if stats.SavedAPICalls != 2 { // 1 filtered + 1 cache hit
		t.Errorf("expected 2 saved API calls, got %d", stats.SavedAPICalls)
	}
	expectedCost := 2 * UnitCostUSDPerCall
	if stats.EstimatedCostSavedUSD != expectedCost {
		t.Errorf("expected cost saved %f, got %f", expectedCost, stats.EstimatedCostSavedUSD)
	}

	// Verify ASCII formatting
	ascii := FormatASCIIFunnel(stats)
	if !strings.Contains(ascii, "Detected Posts:          2") {
		t.Errorf("ASCII funnel missing detected posts count: %s", ascii)
	}
	if !strings.Contains(ascii, "Media Check:       1") {
		t.Errorf("ASCII funnel missing media filter breakdown: %s", ascii)
	}
	if !strings.Contains(ascii, "Cloud Run Dispatched") {
		t.Errorf("ASCII funnel missing cloud run dispatched: %s", ascii)
	}
}

func TestCollectAndPersist(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "state", "pipeline_state.json")

	// Initial empty scan should write empty or return empty
	stats, err := CollectAndPersist(tempDir, stateFile)
	if err != nil {
		t.Fatalf("unexpected error on empty collect: %v", err)
	}
	if stats.TotalRuns != 0 {
		t.Errorf("expected 0 runs on empty dir, got %d", stats.TotalRuns)
	}

	// Persist mock stats directly to state file
	mockStats := PipelineStats{
		TotalRuns:     5,
		DetectedPosts: 15,
		FilteredPosts: 12,
		FilterPassed:  3,
		LastRun: &CrawlRunRecord{
			RunID:      "20261102T120000+0900",
			InputCount: 4,
			Filtered:   3,
			Passed:     1,
			Status:     "Candidates Extracted",
		},
	}
	if err := SavePipelineState(stateFile, mockStats); err != nil {
		t.Fatalf("failed to save pipeline state: %v", err)
	}

	// Now CollectAndPersist on empty dataDir should fall back to stateFile
	recovered, err := CollectAndPersist(tempDir, stateFile)
	if err != nil {
		t.Fatalf("failed to recover from stateFile: %v", err)
	}
	if recovered.TotalRuns != 5 {
		t.Errorf("expected 5 recovered runs, got %d", recovered.TotalRuns)
	}
	if recovered.LastRun == nil || recovered.LastRun.RunID != "20261102T120000+0900" {
		t.Errorf("failed to recover LastRun record")
	}
}
