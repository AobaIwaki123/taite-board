package scrape

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/AobaIwaki123/taite-board/internal/raw"
	"github.com/AobaIwaki123/taite-board/internal/source/x"
)

// PostFetcher retrieves a single post snapshot from an external URL.
type PostFetcher interface {
	FetchPost(ctx context.Context, postURL string) (*raw.Post, error)
}

// TimelineRequest describes parameters for polling an account timeline or seed feed.
type TimelineRequest struct {
	Group      string
	Account    string
	UserID     string
	SinceID    string
	MaxResults int
}

// TimelineResult contains newly discovered posts and partial per-URL errors.
type TimelineResult struct {
	SourceType  string            `json:"source_type"`
	Posts       []*raw.Post       `json:"posts"`
	NewestID    string            `json:"newest_id,omitempty"`
	FetchErrors map[string]string `json:"fetch_errors,omitempty"`
}

// TimelineSource abstracts how new posts are discovered (mock URL list, fixture JSON, or official X API v2).
type TimelineSource interface {
	FetchTimeline(ctx context.Context, req TimelineRequest) (*TimelineResult, error)
}

// CompareSnowflakeID compares two numeric X status IDs.
// Non-numeric IDs fall back to lexical comparison.
func CompareSnowflakeID(a, b string) int {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == b {
		return 0
	}
	ia, okA := new(big.Int).SetString(a, 10)
	ib, okB := new(big.Int).SetString(b, 10)
	if okA && okB {
		return ia.Cmp(ib)
	}
	if a < b {
		return -1
	}
	return 1
}

// MaxSnowflakeID returns the greater of two numeric status IDs.
func MaxSnowflakeID(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	if CompareSnowflakeID(a, b) >= 0 {
		return a
	}
	return b
}

// URLListSource simulates timeline polling using an explicit list of known X post URLs.
// Status IDs older than or equal to req.SinceID are skipped before making network calls.
type URLListSource struct {
	URLs    []string
	Fetcher PostFetcher
	Timeout time.Duration
	Workers int
}

// LoadURLsFromFile reads non-empty, non-comment URLs from a text file.
func LoadURLsFromFile(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseURLLines(string(data)), nil
}

// ParseURLLines extracts trimmed unique non-comment URLs from newline-delimited input.
func ParseURLLines(rawText string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, line := range strings.Split(rawText, "\n") {
		u := strings.TrimSpace(line)
		if u == "" || strings.HasPrefix(u, "#") || seen[u] {
			continue
		}
		seen[u] = true
		out = append(out, u)
	}
	return out
}

// FetchTimeline resolves each eligible URL through PostFetcher while respecting SinceID and MaxResults.
func (s *URLListSource) FetchTimeline(ctx context.Context, req TimelineRequest) (*TimelineResult, error) {
	timeout := s.Timeout
	if timeout <= 0 {
		timeout = 12 * time.Second
	}
	workers := s.Workers
	if workers <= 0 {
		workers = 4
	}
	fetcher := s.Fetcher
	if fetcher == nil {
		fetcher = x.NewClient(timeout)
	}

	type candidateURL struct {
		url      string
		statusID string
	}
	var targets []candidateURL
	for _, rawURL := range s.URLs {
		u := strings.TrimSpace(rawURL)
		if u == "" {
			continue
		}
		sid := x.ExtractStatusID(u)
		if req.SinceID != "" && sid != "" && CompareSnowflakeID(sid, req.SinceID) <= 0 {
			continue
		}
		targets = append(targets, candidateURL{url: u, statusID: sid})
	}

	sort.Slice(targets, func(i, j int) bool {
		return CompareSnowflakeID(targets[i].statusID, targets[j].statusID) > 0
	})
	if req.MaxResults > 0 && len(targets) > req.MaxResults {
		targets = targets[:req.MaxResults]
	}

	res := &TimelineResult{
		SourceType:  "url_list",
		FetchErrors: make(map[string]string),
	}

	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, t := range targets {
		t := t
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			reqCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			post, err := fetcher.FetchPost(reqCtx, t.url)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				res.FetchErrors[t.url] = err.Error()
				return
			}
			if post == nil {
				res.FetchErrors[t.url] = "empty post returned"
				return
			}
			if post.SourceID == "" {
				post.SourceID = t.statusID
			}
			if post.FetchedAt == "" {
				post.FetchedAt = time.Now().UTC().Format(time.RFC3339)
			}
			res.Posts = append(res.Posts, post)
			res.NewestID = MaxSnowflakeID(res.NewestID, post.SourceID)
		}()
	}
	wg.Wait()

	sort.Slice(res.Posts, func(i, j int) bool {
		return CompareSnowflakeID(res.Posts[i].SourceID, res.Posts[j].SourceID) > 0
	})
	if len(res.FetchErrors) == 0 {
		res.FetchErrors = nil
	}
	return res, nil
}

// FixtureSource reads recorded raw.Post snapshots from disk for deterministic zero-cost testing.
type FixtureSource struct {
	Path string
}

// FetchTimeline loads recorded posts from JSON (either an array or {"posts": [...]}).
func (s *FixtureSource) FetchTimeline(_ context.Context, req TimelineRequest) (*TimelineResult, error) {
	if strings.TrimSpace(s.Path) == "" {
		return nil, fmt.Errorf("fixture path is required")
	}
	data, err := os.ReadFile(s.Path)
	if err != nil {
		return nil, fmt.Errorf("reading fixture %s: %w", filepath.Clean(s.Path), err)
	}

	var posts []*raw.Post
	if err := json.Unmarshal(data, &posts); err != nil {
		var wrapped struct {
			Posts []*raw.Post `json:"posts"`
		}
		if wErr := json.Unmarshal(data, &wrapped); wErr != nil {
			return nil, fmt.Errorf("parsing fixture json: %w", err)
		}
		posts = wrapped.Posts
	}

	res := &TimelineResult{SourceType: "fixture"}
	for _, p := range posts {
		if p == nil {
			continue
		}
		if req.SinceID != "" && p.SourceID != "" && CompareSnowflakeID(p.SourceID, req.SinceID) <= 0 {
			continue
		}
		clone := *p
		if clone.Source == "" {
			clone.Source = "x"
		}
		if clone.FetchedAt == "" {
			clone.FetchedAt = time.Now().UTC().Format(time.RFC3339)
		}
		res.Posts = append(res.Posts, &clone)
		res.NewestID = MaxSnowflakeID(res.NewestID, clone.SourceID)
	}

	sort.Slice(res.Posts, func(i, j int) bool {
		return CompareSnowflakeID(res.Posts[i].SourceID, res.Posts[j].SourceID) > 0
	})
	if req.MaxResults > 0 && len(res.Posts) > req.MaxResults {
		res.Posts = res.Posts[:req.MaxResults]
	}
	return res, nil
}

// XAPISource polls the official X API v2 GET /2/users/:id/tweets endpoint using an App-only Bearer Token.
type XAPISource struct {
	BaseURL     string
	BearerToken string
	HTTPClient  *http.Client
}

// NewXAPISource constructs an official X API v2 timeline client.
func NewXAPISource(bearerToken string, timeout time.Duration) *XAPISource {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &XAPISource{
		BaseURL:     "https://api.x.com",
		BearerToken: strings.TrimSpace(bearerToken),
		HTTPClient:  &http.Client{Timeout: timeout},
	}
}

// FetchTimeline fetches incremental tweets for req.UserID after req.SinceID via X API v2.
func (s *XAPISource) FetchTimeline(ctx context.Context, req TimelineRequest) (*TimelineResult, error) {
	if strings.TrimSpace(s.BearerToken) == "" {
		return nil, fmt.Errorf("X_BEARER_TOKEN is required for official X API polling")
	}
	userID := strings.TrimSpace(req.UserID)
	if userID == "" {
		return nil, fmt.Errorf("numeric x_user_id is required to poll GET /2/users/:id/tweets")
	}
	baseURL := strings.TrimRight(s.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://api.x.com"
	}

	maxRes := req.MaxResults
	if maxRes < 5 {
		maxRes = 10
	} else if maxRes > 100 {
		maxRes = 100
	}

	params := url.Values{}
	params.Set("max_results", fmt.Sprintf("%d", maxRes))
	params.Set("exclude", "replies,retweets")
	params.Set("expansions", "attachments.media_keys")
	params.Set("tweet.fields", "text,created_at,note_tweet,attachments,entities")
	params.Set("media.fields", "url,type,preview_image_url")
	if req.SinceID != "" {
		params.Set("since_id", req.SinceID)
	}

	endpoint := fmt.Sprintf("%s/2/users/%s/tweets?%s", baseURL, url.PathEscape(userID), params.Encode())
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("creating x api request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+s.BearerToken)

	client := s.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("calling x api: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("reading x api response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("x api returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload struct {
		Data []struct {
			ID        string `json:"id"`
			Text      string `json:"text"`
			CreatedAt string `json:"created_at"`
			NoteTweet *struct {
				Text     string `json:"text"`
				Entities *struct {
					URLs []xAPIURLEntity `json:"urls"`
				} `json:"entities"`
			} `json:"note_tweet"`
			Entities *struct {
				URLs []xAPIURLEntity `json:"urls"`
			} `json:"entities"`
			Attachments *struct {
				MediaKeys []string `json:"media_keys"`
			} `json:"attachments"`
		} `json:"data"`
		Includes struct {
			Media []struct {
				MediaKey        string `json:"media_key"`
				Type            string `json:"type"`
				URL             string `json:"url"`
				PreviewImageURL string `json:"preview_image_url"`
			} `json:"media"`
		} `json:"includes"`
		Meta struct {
			NewestID string `json:"newest_id"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("parsing x api json: %w", err)
	}

	mediaMap := make(map[string]string, len(payload.Includes.Media))
	for _, m := range payload.Includes.Media {
		u := strings.TrimSpace(m.URL)
		if u == "" && m.Type == "photo" {
			u = strings.TrimSpace(m.PreviewImageURL)
		}
		if m.Type == "photo" && u != "" {
			mediaMap[m.MediaKey] = ensureLargeTwitterMediaURL(u)
		}
	}

	account := strings.TrimPrefix(strings.TrimSpace(req.Account), "@")
	fetchedNow := time.Now().UTC().Format(time.RFC3339)
	res := &TimelineResult{
		SourceType: "x_api_v2",
		NewestID:   payload.Meta.NewestID,
	}

	for _, tw := range payload.Data {
		text := tw.Text
		var urlEntities []xAPIURLEntity
		if tw.Entities != nil {
			urlEntities = append(urlEntities, tw.Entities.URLs...)
		}
		if tw.NoteTweet != nil && strings.TrimSpace(tw.NoteTweet.Text) != "" {
			text = tw.NoteTweet.Text
			if tw.NoteTweet.Entities != nil {
				urlEntities = append(urlEntities, tw.NoteTweet.Entities.URLs...)
			}
		}
		text = expandXAPIURLs(text, urlEntities)

		var imgs []string
		if tw.Attachments != nil {
			for _, mk := range tw.Attachments.MediaKeys {
				if u, ok := mediaMap[mk]; ok {
					imgs = append(imgs, u)
				}
			}
		}

		postURL := fmt.Sprintf("https://x.com/i/web/status/%s", tw.ID)
		authorURL := ""
		if account != "" {
			postURL = fmt.Sprintf("https://x.com/%s/status/%s", account, tw.ID)
			authorURL = fmt.Sprintf("https://x.com/%s", account)
		}

		res.Posts = append(res.Posts, &raw.Post{
			SchemaVersion: 1,
			Source:        "x",
			SourceID:      tw.ID,
			URL:           postURL,
			Author:        account,
			AuthorURL:     authorURL,
			Text:          strings.TrimSpace(text),
			Images:        imgs,
			FetchedAt:     fetchedNow,
		})
		res.NewestID = MaxSnowflakeID(res.NewestID, tw.ID)
	}

	return res, nil
}

type xAPIURLEntity struct {
	URL         string `json:"url"`
	ExpandedURL string `json:"expanded_url"`
}

func expandXAPIURLs(text string, entities []xAPIURLEntity) string {
	out := text
	for _, ent := range entities {
		if ent.URL != "" && ent.ExpandedURL != "" {
			out = strings.ReplaceAll(out, ent.URL, ent.ExpandedURL)
		}
	}
	return out
}

func ensureLargeTwitterMediaURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host != "pbs.twimg.com" {
		return rawURL
	}
	q := parsed.Query()
	if q.Get("name") == "" {
		q.Set("name", "large")
		parsed.RawQuery = q.Encode()
	}
	return parsed.String()
}
