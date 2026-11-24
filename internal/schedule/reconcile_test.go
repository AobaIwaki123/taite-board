package schedule

import (
	"encoding/json"
	"testing"

	"github.com/AobaIwaki123/taite-board/internal/resolve"
)

func TestApplyReconcileSchedule_MergesDuplicatesAndNormalizesEntities(t *testing.T) {
	input := `{
  "schema_version": 1,
  "generated_at": "2026-11-23T00:00:00+09:00",
  "timezone": "Asia/Tokyo",
  "target_group": { "name": "リボンクロック", "canonical_id": "group:ribbon-clock" },
  "sessions": [
    {
      "id": "20261123-1",
      "title": "🎀ギガ盛りMAX 前夜祭＠新橋🎀",
      "date": "2026-11-23",
      "day_of_week": "月祝",
      "venue": { "canonical_id": "venue:zeal-theater-tokyo", "name": "ZEAL THEATER TOKYO" },
      "live": { "start_time": "--:--", "end_time": "", "stage": null },
      "benefit": { "type": "並行", "start_time": "--:--", "end_time": "", "booth": "ロビ-B" },
      "admission_bonus": "📸特典会：13:40〜14:40（並行A）",
      "ticket": { "platform": "LivePocket", "url": "https://livepocket.jp/e/x8-ap", "prices": [] },
      "source_tweet_url": "https://x.com/Ribbon_Clock/status/111"
    },
    {
      "id": "20261123-2",
      "title": "ギガ盛りMAX 前夜祭＠新橋",
      "date": "2026-11-23",
      "day_of_week": "月祝",
      "venue": { "canonical_id": "venue:zeal-theater-shimbashi", "name": "ZEAL THEATER 新橋" },
      "live": { "start_time": "13:15", "end_time": "13:35", "stage": null },
      "benefit": { "type": "並行", "start_time": "13:40", "end_time": "14:40", "booth": "並行A" },
      "admission_bonus": "指名入場特典：お目当て3択",
      "ticket": { "platform": "", "url": "https://livepocket.jp/e/x8-ap", "prices": [] },
      "source_tweet_url": "https://x.com/Ribbon_Clock/status/222"
    },
    {
      "id": "20261103-1",
      "title": "アイドルマリアージュVol.79",
      "date": "2026-11-03",
      "day_of_week": "火祝",
      "venue": { "canonical_id": "venue:bellesalle-toranomon", "name": "ベルサール虎ノ門" },
      "live": { "start_time": "14:55", "end_time": "15:15", "stage": null },
      "benefit": { "type": "並行", "start_time": "15:20", "end_time": "16:20", "booth": "A" },
      "ticket": { "platform": "TicketDive", "url": "https://ticketdive.com/event/a", "prices": [] },
      "source_tweet_url": "https://x.com/Ribbon_Clock/status/333"
    },
    {
      "id": "20261103-2",
      "title": "＠JAM×アイドル横丁presents 秋葉原アイドルサーキットvol.6",
      "date": "2026-11-03",
      "day_of_week": "火祝",
      "venue": { "canonical_id": "", "name": "神田明神ホール" },
      "live": { "start_time": "18:40", "end_time": "19:00", "stage": null },
      "benefit": { "type": "並行", "start_time": "19:10", "end_time": "20:10", "booth": "C" },
      "photo": { "available": true, "rule": "1曲目『えすえぬ・えーす』撮影可能", "raw_text": "1曲目『えすえぬ・えーす』撮影可能" },
      "ticket": { "platform": "", "url": "https://tiget.net/events/444", "prices": [] },
      "source_tweet_url": "https://x.com/Ribbon_Clock/status/333"
    }
  ]
}`

	res, err := resolve.NewResolver("")
	if err != nil {
		t.Fatalf("NewResolver failed: %v", err)
	}

	out, report, err := ApplyReconcileSchedule([]byte(input), res)
	if err != nil {
		t.Fatalf("ApplyReconcileSchedule failed: %v", err)
	}

	if !report.Changed || report.BeforeCount != 4 || report.AfterCount != 3 || report.MergedCount != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}

	var doc struct {
		Sessions []struct {
			ID             string `json:"id"`
			Title          string `json:"title"`
			Date           string `json:"date"`
			AdmissionBonus string `json:"admission_bonus"`
			Venue          struct {
				CanonicalID string `json:"canonical_id"`
				Name        string `json:"name"`
			} `json:"venue"`
			Live struct {
				StartTime string `json:"start_time"`
			} `json:"live"`
			Benefit struct {
				Booth string `json:"booth"`
			} `json:"benefit"`
			Photo *struct {
				SongNumber   int      `json:"song_number"`
				AllowedSongs []string `json:"allowed_songs"`
			} `json:"photo"`
			Ticket struct {
				Platform string `json:"platform"`
			} `json:"ticket"`
			SourceRefs []map[string]string `json:"source_refs"`
		} `json:"sessions"`
	}
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("unmarshal reconciled output: %v", err)
	}
	if len(doc.Sessions) != 3 {
		t.Fatalf("expected 3 sessions after merge, got %d", len(doc.Sessions))
	}

	// Sorted chronologically -> 2026-11-03 first, then 2026-11-23
	if doc.Sessions[0].ID != "20261103-1" || doc.Sessions[1].ID != "20261103-2" {
		t.Errorf("expected two distinct 2026-11-03 sessions preserved in order, got %s, %s", doc.Sessions[0].ID, doc.Sessions[1].ID)
	}
	if doc.Sessions[1].Venue.CanonicalID != "venue:神田明神ホール" {
		t.Errorf("expected fallback venue canonical_id on unknown venue, got %q", doc.Sessions[1].Venue.CanonicalID)
	}
	if doc.Sessions[1].Ticket.Platform != "TIGET" {
		t.Errorf("expected TIGET platform detected, got %q", doc.Sessions[1].Ticket.Platform)
	}
	if doc.Sessions[1].Photo == nil || doc.Sessions[1].Photo.SongNumber != 1 || len(doc.Sessions[1].Photo.AllowedSongs) == 0 || doc.Sessions[1].Photo.AllowedSongs[0] != "えすえぬ・えーす" {
		t.Errorf("expected backfilled photo.song_number=1 & allowed_songs, got %+v", doc.Sessions[1].Photo)
	}

	merged := doc.Sessions[2]
	if merged.ID != "20261123-1" {
		t.Errorf("expected survivor ID 20261123-1, got %s", merged.ID)
	}
	if merged.Title != "ギガ盛りMAX 前夜祭＠新橋" {
		t.Errorf("expected cleaned title without emojis, got %q", merged.Title)
	}
	if merged.Venue.CanonicalID != "venue:zeal-theater-shimbashi" || merged.Venue.Name != "ZEAL THEATER 新橋" {
		t.Errorf("expected canonicalized ZEAL THEATER 新橋, got %+v", merged.Venue)
	}
	if merged.Live.StartTime != "13:15" {
		t.Errorf("expected live time 13:15 absorbed, got %q", merged.Live.StartTime)
	}
	if merged.AdmissionBonus != "指名入場特典：お目当て3択" {
		t.Errorf("expected stray bonus replaced with valid bonus, got %q", merged.AdmissionBonus)
	}
	if len(merged.SourceRefs) != 2 {
		t.Errorf("expected 2 merged source_refs, got %+v", merged.SourceRefs)
	}

	// Idempotency check: second run should report zero changes
	_, report2, err := ApplyReconcileSchedule(out, res)
	if err != nil {
		t.Fatalf("second ApplyReconcileSchedule failed: %v", err)
	}
	if report2.Changed {
		t.Errorf("expected idempotent second run (Changed=false), got changes: %+v", report2.Changes)
	}
}
