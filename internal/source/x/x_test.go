package x

import (
	"context"
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
