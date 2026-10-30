package x

import "testing"

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
			want: "https://example.com/image.jpg?name=orig",
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
