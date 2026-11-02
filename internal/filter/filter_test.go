package filter

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/AobaIwaki123/taite-board/internal/candidate"
	"github.com/AobaIwaki123/taite-board/internal/raw"
)

func TestEvaluate_NilPost(t *testing.T) {
	res := Evaluate(nil, DefaultRules, nil, time.Now())
	if res.Passed {
		t.Fatalf("expected nil post to fail filter, got passed")
	}
	if res.Stage != StageMedia {
		t.Errorf("expected StageMedia, got %s", res.Stage)
	}
}

func TestEvaluate_MediaCheck(t *testing.T) {
	rules := DefaultRules
	rules.RequireMedia = true

	now := time.Now()

	// Post without images
	postNoImages := &raw.Post{
		Source:    "x",
		SourceID:  "1001",
		Text:      "リボンクロック ライブ出演情報！タイムテーブル公開",
		Images:    nil,
		FetchedAt: now.Format(time.RFC3339),
	}
	res := Evaluate(postNoImages, rules, nil, now)
	if res.Passed {
		t.Errorf("expected post without images to be rejected")
	}
	if res.Stage != StageMedia {
		t.Errorf("expected StageMedia, got %s", res.Stage)
	}

	// Post with images
	postWithImages := &raw.Post{
		Source:    "x",
		SourceID:  "1001",
		Text:      "リボンクロック ライブ出演情報！タイムテーブル公開",
		Images:    []string{"https://pbs.twimg.com/media/sample.jpg"},
		FetchedAt: now.Format(time.RFC3339),
	}
	res = Evaluate(postWithImages, rules, nil, now)
	if !res.Passed {
		t.Errorf("expected post with images to pass, got rejected with reason: %s", res.Reason)
	}

	// Media not required
	rulesNoMedia := rules
	rulesNoMedia.RequireMedia = false
	res = Evaluate(postNoImages, rulesNoMedia, nil, now)
	if !res.Passed {
		t.Errorf("expected post without images to pass when RequireMedia=false, got %s", res.Reason)
	}
}

func TestEvaluate_AgeCheck(t *testing.T) {
	rules := DefaultRules
	rules.MaxAgeHours = 24

	now := time.Now()

	// Fresh post (2 hours ago)
	freshPost := &raw.Post{
		Source:    "x",
		SourceID:  "1002",
		Text:      "リボンクロック タイムテーブル出演情報",
		Images:    []string{"https://pbs.twimg.com/media/sample.jpg"},
		FetchedAt: now.Add(-2 * time.Hour).Format(time.RFC3339),
	}
	res := Evaluate(freshPost, rules, nil, now)
	if !res.Passed {
		t.Errorf("expected fresh post to pass, got rejected: %s", res.Reason)
	}

	// Expired post (48 hours ago)
	oldPost := &raw.Post{
		Source:    "x",
		SourceID:  "1003",
		Text:      "リボンクロック タイムテーブル出演情報",
		Images:    []string{"https://pbs.twimg.com/media/sample.jpg"},
		FetchedAt: now.Add(-48 * time.Hour).Format(time.RFC3339),
	}
	res = Evaluate(oldPost, rules, nil, now)
	if res.Passed {
		t.Errorf("expected old post to be rejected")
	}
	if res.Stage != StageAge {
		t.Errorf("expected StageAge, got %s", res.Stage)
	}
}

func TestEvaluate_ExcludeKeywords(t *testing.T) {
	rules := DefaultRules
	now := time.Now()

	// Post containing excluded keyword "配信"
	postSpam := &raw.Post{
		Source:    "x",
		SourceID:  "1004",
		Text:      "本日21:00〜 SHOWROOM配信します！遊びにきてね",
		Images:    []string{"https://pbs.twimg.com/media/sample.jpg"},
		FetchedAt: now.Format(time.RFC3339),
	}
	res := Evaluate(postSpam, rules, nil, now)
	if res.Passed {
		t.Errorf("expected post with 'SHOWROOM配信' to be rejected")
	}
	if res.Stage != StageExclude {
		t.Errorf("expected StageExclude, got %s", res.Stage)
	}

	// Post containing everyday greeting
	postGreeting := &raw.Post{
		Source:    "x",
		SourceID:  "1005",
		Text:      "おはようございます！今日も1日がんばろうね〜",
		Images:    []string{"https://pbs.twimg.com/media/sample.jpg"},
		FetchedAt: now.Format(time.RFC3339),
	}
	res = Evaluate(postGreeting, rules, nil, now)
	if res.Passed {
		t.Errorf("expected greeting post to be rejected")
	}
	if res.Stage != StageExclude {
		t.Errorf("expected StageExclude, got %s", res.Stage)
	}
}

func TestEvaluate_TargetKeywords(t *testing.T) {
	rules := DefaultRules
	now := time.Now()

	// Irrelevant post without target keywords
	postIrrelevant := &raw.Post{
		Source:    "x",
		SourceID:  "1006",
		Text:      "美味しいタピオカミルクティーを飲んだよ！",
		Images:    []string{"https://pbs.twimg.com/media/sample.jpg"},
		FetchedAt: now.Format(time.RFC3339),
	}
	res := Evaluate(postIrrelevant, rules, nil, now)
	if res.Passed {
		t.Errorf("expected irrelevant post to be rejected")
	}
	if res.Stage != StageTarget {
		t.Errorf("expected StageTarget, got %s", res.Stage)
	}

	// Relevant timetable post
	postRelevant := &raw.Post{
		Source:    "x",
		SourceID:  "1007",
		Text:      "【ライブ告知】11/15(日) Spotify O-nest リボンクロック 出演！タイテ公開",
		Images:    []string{"https://pbs.twimg.com/media/sample.jpg"},
		FetchedAt: now.Format(time.RFC3339),
	}
	res = Evaluate(postRelevant, rules, nil, now)
	if !res.Passed {
		t.Errorf("expected relevant timetable post to pass, got rejected: %s", res.Reason)
	}
}

func TestEvaluate_Deduplication(t *testing.T) {
	rules := DefaultRules
	now := time.Now()

	post := &raw.Post{
		Source:    "x",
		SourceID:  "2001",
		Text:      "リボンクロック ライブタイテ公開！",
		Images:    []string{"https://pbs.twimg.com/media/sample.jpg"},
		FetchedAt: now.Format(time.RFC3339),
	}

	fp := CalculateRawFingerprint(post)
	if fp == "" {
		t.Fatalf("expected non-empty fingerprint")
	}

	existingMap := map[string]bool{
		fp: true,
	}

	// Duplicate post evaluation
	res := Evaluate(post, rules, existingMap, now)
	if res.Passed {
		t.Errorf("expected duplicate post to be rejected")
	}
	if res.Stage != StageDuplicate {
		t.Errorf("expected StageDuplicate, got %s", res.Stage)
	}

	// Distinct post with different SourceID
	postOther := &raw.Post{
		Source:    "x",
		SourceID:  "2002",
		Text:      "リボンクロック ライブタイテ公開！",
		Images:    []string{"https://pbs.twimg.com/media/sample.jpg"},
		FetchedAt: now.Format(time.RFC3339),
	}
	resOther := Evaluate(postOther, rules, existingMap, now)
	if !resOther.Passed {
		t.Errorf("expected post with different ID to pass, got %s", resOther.Reason)
	}
}

func TestCalculateParseCacheKey(t *testing.T) {
	post := &raw.Post{
		Source:   "x",
		SourceID: "3001",
		Text:     "リボクロ タイテ",
		Images:   []string{"https://pbs.twimg.com/media/sample.jpg"},
	}

	key1 := CalculateParseCacheKey(post, 1, "v1.0.0", "gemini-2.5-flash", "prompt-v1")
	key2 := CalculateParseCacheKey(post, 1, "v1.0.0", "gemini-2.5-flash", "prompt-v1")
	key3 := CalculateParseCacheKey(post, 1, "v1.1.0", "gemini-2.5-flash", "prompt-v1")

	if key1 == "" || key2 == "" || key3 == "" {
		t.Fatalf("cache keys must not be empty")
	}
	if key1 != key2 {
		t.Errorf("cache keys with identical inputs must match: %s vs %s", key1, key2)
	}
	if key1 == key3 {
		t.Errorf("cache keys with different parser versions must differ: %s vs %s", key1, key3)
	}
}

func TestLoadExistingFingerprints(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "filter-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Write candidate 1
	c1 := candidate.IntakeResult{
		RawFingerprint: "fingerprint_abc123",
		SourceID:       "111",
	}
	data1, _ := json.Marshal(c1)
	if err := os.WriteFile(filepath.Join(tempDir, "cand1.json"), data1, 0o644); err != nil {
		t.Fatalf("failed to write cand1: %v", err)
	}

	// Write candidate 2
	c2 := map[string]interface{}{
		"raw_fingerprint": "fingerprint_xyz789",
		"title":           "Test Title",
	}
	data2, _ := json.Marshal(c2)
	if err := os.WriteFile(filepath.Join(tempDir, "cand2.json"), data2, 0o644); err != nil {
		t.Fatalf("failed to write cand2: %v", err)
	}

	fps, err := LoadExistingFingerprints(tempDir)
	if err != nil {
		t.Fatalf("LoadExistingFingerprints returned error: %v", err)
	}

	if !fps["fingerprint_abc123"] {
		t.Errorf("expected fingerprint_abc123 to be loaded")
	}
	if !fps["fingerprint_xyz789"] {
		t.Errorf("expected fingerprint_xyz789 to be loaded")
	}
	if fps["unknown"] {
		t.Errorf("unexpected fingerprint found")
	}
}

func TestCheckpoint_SaveAndLoad(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "checkpoint-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cpPath := filepath.Join(tempDir, "checkpoint.json")

	// Initially nil
	cp, err := LoadCheckpoint(cpPath)
	if err != nil {
		t.Fatalf("unexpected error loading non-existent checkpoint: %v", err)
	}
	if cp != nil {
		t.Fatalf("expected nil checkpoint, got %+v", cp)
	}

	// Save
	newCP := &Checkpoint{
		LatestSinceID: "2116817522318250115",
		ProcessedIDs:  []string{"2116817522318250115", "2116002151944401001"},
	}
	if err := SaveCheckpoint(cpPath, newCP); err != nil {
		t.Fatalf("failed to save checkpoint: %v", err)
	}

	// Load
	loaded, err := LoadCheckpoint(cpPath)
	if err != nil {
		t.Fatalf("failed to load checkpoint: %v", err)
	}
	if loaded == nil || loaded.LatestSinceID != "2116817522318250115" {
		t.Errorf("expected latest_since_id 2116817522318250115, got %v", loaded)
	}
	if len(loaded.ProcessedIDs) != 2 {
		t.Errorf("expected 2 processed_ids, got %d", len(loaded.ProcessedIDs))
	}
}

func TestLoadKnownDeduplicationKeys_ScheduleAndCheckpoint(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "dedup-keys-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	schedPath := filepath.Join(tempDir, "schedule.json")
	schedJSON := `{
		"sessions": [
			{
				"source_tweet_url": "https://x.com/Ribbon_Clock/status/2112891667640656194",
				"source_refs": [{"source_id": "2112891667640656194"}]
			}
		]
	}`
	_ = os.WriteFile(schedPath, []byte(schedJSON), 0644)

	cpPath := filepath.Join(tempDir, "checkpoint.json")
	cpJSON := `{
		"latest_since_id": "2116002151944401001",
		"processed_ids": ["2116002151944401001", "2115639776422625284"]
	}`
	_ = os.WriteFile(cpPath, []byte(cpJSON), 0644)

	keys, err := LoadKnownDeduplicationKeys("", schedPath, cpPath)
	if err != nil {
		t.Fatalf("failed to load deduplication keys: %v", err)
	}

	if !keys["2112891667640656194"] {
		t.Errorf("expected post id from schedule to be indexed")
	}
	if !keys["2116002151944401001"] {
		t.Errorf("expected post id from checkpoint to be indexed")
	}
	if !keys["2115639776422625284"] {
		t.Errorf("expected processed_id from checkpoint to be indexed")
	}
}
