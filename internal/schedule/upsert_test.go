package schedule

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/AobaIwaki123/taite-board/internal/resolve"
)

func TestUpsertPreservesSeparateSessions(t *testing.T) {
	raw := []byte(`{"schema_version":1,"timezone":"Asia/Tokyo","sessions":[{"id":"20261123-1","date":"2026-11-23","title":"公演","session_name":"1部","venue":{"canonical_id":"venue:hall","name":"Hall"},"live":{"start_time":"12:00"},"benefit":{"type":"並行","start_time":"--:--"}}]}`)
	for _, tc := range []struct {
		name string
		opts AddSessionOptions
	}{
		{"part", AddSessionOptions{Date: "2026-11-23", Title: "公演", SessionName: "2部", VenueName: "Hall", VenueCanonicalID: "venue:hall", LiveStartTime: "12:00"}},
		{"time", AddSessionOptions{Date: "2026-11-23", Title: "公演", VenueName: "Hall", VenueCanonicalID: "venue:hall", LiveStartTime: "13:00"}},
		{"venue", AddSessionOptions{Date: "2026-11-23", Title: "公演", VenueName: "Other", VenueCanonicalID: "venue:other", LiveStartTime: "12:00"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, id, err := ApplyAddSession(raw, tc.opts)
			if err != nil || id != "20261123-2" {
				t.Fatalf("id=%q err=%v", id, err)
			}
			var d struct {
				Sessions []json.RawMessage `json:"sessions"`
			}
			if err := json.Unmarshal(out, &d); err != nil || len(d.Sessions) != 2 {
				t.Fatalf("sessions=%d err=%v", len(d.Sessions), err)
			}
		})
	}
}

func TestUpsertAmbiguousRequiresTargetAndExactTimeMatches(t *testing.T) {
	raw := []byte(`{"schema_version":1,"timezone":"Asia/Tokyo","sessions":[{"id":"20261123-1","date":"2026-11-23","title":"昼公演","session_name":"1部","venue":{"canonical_id":"venue:hall","name":"Hall"},"live":{"start_time":"12:00"},"benefit":{"type":"並行","start_time":"--:--"}},{"id":"20261123-2","date":"2026-11-23","title":"夜公演","session_name":"2部","venue":{"canonical_id":"venue:hall","name":"Hall"},"live":{"start_time":"18:00"},"benefit":{"type":"並行","start_time":"--:--"}}]}`)
	if _, _, err := ApplyAddSession(raw, AddSessionOptions{Date: "2026-11-23", Title: "撮影可能"}); !errors.Is(err, ErrAmbiguousSession) {
		t.Fatalf("expected ambiguity, got %v", err)
	}
	for _, opts := range []AddSessionOptions{
		{Date: "2026-11-23", TargetSessionID: "20261123-2", PhotoRule: "撮影可能"},
		{Date: "2026-11-23", LiveStartTime: "18:00", PhotoRule: "撮影可能"},
	} {
		_, id, err := ApplyAddSession(raw, opts)
		if err != nil || id != "20261123-2" {
			t.Fatalf("id=%s err=%v", id, err)
		}
	}
}

func TestSourcePriorityUsesPerPostInformation(t *testing.T) {
	raw := []byte(`{"schema_version":1,"timezone":"Asia/Tokyo","sessions":[]}`)
	thin := AddSessionOptions{Date: "2026-11-23", Title: "ライブ出演 (2026-11-23)", VenueName: "未定", SourceTweetURL: "https://x.com/group/status/111"}
	first, _, err := ApplyAddSession(raw, thin)
	if err != nil {
		t.Fatal(err)
	}
	detail := AddSessionOptions{Date: "2026-11-23", Title: "詳細告知", VenueName: "Hall", LiveStartTime: "12:00", LiveEndTime: "12:20", TicketURL: "https://example.org/ticket", SourceTweetURL: "https://x.com/group/status/222"}
	second, _, err := ApplyAddSession(first, detail)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Sessions []map[string]interface{} `json:"sessions"`
	}
	if err := json.Unmarshal(second, &doc); err != nil {
		t.Fatal(err)
	}
	s := doc.Sessions[0]
	if stringField(s, "source_tweet_url") != detail.SourceTweetURL {
		t.Fatalf("wrong source: %v", s)
	}
	third, _, err := ApplyAddSession(second, AddSessionOptions{Date: "2026-11-23", SourceTweetURL: "https://x.com/group/status/333", PhotoRule: "撮影可能"})
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(third, &doc); err != nil {
		t.Fatal(err)
	}
	if stringField(doc.Sessions[0], "source_tweet_url") != detail.SourceTweetURL {
		t.Fatal("less detailed source replaced detailed source")
	}
}

func TestReconcilePromotesMoreDetailedSource(t *testing.T) {
	r, err := resolve.NewResolver("")
	if err != nil {
		t.Fatal(err)
	}
	dst := map[string]interface{}{"title": "ライブ出演 (2026-11-23)", "source_tweet_url": "https://x.com/group/status/111", "venue": map[string]interface{}{"name": "未定"}}
	src := map[string]interface{}{"title": "公演", "source_tweet_url": "https://x.com/group/status/222", "venue": map[string]interface{}{"name": "Hall"}, "live": map[string]interface{}{"start_time": "12:00"}}
	mergeReconciledSessions(dst, src, r)
	if stringField(dst, "source_tweet_url") != "https://x.com/group/status/222" {
		t.Fatalf("wrong source: %#v", dst)
	}
}
