package schedule

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSync(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "source.json")
	target1 := filepath.Join(tmpDir, "sub1", "target1.json")
	target2 := filepath.Join(tmpDir, "sub2", "target2.json")

	content := []byte(`{"hello": "world"}` + "\n")
	if err := os.WriteFile(srcFile, content, 0644); err != nil {
		t.Fatalf("writing source: %v", err)
	}

	hash, err := Sync(srcFile, target1, target2)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
	if len(hash) != 64 {
		t.Errorf("expected 64-char sha256, got %s", hash)
	}

	for _, p := range []string{target1, target2} {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Errorf("reading %s: %v", p, err)
		}
		if string(data) != string(content) {
			t.Errorf("content mismatch in %s: got %s, want %s", p, data, content)
		}
	}
}

func TestUpdateSession(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "schedule.json")
	targetCopy := filepath.Join(tmpDir, "copy.json")

	initial := map[string]interface{}{
		"schema_version": 1,
		"generated_at":   "2026-10-01T00:00:00Z",
		"timezone":       "Asia/Tokyo",
		"target_group": map[string]interface{}{
			"name": "Test Group",
		},
		"sessions": []interface{}{
			map[string]interface{}{
				"id":               "sess-1",
				"title":            "Live 1",
				"source_tweet_url": "https://x.com/group/status/1111111111",
				"special_notes":    "Original notes",
				"photo":            nil,
			},
		},
	}
	bytes, _ := json.Marshal(initial)
	if err := os.WriteFile(srcFile, bytes, 0644); err != nil {
		t.Fatalf("writing test schedule: %v", err)
	}

	err := UpdateSession(UpdateSessionOptions{
		SessionID:         "sess-1",
		PhotoRule:         "1曲目撮影可能",
		PhotoRawText:      "1曲目撮影可能です！",
		SourceTweetURL:    "https://x.com/group/status/2222222222",
		UpdateOfficialURL: true,
		SpecialNotes:      "私服ライブ",
		FilePath:          srcFile,
		SyncTargets:       []string{targetCopy},
	})
	if err != nil {
		t.Fatalf("UpdateSession failed: %v", err)
	}

	// Read back and verify
	readBytes, err := os.ReadFile(srcFile)
	if err != nil {
		t.Fatalf("reading updated schedule: %v", err)
	}
	var updated map[string]interface{}
	_ = json.Unmarshal(readBytes, &updated)

	sessions := updated["sessions"].([]interface{})
	sess1 := sessions[0].(map[string]interface{})

	// Verify photo
	photo := sess1["photo"].(map[string]interface{})
	if photo["rule"] != "1曲目撮影可能" || photo["available"] != true {
		t.Errorf("photo rule mismatch: %v", photo)
	}

	// Verify official tweet url updated
	if sess1["source_tweet_url"] != "https://x.com/group/status/2222222222" {
		t.Errorf("source_tweet_url not updated: %v", sess1["source_tweet_url"])
	}

	// Verify source_refs appended
	refs := sess1["source_refs"].([]interface{})
	if len(refs) != 1 {
		t.Fatalf("expected 1 ref, got %d", len(refs))
	}
	r0 := refs[0].(map[string]interface{})
	if r0["source_id"] != "2222222222" {
		t.Errorf("expected source_id 2222222222, got %v", r0["source_id"])
	}

	// Verify notes merged
	if sess1["special_notes"] != "私服ライブ / Original notes" {
		t.Errorf("special_notes mismatch: %v", sess1["special_notes"])
	}

	// Verify sync target has same content
	targetBytes, err := os.ReadFile(targetCopy)
	if err != nil {
		t.Fatalf("reading sync copy: %v", err)
	}
	if string(targetBytes) != string(readBytes) {
		t.Errorf("targetCopy mismatch with srcFile")
	}
}
