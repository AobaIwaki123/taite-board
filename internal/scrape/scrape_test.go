package scrape

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AobaIwaki123/taite-board/internal/filter"
	"github.com/AobaIwaki123/taite-board/internal/parser"
	"github.com/AobaIwaki123/taite-board/internal/raw"
)

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type stubPostFetcher struct {
	posts map[string]*raw.Post
	errs  map[string]error
}

func (f *stubPostFetcher) FetchPost(_ context.Context, postURL string) (*raw.Post, error) {
	if err, ok := f.errs[postURL]; ok {
		return nil, err
	}
	if p, ok := f.posts[postURL]; ok {
		clone := *p
		return &clone, nil
	}
	return nil, errors.New("not found")
}

func TestURLListSource_RespectsSinceIDAndIsolatesErrors(t *testing.T) {
	src := &URLListSource{
		URLs: []string{
			"https://x.com/Ribbon_Clock/status/100",
			"https://x.com/Ribbon_Clock/status/200",
			"https://x.com/Ribbon_Clock/status/300",
		},
		Fetcher: &stubPostFetcher{
			posts: map[string]*raw.Post{
				"https://x.com/Ribbon_Clock/status/200": {
					Source:   "x",
					SourceID: "200",
					URL:      "https://x.com/Ribbon_Clock/status/200",
					Text:     "ライブ出演告知 11/28 渋谷RING 出演 18:00-18:25",
					Images:   []string{"https://pbs.twimg.com/media/abc.jpg?name=large"},
				},
			},
			errs: map[string]error{
				"https://x.com/Ribbon_Clock/status/300": errors.New("upstream 503"),
			},
		},
		Workers: 2,
	}

	res, err := src.FetchTimeline(context.Background(), TimelineRequest{SinceID: "100"})
	if err != nil {
		t.Fatalf("FetchTimeline failed: %v", err)
	}
	if len(res.Posts) != 1 || res.Posts[0].SourceID != "200" {
		t.Fatalf("expected only post 200 (100 skipped, 300 errored), got %+v", res.Posts)
	}
	if len(res.FetchErrors) != 1 || res.FetchErrors["https://x.com/Ribbon_Clock/status/300"] == "" {
		t.Fatalf("expected isolated fetch error for status 300, got %+v", res.FetchErrors)
	}
}

func TestXAPISource_ParsesNoteTweetAndMedia(t *testing.T) {
	var seenAuth, seenSinceID string
	src := NewXAPISource("test-bearer", 5*time.Second)
	src.HTTPClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			seenAuth = r.Header.Get("Authorization")
			seenSinceID = r.URL.Query().Get("since_id")
			payload := `{
				"data": [{
					"id": "2118236876075053443",
					"text": "short truncated https://t.co/abc...",
					"note_tweet": {
						"text": "11/28(土) 渋谷RING『テストライブ』出演 18:30-18:55\nhttps://t.co/ticket1",
						"entities": {
							"urls": [{"url": "https://t.co/ticket1", "expanded_url": "https://t.livepocket.jp/e/test1"}]
						}
					},
					"attachments": {"media_keys": ["3_999"]}
				}],
				"includes": {
					"media": [{"media_key": "3_999", "type": "photo", "url": "https://pbs.twimg.com/media/HVi3dC9bsAAinY6.jpg"}]
				},
				"meta": {"newest_id": "2118236876075053443"}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(payload)),
			}, nil
		}),
	}

	res, err := src.FetchTimeline(context.Background(), TimelineRequest{
		Account: "Ribbon_Clock",
		UserID:  "12345",
		SinceID: "2000000000000000000",
	})
	if err != nil {
		t.Fatalf("FetchTimeline failed: %v", err)
	}
	if seenAuth != "Bearer test-bearer" {
		t.Fatalf("expected Bearer auth header, got %q", seenAuth)
	}
	if seenSinceID != "2000000000000000000" {
		t.Fatalf("expected since_id forwarded, got %q", seenSinceID)
	}
	if res.NewestID != "2118236876075053443" || len(res.Posts) != 1 {
		t.Fatalf("unexpected XAPI result: %+v", res)
	}
	got := res.Posts[0]
	if !strings.Contains(got.Text, "https://t.livepocket.jp/e/test1") {
		t.Fatalf("expected expanded URL in note_tweet text, got %q", got.Text)
	}
	if len(got.Images) != 1 || !strings.Contains(got.Images[0], "name=large") {
		t.Fatalf("expected photo media with name=large, got %+v", got.Images)
	}
}

func TestRunner_StagesFilteredPostsAndDeduplicatesRerun(t *testing.T) {
	now := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)
	dir := t.TempDir()
	statePath := filepath.Join(dir, "scrape_state.json")
	fixturePath := filepath.Join(dir, "fixture.json")

	fixturePosts := []*raw.Post{
		{
			Source:    "x",
			SourceID:  "101",
			URL:       "https://x.com/Ribbon_Clock/status/101",
			Text:      "おはよう！今日も一日がんばろう",
			Images:    []string{"https://pbs.twimg.com/media/selfie.jpg"},
			FetchedAt: now.Format(time.RFC3339),
		},
		{
			Source:    "x",
			SourceID:  "102",
			URL:       "https://x.com/Ribbon_Clock/status/102",
			Text:      "ライブ出演告知\n11/28(土) 渋谷RING\n『アイドルライブ SP』\n出演 18:30-18:55\n入場特典: 2S写メ券",
			Images:    []string{"https://pbs.twimg.com/media/flyer1.jpg?name=large"},
			FetchedAt: now.Format(time.RFC3339),
		},
	}
	rawFixture, _ := json.Marshal(fixturePosts)
	if err := os.WriteFile(fixturePath, rawFixture, 0644); err != nil {
		t.Fatal(err)
	}

	runner, err := NewRunner("")
	if err != nil {
		t.Fatal(err)
	}
	stateStore := &FileStateStore{Path: statePath}

	report, err := runner.Run(context.Background(), RunOptions{
		Group:   "ribonclock",
		Source:  &FixtureSource{Path: fixturePath},
		State:   stateStore,
		NowFunc: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if report.FetchedCount != 2 || report.PassedCount != 1 || report.SkippedCount != 1 || report.StagedCount != 1 {
		t.Fatalf("unexpected report counts: %+v", report)
	}
	if report.SkippedByStage[string(filter.StageExclude)] != 1 {
		t.Fatalf("expected post 101 skipped by exclude_keyword, got %+v", report.SkippedByStage)
	}
	if report.NewestSinceID != "102" {
		t.Fatalf("expected NewestSinceID=102, got %q", report.NewestSinceID)
	}

	doc, err := stateStore.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.PendingQueue) != 1 || doc.PendingQueue[0].ID != "ribonclock:102" {
		t.Fatalf("expected ribonclock:102 staged in PendingQueue, got %+v", doc.PendingQueue)
	}

	// Second run with same fixture should fetch 0 posts because checkpoint cursor moved to 102
	report2, err := runner.Run(context.Background(), RunOptions{
		Group:   "ribonclock",
		Source:  &FixtureSource{Path: fixturePath},
		State:   stateStore,
		NowFunc: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("second Run failed: %v", err)
	}
	if report2.FetchedCount != 0 {
		t.Fatalf("expected 0 fetched on second run after since_id=102, got %d", report2.FetchedCount)
	}
}

func TestRunner_AutoUpsertAndAmbiguousFallback(t *testing.T) {
	now := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)
	dir := t.TempDir()
	schedPath := filepath.Join(dir, "schedule.json")
	statePath := filepath.Join(dir, "scrape_state.json")

	initialSched := `{
		"schema_version": 1,
		"timezone": "Asia/Tokyo",
		"generated_at": "2026-11-25T00:00:00+09:00",
		"target_group": {
			"name": "リボンクロック",
			"canonical_id": "group:ribbon-clock"
		},
		"sessions": [
			{
				"id": "20261128-1",
				"title": "テストライブ 1部",
				"session_name": "1部",
				"date": "2026-11-28",
				"day_of_week": "土",
				"venue": {"canonical_id": "venue:shibuya-ring", "name": "渋谷RING", "map_query": "渋谷RING", "map_url": "https://maps.google.com/?q=渋谷RING"},
				"live": {"start_time": "12:00", "end_time": "12:25", "stage": null},
				"benefit": {"type": "並行", "start_time": "12:30", "end_time": "13:30", "booth": ""},
				"ticket": {"platform": "", "url": null, "prices": []},
				"image_url": "",
				"source_tweet_url": "https://x.com/Ribbon_Clock/status/1",
				"images": [],
				"source_refs": []
			},
			{
				"id": "20261128-2",
				"title": "テストライブ 2部",
				"session_name": "2部",
				"date": "2026-11-28",
				"day_of_week": "土",
				"venue": {"canonical_id": "venue:shibuya-ring", "name": "渋谷RING", "map_query": "渋谷RING", "map_url": "https://maps.google.com/?q=渋谷RING"},
				"live": {"start_time": "18:00", "end_time": "18:25", "stage": null},
				"benefit": {"type": "並行", "start_time": "18:30", "end_time": "19:30", "booth": ""},
				"ticket": {"platform": "", "url": null, "prices": []},
				"image_url": "",
				"source_tweet_url": "https://x.com/Ribbon_Clock/status/2",
				"images": [],
				"source_refs": []
			}
		]
	}`
	if err := os.WriteFile(schedPath, []byte(initialSched), 0644); err != nil {
		t.Fatal(err)
	}

	runner, _ := NewRunner("")
	stateStore := &FileStateStore{Path: statePath}
	schedStore := &FileScheduleAccessor{Path: schedPath}

	src := &URLListSource{
		URLs: []string{
			// Unambiguous new date -> auto-upserted
			"https://x.com/Ribbon_Clock/status/501",
			// Same ambiguous date 11/28 with 1部/2部 and no distinguishing time -> falls back to ambiguous staged item
			"https://x.com/Ribbon_Clock/status/502",
		},
		Fetcher: &stubPostFetcher{
			posts: map[string]*raw.Post{
				"https://x.com/Ribbon_Clock/status/501": {
					Source:    "x",
					SourceID:  "501",
					URL:       "https://x.com/Ribbon_Clock/status/501",
					Text:      "ライブ出演告知 11/30(月) 渋谷RING『新ライブ』出演 19:00-19:25",
					Images:    []string{"https://pbs.twimg.com/media/new1.jpg?name=large"},
					FetchedAt: now.Format(time.RFC3339),
				},
				"https://x.com/Ribbon_Clock/status/502": {
					Source:    "x",
					SourceID:  "502",
					URL:       "https://x.com/Ribbon_Clock/status/502",
					Text:      "11/28(土) 渋谷RING ライブ入場特典：2S写メ券\n1曲目『えすえぬ・えーす』撮影可能",
					Images:    []string{"https://pbs.twimg.com/media/perk1.jpg?name=large"},
					FetchedAt: now.Format(time.RFC3339),
				},
			},
		},
	}

	report, err := runner.Run(context.Background(), RunOptions{
		Group:      "ribonclock",
		Source:     src,
		State:      stateStore,
		Schedule:   schedStore,
		AutoUpsert: true,
		NowFunc:    func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if report.AutoUpsertedCount != 1 || report.AmbiguousCount != 1 {
		t.Fatalf("expected 1 auto_upserted and 1 ambiguous, got %+v", report)
	}

	doc, _ := stateStore.Load(context.Background())
	if len(doc.PendingQueue) != 1 || doc.PendingQueue[0].Status != StatusAmbiguous {
		t.Fatalf("expected ambiguous post 502 queued as StatusAmbiguous, got %+v", doc.PendingQueue)
	}
}

type stubAIParser struct{}

func (stubAIParser) Parse(_ context.Context, _ *raw.Post) (*parser.ParseResponse, error) {
	return nil, nil
}
