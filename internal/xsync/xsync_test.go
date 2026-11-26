package xsync

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/AobaIwaki123/taite-board/internal/candidate"
	"github.com/AobaIwaki123/taite-board/internal/parser"
	"github.com/AobaIwaki123/taite-board/internal/raw"
	"github.com/AobaIwaki123/taite-board/internal/resolve"
	"github.com/AobaIwaki123/taite-board/internal/schedule"
)

type fakeFetcher struct {
	posts map[string]*raw.Post
	calls []string
}

func (f *fakeFetcher) FetchPost(_ context.Context, postURL string) (*raw.Post, error) {
	f.calls = append(f.calls, postURL)
	if p, ok := f.posts[postURL]; ok {
		return p, nil
	}
	return nil, errors.New("not found")
}

type fakeAI struct {
	calls int
}

func (a *fakeAI) Parse(_ context.Context, _ *raw.Post) (*parser.ParseResponse, error) {
	a.calls++
	return &parser.ParseResponse{Candidate: candidate.EventCandidate{LiveStartRaw: "19:00", LiveEndRaw: "19:20"}}, nil
}

type fakeImageFP struct {
	sigs map[string]schedule.ImageSignature
}

func (f *fakeImageFP) Fingerprint(_ context.Context, url string) (schedule.ImageSignature, error) {
	if s, ok := f.sigs[url]; ok {
		return s, nil
	}
	return schedule.ImageSignature{}, errors.New("miss")
}

func TestReSyncScheduleBytes_BackfillsSecondPhotoAndAppliesAICostGuards(t *testing.T) {
	initial := []byte(`{
  "schema_version": 1,
  "timezone": "Asia/Tokyo",
  "target_group": {"name": "リボンクロック"},
  "sessions": [
    {
      "id": "20261125-1",
      "title": "POLAROID - FREE -",
      "date": "2026-11-25",
      "day_of_week": "水",
      "venue": {"canonical_id": "venue:harajuku-ruido", "name": "原宿RUIDO", "map_query": "原宿RUIDO", "map_url": "https://maps.google.com/?q=原宿RUIDO"},
      "live": {"start_time": "18:45", "end_time": "19:05"},
      "benefit": {"type": "並行", "start_time": "19:10", "end_time": "20:10", "booth": "並行B"},
      "ticket": {"platform": "TicketDive", "url": "https://ticketdive.com/event/261125_PF", "prices": []},
      "image_url": "https://pbs.twimg.com/media/flyer1.jpg?name=large",
      "images": [{"type": "flyer", "label": "告知画像", "url": "https://pbs.twimg.com/media/flyer1.jpg?name=large"}],
      "source_tweet_url": "https://x.com/Ribbon_Clock/status/2125167544894198124",
      "source_refs": [{"type": "x_post", "source_id": "2125167544894198124", "url": "https://x.com/Ribbon_Clock/status/2125167544894198124"}]
    },
    {
      "id": "20261129-1",
      "title": "アイコレ",
      "date": "2026-11-29",
      "day_of_week": "日",
      "venue": {"canonical_id": "venue:hall", "name": "Hall", "map_query": "Hall", "map_url": "https://maps.google.com/?q=Hall"},
      "live": {"start_time": "19:05", "end_time": "19:25"},
      "benefit": {"type": "並行", "start_time": "19:30", "end_time": "20:30"},
      "ticket": {"platform": "", "url": null, "prices": []},
      "images": [
        {"type": "flyer", "url": "https://pbs.twimg.com/media/a.jpg"},
        {"type": "flyer", "url": "https://pbs.twimg.com/media/b.jpg"}
      ],
      "source_tweet_url": "https://x.com/Ribbon_Clock/status/999999999999999999"
    }
  ]
}`)

	fetcher := &fakeFetcher{posts: map[string]*raw.Post{
		"https://x.com/Ribbon_Clock/status/2125167544894198124": {
			SourceID: "2125167544894198124",
			URL:      "https://x.com/Ribbon_Clock/status/2125167544894198124",
			Text:     "明日のライブ 原宿RUIDO",
			Images: []string{
				"https://pbs.twimg.com/media/dup-flyer.jpg?name=large",
				"https://pbs.twimg.com/media/timetable.jpg?name=large",
			},
		},
	}}
	fp := &fakeImageFP{sigs: map[string]schedule.ImageSignature{
		"https://pbs.twimg.com/media/flyer1.jpg?name=large":    {Hash: 0x7d712f2b33372759, AspectRatio: 0.707, Valid: true},
		"https://pbs.twimg.com/media/dup-flyer.jpg?name=large": {Hash: 0x7d712f2b33372759, AspectRatio: 0.707, Valid: true},
		"https://pbs.twimg.com/media/timetable.jpg?name=large": {Hash: 0x1111222233334444, AspectRatio: 0.707, Valid: true},
	}}
	// Adjust distinct Grid so MAE16 clearly exceeds 7.0
	var distinctGrid [16][16]float64
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			distinctGrid[y][x] = 200
		}
	}
	sigTT := fp.sigs["https://pbs.twimg.com/media/timetable.jpg?name=large"]
	sigTT.Grid = distinctGrid
	fp.sigs["https://pbs.twimg.com/media/timetable.jpg?name=large"] = sigTT

	ai := &fakeAI{}
	res, err := resolve.NewResolver("")
	if err != nil {
		t.Fatal(err)
	}

	outBytes, rep, err := ReSyncScheduleBytes(context.Background(), "ribonclock", initial, Options{
		Fetcher:            fetcher,
		Resolver:           res,
		ImageFingerprinter: fp,
		AIParser:           ai,
		WithAI:             true,
		MinImagesToSkip:    2,
		IncludePast:        true,
	})
	if err != nil {
		t.Fatalf("ReSyncScheduleBytes failed: %v", err)
	}
	if !rep.Changed || rep.Updated != 1 || rep.Skipped != 1 {
		t.Fatalf("unexpected report: %+v", rep)
	}
	if ai.calls != 0 {
		t.Fatalf("expected AI cost guard to block calls when times/2+ images already present, got %d", ai.calls)
	}
	if len(fetcher.calls) != 1 {
		t.Fatalf("expected 20261129-1 (>=2 images) skipped without HTTP fetch, calls=%v", fetcher.calls)
	}

	var doc struct {
		Sessions []sessionView `json:"sessions"`
	}
	if err := json.Unmarshal(outBytes, &doc); err != nil {
		t.Fatal(err)
	}
	if got := doc.Sessions[0].Images; len(got) != 2 || got[1].URL != "https://pbs.twimg.com/media/timetable.jpg?name=large" {
		t.Fatalf("expected 11.25 timetable added and dup flyer removed, got %+v", got)
	}
}
