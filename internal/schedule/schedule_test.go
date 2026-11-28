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

func TestAddSession_UpsertDedupe(t *testing.T) {
	initial := []byte(`{
		"schema_version": 1,
		"generated_at": "2026-11-22T00:00:00Z",
		"timezone": "Asia/Tokyo",
		"target_group": {"name": "リボンクロック"},
		"sessions": [
			{
				"id": "20261123-1",
				"title": "ギガ盛りMAX 前夜祭＠新橋",
				"date": "2026-11-23",
				"day_of_week": "月祝",
				"venue": {"canonical_id": "venue:zeal-theater-shimbashi", "name": "ZEAL THEATER 新橋", "map_query": "ZEAL THEATER 新橋"},
				"live": {"start_time": "--:--", "end_time": "--:--", "stage": null},
				"benefit": {"type": "未定", "start_time": "--:--", "end_time": "--:--", "booth": "告知待ち"},
				"admission_bonus": "指名入場特典：お目当て3択",
				"ticket": {"platform": "LivePocket", "url": "https://livepocket.jp/e/x8-ap", "prices": [{"tier": "前方", "price": "¥4,000"}]}
			}
		]
	}`)

	mergedBytes, sessionID, err := ApplyAddSession(initial, AddSessionOptions{
		Date:             "2026-11-23",
		Title:            "ギガ盛りMAX 前夜祭@新橋",
		VenueName:        "ZEAL THEATER 新橋",
		VenueCanonicalID: "venue:zeal-theater-shimbashi",
		LiveStartTime:    "13:15",
		LiveEndTime:      "13:35",
		BenefitStartTime: "13:40",
		BenefitEndTime:   "14:40",
		Booth:            "並行A",
		SourceTweetURL:   "https://x.com/Ribbon_Clock/status/2124488067855863874",
	})
	if err != nil {
		t.Fatalf("ApplyAddSession upsert failed: %v", err)
	}
	if sessionID != "20261123-1" {
		t.Fatalf("expected existing session id 20261123-1, got %s", sessionID)
	}

	var doc struct {
		Sessions []map[string]any `json:"sessions"`
	}
	if err := json.Unmarshal(mergedBytes, &doc); err != nil {
		t.Fatalf("unmarshal merged doc: %v", err)
	}
	if len(doc.Sessions) != 1 {
		t.Fatalf("expected 1 deduped session, got %d", len(doc.Sessions))
	}
	s := doc.Sessions[0]
	live := s["live"].(map[string]any)
	benefit := s["benefit"].(map[string]any)
	ticket := s["ticket"].(map[string]any)
	if live["start_time"] != "13:15" || live["end_time"] != "13:35" {
		t.Errorf("expected live time updated, got %v", live)
	}
	if benefit["booth"] != "並行A" || benefit["start_time"] != "13:40" {
		t.Errorf("expected benefit booth & time updated, got %v", benefit)
	}
	if ticket["url"] != "https://livepocket.jp/e/x8-ap" {
		t.Errorf("expected existing ticket URL preserved, got %v", ticket)
	}

	// Verify photo regulation works on both new session and upsert
	photoMerged, _, err := ApplyAddSession(mergedBytes, AddSessionOptions{
		Date:        "2026-11-23",
		Title:       "ギガ盛りMAX 前夜祭@新橋",
		VenueName:   "ZEAL THEATER 新橋",
		PhotoSong:   "えすえぬ・えーす",
		PhotoDevice: "photo_ok",
	})
	if err != nil {
		t.Fatalf("ApplyAddSession photo upsert failed: %v", err)
	}
	var photoDoc struct {
		Sessions []map[string]any `json:"sessions"`
	}
	_ = json.Unmarshal(photoMerged, &photoDoc)
	photoMap, ok := photoDoc.Sessions[0]["photo"].(map[string]any)
	if !ok || photoMap["available"] != true || photoMap["rule"] != "撮可: えすえぬ・えーす" {
		t.Errorf("expected photo map populated on upsert, got %+v", photoDoc.Sessions[0]["photo"])
	}
}

func TestApplyAddSession_TicketPricesCreateAndPreserve(t *testing.T) {
	initial := []byte(`{"schema_version":1,"timezone":"Asia/Tokyo","target_group":{"name":"group"},"sessions":[]}`)
	created, id, err := ApplyAddSession(initial, AddSessionOptions{
		Date:      "2026-11-30",
		Title:     "無銭対バン",
		VenueName: "渋谷RING",
		TicketURL: "https://tiget.net/events/123",
		TicketPrices: []TicketPrice{
			{Tier: "前方", Price: "¥3,500 (+1D)", Amount: 3500},
			{Tier: "一般", Price: "無料", Amount: 0},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if id == "" {
		t.Fatal("expected generated id")
	}
	var doc struct {
		Sessions []struct {
			Ticket struct {
				Platform string                   `json:"platform"`
				URL      string                   `json:"url"`
				Prices   []map[string]interface{} `json:"prices"`
			} `json:"ticket"`
		} `json:"sessions"`
	}
	if err := json.Unmarshal(created, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Sessions) != 1 || len(doc.Sessions[0].Ticket.Prices) != 2 {
		t.Fatalf("expected 2 ticket prices saved, got %+v", doc.Sessions)
	}
	if doc.Sessions[0].Ticket.Prices[1]["amount"] != float64(0) {
		t.Errorf("expected free tier to keep integer amount 0, got %+v", doc.Sessions[0].Ticket.Prices[1])
	}

	// Merging new extracted prices MUST NOT overwrite existing non-empty prices
	merged, _, err := ApplyAddSession(created, AddSessionOptions{
		Date:            "2026-11-30",
		TargetSessionID: id,
		TicketPrices: []TicketPrice{
			{Tier: "上書きテスト", Price: "¥9,999", Amount: 9999},
		},
	})
	if err != nil {
		t.Fatalf("merge failed: %v", err)
	}
	doc.Sessions = nil
	if err := json.Unmarshal(merged, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Sessions[0].Ticket.Prices) != 2 || doc.Sessions[0].Ticket.Prices[0]["tier"] != "前方" {
		t.Fatalf("existing curated prices were overwritten: %+v", doc.Sessions[0].Ticket.Prices)
	}
}
