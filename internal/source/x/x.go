package x

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/AobaIwaki123/taite-board/internal/raw"
)

var (
	tagRegex      = regexp.MustCompile(`<[^>]*>`)
	pbsImageRegex = regexp.MustCompile(`https://pbs\.twimg\.com/media/[a-zA-Z0-9_-]+`)
	statusIDRegex = regexp.MustCompile(`/status/(\d+)`)
)

type oembedResponse struct {
	URL        string `json:"url"`
	AuthorName string `json:"author_name"`
	AuthorURL  string `json:"author_url"`
	HTML       string `json:"html"`
}

// Client implements PostFetcher for X (formerly Twitter) posts.
type Client struct {
	HTTPClient *http.Client
}

// NewClient returns a new X source adapter client.
func NewClient(timeout time.Duration) *Client {
	return &Client{
		HTTPClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// ExtractStatusID returns the numeric status ID from an X URL.
func ExtractStatusID(postURL string) string {
	m := statusIDRegex.FindStringSubmatch(postURL)
	if len(m) > 1 {
		return m[1]
	}
	return ""
}

// FetchPost retrieves post text, metadata, and media images from an X post URL.
func (c *Client) FetchPost(ctx context.Context, postURL string) (*raw.Post, error) {
	statusID := ExtractStatusID(postURL)
	if statusID == "" {
		return nil, fmt.Errorf("invalid x post url: no status id found in %s", postURL)
	}

	// 1. Fetch via official oEmbed API
	oembedURL := fmt.Sprintf("https://publish.twitter.com/oembed?url=%s&omit_script=true", url.QueryEscape(postURL))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, oembedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating oembed request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; TaiteBoardBot/1.0)")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing oembed request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oembed returned status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading oembed body: %w", err)
	}

	var oembed oembedResponse
	if err := json.Unmarshal(body, &oembed); err != nil {
		return nil, fmt.Errorf("parsing oembed json: %w", err)
	}

	// Clean HTML
	cleanHTML := strings.ReplaceAll(oembed.HTML, "<br>", "\n")
	cleanHTML = strings.ReplaceAll(cleanHTML, "<br/>", "\n")
	cleanHTML = strings.ReplaceAll(cleanHTML, "<br />", "\n")
	cleanHTML = strings.ReplaceAll(cleanHTML, "</p>", "\n\n")

	cleanText := tagRegex.ReplaceAllString(cleanHTML, "")
	cleanText = html.UnescapeString(cleanText)
	cleanText = strings.TrimSpace(cleanText)

	// Remove trailing author line
	lines := strings.Split(cleanText, "\n")
	var filtered []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "— ") || strings.HasPrefix(trimmed, "- ") {
			continue
		}
		filtered = append(filtered, line)
	}
	finalText := strings.TrimSpace(strings.Join(filtered, "\n"))

	// 2. Extract media images via Syndication API
	images := c.fetchSyndicationImages(ctx, statusID)

	return &raw.Post{
		SchemaVersion: 1,
		Source:        "x",
		SourceID:      statusID,
		URL:           postURL,
		Author:        oembed.AuthorName,
		AuthorURL:     oembed.AuthorURL,
		Text:          finalText,
		Images:        images,
		FetchedAt:     time.Now().Format(time.RFC3339),
	}, nil
}

func (c *Client) fetchSyndicationImages(ctx context.Context, statusID string) []string {
	syndicationURL := fmt.Sprintf("https://cdn.syndication.twimg.com/tweet-result?id=%s&lang=ja", statusID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, syndicationURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 16_6 like Mac OS X) AppleWebKit/605.1.15")

	resp, err := c.HTTPClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			resp.Body.Close()
		}
		return nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	var result struct {
		MediaDetails []struct {
			Type     string `json:"type"`
			MediaURL string `json:"media_url_https"`
		} `json:"mediaDetails"`
		Photos []struct {
			URL string `json:"url"`
		} `json:"photos"`
	}

	var images []string
	if err := json.Unmarshal(body, &result); err == nil {
		for _, m := range result.MediaDetails {
			if m.MediaURL != "" {
				images = append(images, m.MediaURL)
			}
		}
		if len(images) == 0 {
			for _, p := range result.Photos {
				if p.URL != "" {
					images = append(images, p.URL)
				}
			}
		}
	}

	// Fallback regex scan
	if len(images) == 0 {
		matches := pbsImageRegex.FindAllString(string(body), -1)
		seen := make(map[string]bool)
		for _, m := range matches {
			if !seen[m] {
				seen[m] = true
				images = append(images, m+".jpg")
			}
		}
	}

	return images
}
