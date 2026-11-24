package resolve

import (
	"strings"
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

func TestResolve_VenueAliasesAndBoothAndTicketURL(t *testing.T) {
	r, err := NewResolver("")
	if err != nil {
		t.Fatalf("failed to init default resolver: %v", err)
	}

	tweet := `🎀『POLAROID - FREE -』🎀

🗓️日付：12月2日（水）
🎪会場：原宿RUIDO
🎤出演：18:25〜18:45
📸特典会：18:50〜19:50（並行A）

🪄お目当て特典：下記3つの中から選択
① 10秒動画券（複数使用可）
② 写真＋30秒トーク（複数使用不可）
③ 券5枚⇒集合写真/券10枚⇒私物サイン

❤︎下記リンクにてチケット販売中！❤︎
http://ticketdive.com/event/261202_PF`

	cand, _, needsAI := r.Resolve(tweet, []string{"https://pbs.twimg.com/media/a.jpg"})
	if needsAI {
		t.Errorf("expected needsAI=false when live start & end are present")
	}
	if cand.Title != "POLAROID - FREE -" {
		t.Errorf("expected clean title 'POLAROID - FREE -', got %q", cand.Title)
	}
	if cand.Date != "2026-12-02" {
		t.Errorf("expected 2026-12-02, got %q", cand.Date)
	}
	if cand.BenefitStartRaw != "18:50" || cand.BenefitEndRaw != "19:50" {
		t.Errorf("expected benefit 18:50-19:50, got %s-%s", cand.BenefitStartRaw, cand.BenefitEndRaw)
	}
	if cand.BenefitBoothRaw != "並行A" {
		t.Errorf("expected benefit booth '並行A', got %q", cand.BenefitBoothRaw)
	}
	if cand.TicketURL != "http://ticketdive.com/event/261202_PF" {
		t.Errorf("expected ticket URL extracted, got %q", cand.TicketURL)
	}
	if !strings.Contains(cand.AdmissionBonus, "① 10秒動画券") || strings.Contains(cand.AdmissionBonus, "特典会") {
		t.Errorf("unexpected admission bonus: %q", cand.AdmissionBonus)
	}

	cid, name := r.ResolveVenueName("ZEAL THEATER TOKYO")
	if cid != "venue:zeal-theater-shimbashi" || name != "ZEAL THEATER 新橋" {
		t.Errorf("expected ZEAL THEATER TOKYO to resolve to ZEAL THEATER 新橋, got (%q, %q)", cid, name)
	}

	lCid, lName := r.ResolveVenueName("LUMINE 0")
	if lCid != "venue:lumine-0" || lName != "LUMINE 0" {
		t.Errorf("expected LUMINE 0 canonical resolution, got (%q, %q)", lCid, lName)
	}

	timetreeText := `🎀『アイコレ♡君ときらめく冬のはじまりSP!!!』🎀
🗓️日付：11/28(土)
🎪会場：LUMINE 0
🎤出演：10:45〜11:00
📸特典会：11:05〜12:05（ロビーB）
https://tiget.net/events/537396`
	ttCand, _, _ := r.Resolve(timetreeText, nil)
	if ttCand.BenefitBoothRaw != "ロビーB" {
		t.Errorf("expected katakana booth 'ロビーB' without hyphen mangling, got %q", ttCand.BenefitBoothRaw)
	}

	photoNewlineTweet := `／／
明日のライブ
＼＼

🎀『POLAROID - FREE -』🎀

🗓️日付：11/25（水）
🎪会場：原宿RUIDO
🎤出演：18:45〜19:05
📸特典会：19:10〜20:10（並行B）

明日の撮影可能曲は、
1曲目『えすえぬ・えーす』です！

❤︎チケットは下記リンクにて発売中！❤︎
https://ticketdive.com/event/261125_PF`
	pCand, _, pNeedsAI := r.Resolve(photoNewlineTweet, []string{"https://pbs.twimg.com/media/a.jpg"})
	if pNeedsAI {
		t.Errorf("expected needsAI=false for live + photo tweet")
	}
	if pCand.PhotoSong != "えすえぬ・えーす" {
		t.Errorf("expected multiline photo song 'えすえぬ・えーす', got %q", pCand.PhotoSong)
	}
	if pCand.PhotoAllowed == nil || !*pCand.PhotoAllowed || pCand.PhotoDevice != "photo_ok" {
		t.Errorf("expected photo_ok allowed, got allowed=%v device=%q", pCand.PhotoAllowed, pCand.PhotoDevice)
	}
	if pCand.BenefitBoothRaw != "並行B" || pCand.TicketURL != "https://ticketdive.com/event/261125_PF" {
		t.Errorf("unexpected booth/ticket: %+v", pCand)
	}
}
