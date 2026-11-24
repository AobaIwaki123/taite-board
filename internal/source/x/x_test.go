package x

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNormalizeMediaURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "format and orig query",
			url:  "https://pbs.twimg.com/media/ABC123?format=jpg&name=orig",
			want: "https://pbs.twimg.com/media/ABC123?format=jpg&name=large",
		},
		{
			name: "jpg path and orig query",
			url:  "https://pbs.twimg.com/media/ABC123.jpg?name=orig",
			want: "https://pbs.twimg.com/media/ABC123.jpg?name=large",
		},
		{
			name: "non media URL",
			url:  "https://example.com/image.jpg?name=orig",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeMediaURL(tt.url); got != tt.want {
				t.Fatalf("normalizeMediaURL(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestExtractStatusID(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://x.com/Ribbon_Clock/status/2116817522318250115?s=46", "2116817522318250115"},
		{"https://twitter.com/user/status/12345", "12345"},
		{"https://example.com/not/a/tweet", ""},
	}
	for _, tt := range tests {
		if got := ExtractStatusID(tt.url); got != tt.want {
			t.Errorf("ExtractStatusID(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func TestLiveFetch1104Images(t *testing.T) {
	c := NewClient(15 * time.Second)
	p, err := c.FetchPost(context.Background(), "https://x.com/Ribbon_Clock/status/2116817522318250115")
	if err != nil {
		t.Fatalf("FetchPost error: %v", err)
	}
	if len(p.Images) != 2 {
		t.Fatalf("expected 2 images, got %d: %v", len(p.Images), p.Images)
	}
	t.Logf("Fetched %d images: %v", len(p.Images), p.Images)
}

func TestClient_FxTwitter_FullNoteAndTicketURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"code": 200,
			"tweet": {
				"text": "🎀『POLAROID - FREE -』🎀\n🗓️日付：12月2日（水）\n🎪会場：原宿RUIDO\n🎤出演：18:25〜18:45\n📸特典会：18:50〜19:50（並行A）\nhttp://ticketdive.com/event/261202_PF",
				"author": {"name": "リボンクロック", "url": "https://x.com/Ribbon_Clock"},
				"media": {
					"photos": [
						{"url": "https://pbs.twimg.com/media/HXuoS23agAArPHA.jpg?name=orig"}
					]
				}
			}
		}`))
	}))
	defer srv.Close()

	c := &Client{
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
		FxBaseURL:  srv.URL,
	}
	post, err := c.FetchPost(context.Background(), "https://x.com/Ribbon_Clock/status/2124480514828415477")
	if err != nil {
		t.Fatalf("FetchPost failed: %v", err)
	}
	if !strings.Contains(post.Text, "http://ticketdive.com/event/261202_PF") {
		t.Errorf("expected full expanded ticket URL in text, got: %s", post.Text)
	}
	if len(post.Images) != 1 || !strings.HasSuffix(post.Images[0], "name=large") {
		t.Errorf("expected normalized large image URL, got: %v", post.Images)
	}
}
