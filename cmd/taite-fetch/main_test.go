package main

import (
	"strings"
	"testing"
)

func TestCleanHTMLText(t *testing.T) {
	rawHTML := `<blockquote class="twitter-tweet"><p lang="ja" dir="ltr">🎀『イベント名』🎀<br><br>🗓️日付：11/3（火祝）<br>🎪会場：ホール<br>🎤出演：11:55〜12:15<br><br><a href="https://t.co/xxx">pic.twitter.com/xxx</a></p>&mdash; 作者 (@Author) <a href="...">date</a></blockquote>`
	expectedContains := "🎀『イベント名』🎀\n\n🗓️日付：11/3（火祝）\n🎪会場：ホール\n🎤出演：11:55〜12:15"

	cleaned := cleanHTMLText(rawHTML)
	if !strings.Contains(cleaned, expectedContains) {
		t.Errorf("cleanHTMLText mismatch.\nExpected contains:\n%s\nGot:\n%s", expectedContains, cleaned)
	}
}

func TestPBSImageRegex(t *testing.T) {
	sampleHTML := `<meta property="og:image" content="https://pbs.twimg.com/media/HVxASJ0a0AEORf5?format=webp&amp;name=large">`
	matches := pbsImageRegex.FindAllString(sampleHTML, -1)
	if len(matches) != 1 {
		t.Fatalf("Expected 1 match, got %d", len(matches))
	}
	expected := "https://pbs.twimg.com/media/HVxASJ0a0AEORf5"
	if matches[0] != expected {
		t.Errorf("Expected %s, got %s", expected, matches[0])
	}
}
