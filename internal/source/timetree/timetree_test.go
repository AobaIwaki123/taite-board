package timetree

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestExtractCalendarAndEventID(t *testing.T) {
	cases := []struct {
		url      string
		wantCal  string
		wantEv   string
		wantSucc bool
	}{
		{
			url:      "https://timetreeapp.com/public_calendars/ribbon_clock/events/3093691568935820727",
			wantCal:  "ribbon_clock",
			wantEv:   "3093691568935820727",
			wantSucc: true,
		},
		{
			url:      "https://timetreeapp.com/public_calendars/lui_17707/events/123456?locale=ja",
			wantCal:  "lui_17707",
			wantEv:   "123456",
			wantSucc: true,
		},
		{
			url:      "https://timetr.ee/p/ribbon_clock/3093691568935820727",
			wantCal:  "ribbon_clock",
			wantEv:   "3093691568935820727",
			wantSucc: true,
		},
		{
			url:      "https://x.com/Ribbon_Clock/status/12345",
			wantSucc: false,
		},
	}

	for _, tc := range cases {
		cal, ev, ok := ExtractCalendarAndEventID(tc.url)
		if ok != tc.wantSucc || cal != tc.wantCal || ev != tc.wantEv {
			t.Errorf("ExtractCalendarAndEventID(%q) = (%q, %q, %v), want (%q, %q, %v)",
				tc.url, cal, ev, ok, tc.wantCal, tc.wantEv, tc.wantSucc)
		}
	}
}

func TestClient_FetchPost_LumitreeAndFallback(t *testing.T) {
	lumiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/calendars/ribbon_clock/events" && r.URL.RawQuery == "" {
			_, _ = w.Write([]byte(`{
				"calendar": {"id": "1005370007", "aliasCode": "ribbon_clock", "title": "リボンクロック 公式LIVE予定"},
				"events": [{
					"id": "3093691568935820727",
					"title": "ライブ@新宿",
					"description": "🎀『アイコレ』🎀\n🗓️日付：11/28(土)\n🎪会場：LUMINE 0\n🎤出演：10:45〜11:00\n📸特典会：11:05〜12:05（ロビーB）\nhttps://tiget.net/events/537396",
					"startAt": "2026-11-28T09:00:00+09:00",
					"url": "https://timetr.ee/p/ribbon_clock/3093691568935820727",
					"imageUrls": ["https://attachments.timetreeapp.com/a.jpg", "https://attachments.timetreeapp.com/b.jpg"]
				}]
			}`))
			return
		}
		_, _ = w.Write([]byte(`{"calendar":{"aliasCode":"ribbon_clock"},"events":[]}`))
	}))
	defer lumiSrv.Close()

	ttSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html><html><head>
			<meta property="og:title" content="フォールバック公演">
			<meta property="og:description" content="12/05(土) 渋谷RING 出演：19:00〜19:20">
			<meta property="og:image" content="https://attachments.timetreeapp.com/fallback.jpg">
		</head></html>`))
	}))
	defer ttSrv.Close()

	client := &Client{
		HTTPClient:      &http.Client{Timeout: 5 * time.Second},
		LumitreeBaseURL: lumiSrv.URL,
		TimeTreeBaseURL: ttSrv.URL,
	}

	// 1. Lumitree hit returns full multi-line description and all images
	post, err := client.FetchPost(context.Background(), "https://timetreeapp.com/public_calendars/ribbon_clock/events/3093691568935820727")
	if err != nil {
		t.Fatalf("FetchPost lumitree failed: %v", err)
	}
	if post.Source != "timetree" || post.SourceID != "3093691568935820727" {
		t.Errorf("unexpected source/id: %+v", post)
	}
	if post.Author != "リボンクロック 公式LIVE予定" {
		t.Errorf("unexpected author: %s", post.Author)
	}
	if !strings.Contains(post.Text, "tiget.net/events/537396") {
		t.Errorf("expected full description with ticket url, got: %s", post.Text)
	}
	if len(post.Images) != 2 {
		t.Errorf("expected 2 images, got %d", len(post.Images))
	}

	// 2. Event absent in lumitree falls back to TimeTree OGP HTML
	fallbackPost, err := client.FetchPost(context.Background(), "https://timetreeapp.com/public_calendars/ribbon_clock/events/999999999")
	if err != nil {
		t.Fatalf("FetchPost fallback failed: %v", err)
	}
	if fallbackPost.Source != "timetree" || !strings.Contains(fallbackPost.Text, "渋谷RING") {
		t.Errorf("unexpected fallback post: %+v", fallbackPost)
	}
	if len(fallbackPost.Images) != 1 || fallbackPost.Images[0] != "https://attachments.timetreeapp.com/fallback.jpg" {
		t.Errorf("expected og:image in fallback, got %v", fallbackPost.Images)
	}
}
