package resolve

import (
	"testing"
)

func TestNormalizeText(t *testing.T) {
	input := "１１月７日　ＬＩＶＥ：１８：３０〜１９：００"
	expected := "11月7日 ＬＩＶＥ:18:30-19:00"
	actual := NormalizeText(input)
	if actual != expected {
		t.Errorf("NormalizeText mismatch:\nexpected: %s\ngot:      %s", expected, actual)
	}
}

func TestResolve_CompleteText(t *testing.T) {
	r, err := NewResolver("../../data/curated/entities.json")
	if err != nil {
		t.Fatalf("failed to init resolver: %v", err)
	}

	tweet := `
🎀リボンクロック 次回LIVE告知🎀
11月7日(土)
会場：渋谷RING
LIVE 18:30〜19:00
特典会 19:15〜20:15
動員特典あり！お待ちしています！
`
	cand, evidences, needsAI := r.Resolve(tweet, []string{"https://pbs.twimg.com/media/flyer.jpg"})

	if cand.GroupCanonicalID != "group:ribbon-clock" {
		t.Errorf("expected group:ribbon-clock, got %s", cand.GroupCanonicalID)
	}
	if cand.VenueCanonicalID != "venue:shibuya-ring" {
		t.Errorf("expected venue:shibuya-ring, got %s", cand.VenueCanonicalID)
	}
	if cand.Date != "2026-11-07" {
		t.Errorf("expected 2026-11-07, got %s", cand.Date)
	}
	if cand.LiveStartRaw != "18:30" || cand.LiveEndRaw != "19:00" {
		t.Errorf("expected 18:30-19:00, got %s-%s", cand.LiveStartRaw, cand.LiveEndRaw)
	}
	if cand.BenefitStartRaw != "19:15" || cand.BenefitEndRaw != "20:15" {
		t.Errorf("expected 19:15-20:15, got %s-%s", cand.BenefitStartRaw, cand.BenefitEndRaw)
	}
	if needsAI {
		t.Errorf("expected needsAI=false when all times present in text, got true")
	}
	if len(evidences) < 4 {
		t.Errorf("expected at least 4 evidence items, got %d", len(evidences))
	}
}

func TestResolve_ImageRequiresAI(t *testing.T) {
	r, err := NewResolver("../../data/curated/entities.json")
	if err != nil {
		t.Fatalf("failed to init resolver: %v", err)
	}

	tweet := `
🎀リボンクロック出演情報🎀
11/22(日) 新宿ZircoTokyo
タイムテーブルは画像をチェックしてね！
`
	images := []string{"https://pbs.twimg.com/media/timetable.jpg"}
	cand, _, needsAI := r.Resolve(tweet, images)

	if cand.Date != "2026-11-22" {
		t.Errorf("expected date 2026-11-22, got %s", cand.Date)
	}
	if cand.VenueCanonicalID != "venue:shinjuku-zirco-tokyo" {
		t.Errorf("expected venue:shinjuku-zirco-tokyo, got %s", cand.VenueCanonicalID)
	}
	if cand.LiveStartRaw != "" {
		t.Errorf("expected empty live start in text, got %s", cand.LiveStartRaw)
	}
	if !needsAI {
		t.Errorf("expected needsAI=true when timetable image present without text times, got false")
	}
}

func TestResolve_PerkAndPhotoPatch(t *testing.T) {
	r, err := NewResolver("../../data/curated/entities.json")
	if err != nil {
		t.Fatalf("failed to init resolver: %v", err)
	}

	tweet := `
🎀リボンクロック 明日11/4の特典情報🎀
🎁指名入場特典：宿題チェキ券＋集合写メ
📸撮影可能曲：【未来へのファンファーレ】動画撮影OK！
お待ちしております！
`
	cand, _, needsAI := r.Resolve(tweet, nil)

	if !cand.IsPatch {
		t.Errorf("expected cand.IsPatch=true, got false")
	}
	if cand.AdmissionBonus != "指名入場特典：宿題チェキ券＋集合写メ" {
		t.Errorf("expected admission bonus, got %q", cand.AdmissionBonus)
	}
	if cand.PhotoSong != "未来へのファンファーレ" {
		t.Errorf("expected photo song '未来へのファンファーレ', got %q", cand.PhotoSong)
	}
	if cand.PhotoDevice != "video_ok" {
		t.Errorf("expected photo device 'video_ok', got %q", cand.PhotoDevice)
	}
	if needsAI {
		t.Errorf("expected needsAI=false for deterministic perk/photo patch without timetable images, got true")
	}
}
