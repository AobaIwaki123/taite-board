package timetreesync

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/AobaIwaki123/taite-board/internal/gcs"
	"github.com/AobaIwaki123/taite-board/internal/schedule"
	"github.com/AobaIwaki123/taite-board/internal/source/timetree"
)

type fakeSource struct {
	events map[string][]timetree.Event
	calls  []string
	err    error
}

func (s *fakeSource) ListEvents(_ context.Context, id string) ([]timetree.Event, error) {
	s.calls = append(s.calls, id)
	return s.events[id], s.err
}

type fakeObject struct {
	data       []byte
	generation int
}
type fakeStore struct {
	objects   map[string]fakeObject
	failFirst bool
	writes    int
}

func (s *fakeStore) GetObjectVersion(_ context.Context, _, object string) ([]byte, string, error) {
	v, ok := s.objects[object]
	if !ok {
		return nil, "", gcs.ErrObjectNotFound
	}
	return append([]byte(nil), v.data...), string(rune('0' + v.generation)), nil
}
func (s *fakeStore) PutObjectIfGeneration(_ context.Context, _, object, _, _ string, data []byte, gen string) error {
	v := s.objects[object]
	if s.failFirst {
		s.failFirst = false
		v.generation++
		var doc map[string]interface{}
		_ = json.Unmarshal(v.data, &doc)
		doc["concurrent_update"] = "preserved"
		v.data, _ = json.Marshal(doc)
		s.objects[object] = v
		for _, mirror := range []string{"schedule.json", "mock_schedule.json"} {
			if m, ok := s.objects[mirror]; ok {
				m.data = append([]byte(nil), v.data...)
				m.generation++
				s.objects[mirror] = m
			}
		}
		return gcs.ErrGenerationMismatch
	}
	if gen != string(rune('0'+v.generation)) {
		return gcs.ErrGenerationMismatch
	}
	v.generation++
	v.data = append([]byte(nil), data...)
	s.objects[object] = v
	s.writes++
	return nil
}

func TestExtractUsesExplicitTimesOnly(t *testing.T) {
	event := timetree.Event{ID: "1", Title: "ライブ@新宿", StartAt: "2026-11-28T09:00:00+09:00", AllDay: true, Description: "🎀『アイコレ』🎀\n🎪会場：LUMINE 0\n🎤出演：10:45〜11:00\n📸特典会：11:05〜12:05（ロビーB）\n🎫tiget.net/events/123"}
	opts, err := Extract("ribbon_clock", event)
	if err != nil {
		t.Fatal(err)
	}
	if opts.Date != "2026-11-28" || opts.Title != "アイコレ" || opts.VenueName != "LUMINE 0" || opts.LiveStartTime != "10:45" || opts.BenefitStartTime != "11:05" || opts.TicketURL != "https://tiget.net/events/123" {
		t.Fatalf("unexpected extraction: %+v", opts)
	}
	event.Description = "開場 10:30〜11:00"
	opts, err = Extract("ribbon_clock", event)
	if err != nil {
		t.Fatal(err)
	}
	if opts.LiveStartTime != "" || opts.BenefitStartTime != "" {
		t.Fatalf("unlabelled time treated as performance: %+v", opts)
	}
	event.URL = "https://eplus.jp/sf/example"
	opts, err = Extract("ribbon_clock", event)
	if err != nil || opts.SourceTweetURL != "https://timetr.ee/p/ribbon_clock/1" || opts.TicketURL != event.URL {
		t.Fatalf("source and ticket confused: %+v, %v", opts, err)
	}
	event.Title = "公演 第２部"
	opts, err = Extract("ribbon_clock", event)
	if err != nil || opts.SessionName != "2部" {
		t.Fatalf("session label not extracted: %+v %v", opts, err)
	}
	event.Title = "公演 1部・2部"
	if _, err := Extract("ribbon_clock", event); err == nil {
		t.Fatal("expected ambiguous session label")
	}
}

func TestRunSequentialIdempotentAndPreservesConfirmedData(t *testing.T) {
	initial := []byte(`{"schema_version":1,"generated_at":"2026-11-01T00:00:00Z","timezone":"Asia/Tokyo","target_group":{"name":"group"},"custom_root":{"preserve":true},"sessions":[{"id":"20261128-1","date":"2026-11-28","title":"アイコレ","venue":{"canonical_id":"venue:hall","name":"LUMINE 0"},"live":{"start_time":"10:45","end_time":"11:00"},"benefit":{"type":"並行","start_time":"11:05","end_time":"12:05"},"ticket":{"url":"https://old.example","prices":[]},"source_refs":[{"type":"timetree_event","source_id":"111","url":"https://timetr.ee/p/ribbon_clock/111"}]}]}`)
	other := []byte(`{"schema_version":1,"generated_at":"2026-11-01T00:00:00Z","timezone":"Asia/Tokyo","target_group":{"name":"group"},"sessions":[]}`)
	store := &fakeStore{objects: map[string]fakeObject{"data/groups/ribonclock/schedule.json": {initial, 1}, "schedule.json": {initial, 1}, "mock_schedule.json": {initial, 1}, "data/groups/dollykiss/schedule.json": {other, 1}}, failFirst: true}
	source := &fakeSource{events: map[string][]timetree.Event{
		"ribbon_clock": {{ID: "111", Title: "アイコレ", StartAt: "2026-11-28T09:00:00+09:00", Description: "🎪会場：LUMINE 0\n🎤出演：10:50〜11:10\n📸特典会：11:05〜12:05\n🎫tiget.net/events/123", ImageUrls: []string{"https://example.test/flyer.jpg", "https://example.test/timetable.jpg"}}},
		"lui_17707":    {{ID: "222", Title: "別公演", StartAt: "2026-11-29T09:00:00+09:00", Description: "📍大塚\n🎤12:40-13:05"}},
	}}
	from := time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC)
	opts := Options{Source: source, Store: store, Bucket: "test", Targets: []Target{{"ribonclock", "ribbon_clock"}, {"dollykiss", "lui_17707"}}, From: from, Until: from.AddDate(0, 2, 0)}
	summary, err := Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("run: %v %+v", err, summary)
	}
	if strings.Join(source.calls, ",") != "ribbon_clock,lui_17707" || store.writes != 4 {
		t.Fatalf("calls=%v writes=%d", source.calls, store.writes)
	}
	var doc map[string]interface{}
	if err := json.Unmarshal(store.objects["data/groups/ribonclock/schedule.json"].data, &doc); err != nil {
		t.Fatal(err)
	}
	s := doc["sessions"].([]interface{})[0].(map[string]interface{})
	if s["live"].(map[string]interface{})["start_time"] != "10:45" || s["ticket"].(map[string]interface{})["url"] != "https://old.example" || len(s["images"].([]interface{})) != 2 || doc["custom_root"] == nil || doc["concurrent_update"] != "preserved" {
		t.Fatalf("confirmed data not preserved: %+v", doc)
	}
	before := store.writes
	summary, err = Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("repeat: %v %+v", err, summary)
	}
	if store.writes != before {
		t.Fatalf("repeat wrote %d additional objects", store.writes-before)
	}
}

func TestRunDoesNotMergeUnidentifiedSameDayEvent(t *testing.T) {
	initial := []byte(`{"schema_version":1,"generated_at":"2026-11-01T00:00:00Z","timezone":"Asia/Tokyo","target_group":{"name":"group"},"sessions":[{"id":"20261129-1","date":"2026-11-29","title":"公演A","venue":{"name":"未定"},"live":{"start_time":"--:--"},"benefit":{"type":"並行","start_time":"--:--"},"ticket":{"prices":[]}}]}`)
	store := &fakeStore{objects: map[string]fakeObject{"data/groups/dollykiss/schedule.json": {initial, 1}}}
	source := &fakeSource{events: map[string][]timetree.Event{"cal": {{ID: "2", Title: "ライブ予定", StartAt: "2026-11-29T09:00:00+09:00"}}}}
	from := time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC)
	summary, err := Run(context.Background(), Options{Source: source, Store: store, Bucket: "test", Targets: []Target{{"dollykiss", "cal"}}, From: from, Until: from.AddDate(0, 1, 0)})
	if err != nil || len(summary.Results[0].Conflicts) != 1 || store.writes != 0 {
		t.Fatalf("unexpected run: %+v, %v writes=%d", summary, err, store.writes)
	}
}

func TestRunFetchFailureDoesNotWrite(t *testing.T) {
	source := &fakeSource{err: errors.New("unavailable")}
	store := &fakeStore{objects: map[string]fakeObject{}}
	from := time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC)
	summary, err := Run(context.Background(), Options{Source: source, Store: store, Bucket: "test", Targets: []Target{{"x", "x"}}, From: from, Until: from.AddDate(0, 1, 0)})
	if err == nil || summary.Results[0].Error == "" || store.writes != 0 {
		t.Fatalf("expected failure without writes: %+v %v", summary, err)
	}
}

func TestMovedOrMissingEventDoesNotEraseExistingSession(t *testing.T) {
	initial := []byte(`{"schema_version":1,"generated_at":"2026-11-01T00:00:00Z","timezone":"Asia/Tokyo","target_group":{"name":"group"},"sessions":[{"id":"20261129-1","date":"2026-11-29","title":"公演A","venue":{"name":"Hall"},"live":{"start_time":"--:--"},"benefit":{"type":"並行","start_time":"--:--"},"ticket":{"prices":[]},"source_refs":[{"type":"timetree_event","source_id":"123","url":"https://timetr.ee/p/cal/123"}]}]}`)
	store := &fakeStore{objects: map[string]fakeObject{"data/groups/dollykiss/schedule.json": {initial, 1}}}
	source := &fakeSource{events: map[string][]timetree.Event{"cal": {{ID: "123", Title: "公演A", StartAt: "2026-11-30T09:00:00+09:00"}}}}
	from := time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC)
	opts := Options{Source: source, Store: store, Bucket: "test", Targets: []Target{{"dollykiss", "cal"}}, From: from, Until: from.AddDate(0, 1, 0)}
	summary, err := Run(context.Background(), opts)
	if err != nil || len(summary.Results[0].Conflicts) != 1 || store.writes != 0 {
		t.Fatalf("moved event: %+v, %v", summary, err)
	}
	source.events["cal"] = []timetree.Event{}
	if _, err = Run(context.Background(), opts); err != nil || store.writes != 0 || string(store.objects["data/groups/dollykiss/schedule.json"].data) != string(initial) {
		t.Fatalf("missing event erased data: %v", err)
	}
}

func TestMirrorDriftStopsBeforePrimaryWrite(t *testing.T) {
	initial := []byte(`{"schema_version":1,"generated_at":"2026-11-01T00:00:00Z","timezone":"Asia/Tokyo","target_group":{"name":"group"},"sessions":[]}`)
	other := []byte(`{"schema_version":1,"generated_at":"2026-11-01T00:00:00Z","timezone":"Asia/Tokyo","target_group":{"name":"different"},"sessions":[]}`)
	store := &fakeStore{objects: map[string]fakeObject{"data/groups/ribonclock/schedule.json": {initial, 1}, "schedule.json": {other, 1}, "mock_schedule.json": {initial, 1}}}
	source := &fakeSource{events: map[string][]timetree.Event{"cal": {{ID: "123", Title: "公演A", StartAt: "2026-11-30T09:00:00+09:00"}}}}
	from := time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC)
	_, err := Run(context.Background(), Options{Source: source, Store: store, Bucket: "test", Targets: []Target{{"ribonclock", "cal"}}, From: from, Until: from.AddDate(0, 1, 0)})
	if err == nil || store.writes != 0 {
		t.Fatalf("expected mismatch without writes: %v writes=%d", err, store.writes)
	}
}

type stubImageFP struct {
	sigs map[string]schedule.ImageSignature
}

func (s *stubImageFP) Fingerprint(_ context.Context, url string) (schedule.ImageSignature, error) {
	if sig, ok := s.sigs[url]; ok {
		return sig, nil
	}
	return schedule.ImageSignature{}, errors.New("not found")
}

func TestVisualDedupePreventsDuplicateFlyerAcrossTimeTreeAndX(t *testing.T) {
	initial := []byte(`{"schema_version":1,"generated_at":"2026-11-01T00:00:00Z","timezone":"Asia/Tokyo","target_group":{"name":"group"},"sessions":[{"id":"20261129-1","date":"2026-11-29","title":"公演A","venue":{"name":"Hall"},"live":{"start_time":"12:00"},"benefit":{"type":"並行","start_time":"12:30"},"ticket":{"prices":[]},"image_url":"https://pbs.twimg.com/media/Xflyer.jpg?name=large","images":[{"type":"flyer","label":"告知画像","url":"https://pbs.twimg.com/media/Xflyer.jpg?name=large"}],"source_refs":[{"type":"timetree_event","source_id":"ev1","url":"https://timetr.ee/p/cal/ev1"}]}]}`)
	store := &fakeStore{objects: map[string]fakeObject{"data/groups/dollykiss/schedule.json": {initial, 1}}}
	source := &fakeSource{events: map[string][]timetree.Event{"cal": {{
		ID:        "ev1",
		Title:     "公演A",
		StartAt:   "2026-11-29T09:00:00+09:00",
		ImageUrls: []string{"https://attachments.timetreeapp.com/dup-flyer.jpg", "https://attachments.timetreeapp.com/timetable.jpg"},
	}}}}
	fp := &stubImageFP{sigs: map[string]schedule.ImageSignature{
		"https://pbs.twimg.com/media/Xflyer.jpg?name=large":  {Hash: 0xad0ca323bfcb4f1f, AspectRatio: 0.748, Valid: true},
		"https://attachments.timetreeapp.com/dup-flyer.jpg":  {Hash: 0x2d0ca3a2bbcbd71f, AspectRatio: 0.748, Valid: true},
		"https://attachments.timetreeapp.com/timetable.jpg":  {Hash: 0xa5eaeaeaebcb5d1f, AspectRatio: 0.748, Valid: true},
	}}
	from := time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC)
	opts := Options{Source: source, Store: store, Bucket: "test", Targets: []Target{{"dollykiss", "cal"}}, From: from, Until: from.AddDate(0, 1, 0), ImageFingerprinter: fp}
	summary, err := Run(context.Background(), opts)
	if err != nil || store.writes != 1 || summary.Results[0].Changed == 0 {
		t.Fatalf("unexpected first run: %+v err=%v writes=%d", summary, err, store.writes)
	}
	var doc struct {
		Sessions []struct {
			Images []struct{ URL string `json:"url"` } `json:"images"`
		} `json:"sessions"`
	}
	if err := json.Unmarshal(store.objects["data/groups/dollykiss/schedule.json"].data, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Sessions[0].Images) != 2 {
		t.Fatalf("expected X flyer + TimeTree timetable (2 images), got %d: %+v", len(doc.Sessions[0].Images), doc.Sessions[0].Images)
	}
	if summary2, err := Run(context.Background(), opts); err != nil || store.writes != 1 || summary2.Results[0].Changed != 0 {
		t.Fatalf("expected idempotent second run without write, got %+v err=%v writes=%d", summary2, err, store.writes)
	}
}

func TestExtractIncludesTicketPricesFromDescription(t *testing.T) {
	event := timetree.Event{
		ID:          "ev-price",
		Title:       "対バン公演",
		StartAt:     "2026-11-29T10:00:00+09:00",
		Description: "🎪会場：新宿WALLY\n🎤出演：18:00〜18:20\n🎫料金：前方 ¥4,000 / 通常 ¥2,000 (+1D)",
	}
	opts, err := Extract("ribbon_clock", event)
	if err != nil {
		t.Fatal(err)
	}
	if len(opts.TicketPrices) != 2 {
		t.Fatalf("expected 2 ticket prices extracted from TimeTree description, got %+v", opts.TicketPrices)
	}
	if opts.TicketPrices[0].Tier != "前方" || opts.TicketPrices[0].Amount != 4000 || opts.TicketPrices[1].Tier != "通常" || opts.TicketPrices[1].Price != "¥2,000 (+1D)" {
		t.Errorf("unexpected extracted prices: %+v", opts.TicketPrices)
	}
}
