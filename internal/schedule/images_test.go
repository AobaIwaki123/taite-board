package schedule

import (
	"encoding/json"
	"testing"
)

func TestDedupeImagesKeepsDistinctMediaAndBestSize(t *testing.T) {
	s := map[string]interface{}{"images": []interface{}{
		map[string]interface{}{"url": "https://pbs.twimg.com/media/ABC?format=jpg&name=small"},
		map[string]interface{}{"url": "http://pbs.twimg.com/media/ABC.jpg?name=orig"},
		map[string]interface{}{"url": "https://pbs.twimg.com/media/DEF.jpg?name=large"},
	}}
	if !dedupeImages(s) {
		t.Fatal("expected duplicate to be removed")
	}
	imgs := s["images"].([]interface{})
	if len(imgs) != 2 || stringField(imgs[0].(map[string]interface{}), "url") != "http://pbs.twimg.com/media/ABC.jpg?name=orig" {
		t.Fatalf("unexpected images: %#v", imgs)
	}
}

func TestUpsertSupplementOnSingleDate(t *testing.T) {
	raw := []byte(`{"schema_version":1,"timezone":"Asia/Tokyo","sessions":[{"id":"20261123-1","date":"2026-11-23","title":"公演","venue":{"canonical_id":"venue:hall","name":"Hall"},"live":{"start_time":"12:00"},"benefit":{"type":"並行","start_time":"--:--"}}]}`)
	updated, id, err := ApplyAddSession(raw, AddSessionOptions{Date: "2026-11-23", PhotoRule: "撮影可能"})
	if err != nil || id != "20261123-1" {
		t.Fatalf("id=%s err=%v", id, err)
	}
	var doc struct {
		Sessions []map[string]interface{} `json:"sessions"`
	}
	if err := json.Unmarshal(updated, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Sessions) != 1 || doc.Sessions[0]["photo"] == nil {
		t.Fatalf("unexpected sessions: %#v", doc.Sessions)
	}
}

func TestUpsertAmbiguousDateFails(t *testing.T) {
	raw := []byte(`{"schema_version":1,"timezone":"Asia/Tokyo","sessions":[{"id":"20261123-1","date":"2026-11-23","title":"昼","session_name":"1部","venue":{"canonical_id":"venue:hall"},"live":{"start_time":"12:00"}},{"id":"20261123-2","date":"2026-11-23","title":"夜","session_name":"2部","venue":{"canonical_id":"venue:hall"},"live":{"start_time":"18:00"}}]}`)
	if _, _, err := ApplyAddSession(raw, AddSessionOptions{Date: "2026-11-23", PhotoRule: "撮影可能"}); err == nil {
		t.Fatal("expected ambiguous match error")
	}
}

func TestMergeSessionFieldsAccumulatesImages(t *testing.T) {
	s := map[string]interface{}{"images": []interface{}{map[string]interface{}{"url": "https://pbs.twimg.com/media/ABC.jpg"}}}
	mergeSessionFields(s, AddSessionOptions{ImageURL: "https://pbs.twimg.com/media/DEF.jpg"}, nil, "", "", nil)
	if imgs := s["images"].([]interface{}); len(imgs) != 2 {
		t.Fatalf("expected both images: %#v", imgs)
	}
}

func TestApplyAddSession_MultipleImageURLs(t *testing.T) {
	initial := []byte(`{"schema_version":1,"timezone":"Asia/Tokyo","sessions":[]}`)
	updated, id, err := ApplyAddSession(initial, AddSessionOptions{
		Date:      "2026-11-25",
		Title:     "POLAROID",
		VenueName: "原宿RUIDO",
		ImageURLs: []string{
			"https://pbs.twimg.com/media/A.jpg?name=large",
			"https://pbs.twimg.com/media/B.jpg?name=large",
		},
	})
	if err != nil || id == "" {
		t.Fatalf("add multi-image failed: id=%s err=%v", id, err)
	}
	var doc struct {
		Sessions []struct {
			ImageURL string `json:"image_url"`
			Images   []struct {
				URL string `json:"url"`
			} `json:"images"`
		} `json:"sessions"`
	}
	if err := json.Unmarshal(updated, &doc); err != nil {
		t.Fatal(err)
	}
	if got := doc.Sessions[0]; got.ImageURL == "" || len(got.Images) != 2 {
		t.Fatalf("expected 2 images, got %+v", got)
	}
}
