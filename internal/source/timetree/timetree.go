package timetree

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/AobaIwaki123/taite-board/internal/raw"
)

const defaultLumitreeBaseURL = "https://lumitree.aooba.net"

var (
	publicCalEventRegex = regexp.MustCompile(`^/public_calendars/([^/]+)/events/([^/?#]+)`)
	shortEventRegex     = regexp.MustCompile(`^/p/([^/]+)/([^/?#]+)`)
	metaTagRegex        = regexp.MustCompile(`(?is)<meta\s+[^>]*?(?:property|name)=["']([^"']+)["'][^>]*?content=["']([^"']*)["'][^>]*?>`)
	metaTagAltRegex     = regexp.MustCompile(`(?is)<meta\s+[^>]*?content=["']([^"']*)["'][^>]*?(?:property|name)=["']([^"']+)["'][^>]*?>`)
)

// Client implements PostFetcher for TimeTree public calendar events.
type Client struct {
	HTTPClient      *http.Client
	LumitreeBaseURL string
	TimeTreeBaseURL string
}

// NewClient returns a new TimeTree source adapter client.
func NewClient(timeout time.Duration) *Client {
	base := strings.TrimSpace(os.Getenv("LUMITREE_BASE_URL"))
	if base == "" {
		base = defaultLumitreeBaseURL
	}
	return &Client{
		HTTPClient: &http.Client{
			Timeout: timeout,
		},
		LumitreeBaseURL: strings.TrimRight(base, "/"),
		TimeTreeBaseURL: "https://timetreeapp.com",
	}
}

// ExtractCalendarAndEventID extracts (calendarID, eventID) from a TimeTree public event URL.
func ExtractCalendarAndEventID(rawURL string) (string, string, bool) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", "", false
	}
	host := strings.ToLower(u.Hostname())
	switch host {
	case "timetreeapp.com", "www.timetreeapp.com":
		m := publicCalEventRegex.FindStringSubmatch(u.Path)
		if len(m) == 3 {
			return m[1], m[2], true
		}
	case "timetr.ee", "www.timetr.ee":
		m := shortEventRegex.FindStringSubmatch(u.Path)
		if len(m) == 3 {
			return m[1], m[2], true
		}
	}
	return "", "", false
}

type lumitreeResponse struct {
	Calendar struct {
		ID        string `json:"id"`
		AliasCode string `json:"aliasCode"`
		Title     string `json:"title"`
	} `json:"calendar"`
	Events []lumitreeEvent `json:"events"`
}

type lumitreeEvent struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	StartAt     string   `json:"startAt"`
	EndAt       string   `json:"endAt"`
	AllDay      bool     `json:"allDay"`
	Location    string   `json:"location"`
	URL         string   `json:"url"`
	ImageUrls   []string `json:"imageUrls"`
}

// FetchPost retrieves an event from TimeTree (via lumitree API first, then HTML OGP fallback).
func (c *Client) FetchPost(ctx context.Context, eventURL string) (*raw.Post, error) {
	calID, eventID, ok := ExtractCalendarAndEventID(eventURL)
	if !ok {
		return nil, fmt.Errorf("invalid timetree event url: %s", eventURL)
	}

	if post, err := c.fetchViaLumitree(ctx, calID, eventID, eventURL); err == nil && post != nil {
		return post, nil
	}

	return c.fetchViaPublicHTML(ctx, calID, eventID, eventURL)
}

func (c *Client) fetchViaLumitree(ctx context.Context, calID, eventID, origURL string) (*raw.Post, error) {
	base := strings.TrimRight(c.LumitreeBaseURL, "/")
	if base == "" {
		base = defaultLumitreeBaseURL
	}

	jst := time.FixedZone("Asia/Tokyo", 9*60*60)
	now := time.Now().In(jst)
	endpoints := []string{
		fmt.Sprintf("%s/api/v1/calendars/%s/events", base, url.PathEscape(calID)),
	}
	for offset := 0; offset <= 2; offset++ {
		target := now.AddDate(0, offset, 0)
		endpoints = append(endpoints, fmt.Sprintf("%s/api/v1/calendars/%s/events?year=%d&month=%d",
			base, url.PathEscape(calID), target.Year(), int(target.Month())))
	}

	for _, ep := range endpoints {
		respData, err := c.fetchLumitreeEndpoint(ctx, ep)
		if err != nil {
			continue
		}
		for _, ev := range respData.Events {
			if ev.ID == eventID {
				author := strings.TrimSpace(respData.Calendar.Title)
				if author == "" {
					author = calID
				}
				return buildRawPostFromEvent(ev, author, origURL), nil
			}
		}
	}

	return nil, fmt.Errorf("event %s not found in lumitree calendar %s", eventID, calID)
}

func (c *Client) fetchLumitreeEndpoint(ctx context.Context, endpoint string) (*lumitreeResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; TaiteBoardBot/1.0)")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("lumitree returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return nil, err
	}

	var parsed lumitreeResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	return &parsed, nil
}

func buildRawPostFromEvent(ev lumitreeEvent, author, origURL string) *raw.Post {
	var textParts []string
	desc := strings.TrimSpace(ev.Description)
	if desc != "" {
		textParts = append(textParts, desc)
	} else if ev.Title != "" {
		textParts = append(textParts, ev.Title)
	}

	// Ensure YYYY-MM-DD and location/venue hints are present if missing from description
	if len(ev.StartAt) >= 10 && !strings.Contains(desc, ev.StartAt[:10]) {
		dateHint := ev.StartAt[:10]
		if !strings.Contains(desc, strings.ReplaceAll(dateHint[5:], "-", "/")) &&
			!strings.Contains(desc, strings.TrimPrefix(strings.ReplaceAll(dateHint[5:], "-", "/"), "0")) {
			textParts = append(textParts, "日付: "+dateHint)
		}
	}
	if strings.TrimSpace(ev.Location) != "" && !strings.Contains(desc, ev.Location) {
		textParts = append(textParts, "会場: "+strings.TrimSpace(ev.Location))
	}

	images := make([]string, 0, len(ev.ImageUrls))
	for _, img := range ev.ImageUrls {
		if u := strings.TrimSpace(img); u != "" {
			images = append(images, u)
		}
	}

	canonicalURL := strings.TrimSpace(origURL)
	if canonicalURL == "" {
		canonicalURL = ev.URL
	}

	return &raw.Post{
		SchemaVersion: 1,
		Source:        "timetree",
		SourceID:      ev.ID,
		URL:           canonicalURL,
		Author:        author,
		Text:          strings.Join(textParts, "\n"),
		Images:        images,
		FetchedAt:     time.Now().UTC().Format(time.RFC3339),
	}
}

func (c *Client) fetchViaPublicHTML(ctx context.Context, calID, eventID, origURL string) (*raw.Post, error) {
	base := strings.TrimRight(c.TimeTreeBaseURL, "/")
	if base == "" {
		base = "https://timetreeapp.com"
	}
	pageURL := fmt.Sprintf("%s/public_calendars/%s/events/%s?locale=ja",
		base, url.PathEscape(calID), url.PathEscape(eventID))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating timetree html request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; TaiteBoardBot/1.0)")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching timetree html: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("timetree page returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return nil, fmt.Errorf("reading timetree html: %w", err)
	}

	metas := extractMetaTags(string(body))
	title := strings.TrimSpace(metas["og:title"])
	desc := strings.TrimSpace(metas["og:description"])
	if desc == "" {
		desc = strings.TrimSpace(metas["description"])
	}
	if title == "" && desc == "" {
		return nil, fmt.Errorf("timetree page contained no event metadata for %s", eventID)
	}

	text := desc
	if text == "" {
		text = title
	}

	var images []string
	if img := strings.TrimSpace(metas["og:image"]); img != "" {
		images = append(images, img)
	}

	canonicalURL := strings.TrimSpace(origURL)
	if canonicalURL == "" {
		canonicalURL = pageURL
	}

	return &raw.Post{
		SchemaVersion: 1,
		Source:        "timetree",
		SourceID:      eventID,
		URL:           canonicalURL,
		Author:        calID,
		Text:          text,
		Images:        images,
		FetchedAt:     time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func extractMetaTags(rawHTML string) map[string]string {
	out := make(map[string]string)
	for _, m := range metaTagRegex.FindAllStringSubmatch(rawHTML, -1) {
		if len(m) == 3 {
			key := strings.ToLower(strings.TrimSpace(m[1]))
			if _, exists := out[key]; !exists {
				out[key] = html.UnescapeString(m[2])
			}
		}
	}
	for _, m := range metaTagAltRegex.FindAllStringSubmatch(rawHTML, -1) {
		if len(m) == 3 {
			key := strings.ToLower(strings.TrimSpace(m[2]))
			if _, exists := out[key]; !exists {
				out[key] = html.UnescapeString(m[1])
			}
		}
	}
	return out
}
