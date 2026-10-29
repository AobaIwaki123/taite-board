package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// PostData holds structured post information extracted from an X/Twitter post.
type PostData struct {
	URL       string   `json:"url"`
	Author    string   `json:"author"`
	AuthorURL string   `json:"author_url,omitempty"`
	Text      string   `json:"text"`
	Images    []string `json:"images"`
	FetchedAt string   `json:"fetched_at"`
}

type oembedResponse struct {
	URL        string `json:"url"`
	AuthorName string `json:"author_name"`
	AuthorURL  string `json:"author_url"`
	HTML       string `json:"html"`
}

var (
	tagRegex       = regexp.MustCompile(`<[^>]*>`)
	pbsImageRegex  = regexp.MustCompile(`https://pbs\.twimg\.com/media/[a-zA-Z0-9_-]+`)
	statusIDRegex  = regexp.MustCompile(`/status/(\d+)`)
)

func main() {
	var (
		timeoutSec  = flag.Int("timeout", 5, "HTTP request timeout in seconds")
		outputPath  = flag.String("out", "", "Output file path (default stdout)")
		downloadDir = flag.String("download", "", "Download extracted images to directory")
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: taite-fetch [options] <x-post-url>\n\nOptions:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	targetURL := flag.Arg(0)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeoutSec)*time.Second)
	defer cancel()

	data, err := extractPost(ctx, targetURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Download images if requested
	if *downloadDir != "" && len(data.Images) > 0 {
		if err := os.MkdirAll(*downloadDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create download dir: %v\n", err)
		} else {
			for i, imgURL := range data.Images {
				fileName := fmt.Sprintf("image_%d.jpg", i+1)
				savePath := filepath.Join(*downloadDir, fileName)
				if err := downloadImage(ctx, imgURL, savePath); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to download %s: %v\n", imgURL, err)
				} else {
					fmt.Fprintf(os.Stderr, "Downloaded image to %s\n", savePath)
				}
			}
		}
	}

	outputJSON, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to format JSON: %v\n", err)
		os.Exit(1)
	}

	if *outputPath != "" {
		if err := os.WriteFile(*outputPath, outputJSON, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write output file: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Wrote extracted data to %s\n", *outputPath)
	} else {
		fmt.Println(string(outputJSON))
	}
}

func extractPost(ctx context.Context, rawURL string) (*PostData, error) {
	// Normalize URL
	normalizedURL := strings.Replace(rawURL, "x.com/", "twitter.com/", 1)
	if u, err := url.Parse(normalizedURL); err == nil {
		u.RawQuery = "" // strip query params
		normalizedURL = u.String()
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	var (
		wg      sync.WaitGroup
		oembed  *oembedResponse
		images  []string
		oeErr   error
		imgErr  error
	)

	wg.Add(2)

	// Fetch oEmbed in parallel
	go func() {
		defer wg.Done()
		oembed, oeErr = fetchOEmbed(ctx, client, normalizedURL)
	}()

	// Fetch OG Images in parallel
	go func() {
		defer wg.Done()
		images, imgErr = fetchOGImages(ctx, client, normalizedURL)
	}()

	wg.Wait()

	if oeErr != nil && imgErr != nil {
		return nil, fmt.Errorf("failed both oEmbed (%v) and image extraction (%v)", oeErr, imgErr)
	}

	result := &PostData{
		URL:       normalizedURL,
		Images:    images,
		FetchedAt: time.Now().Format(time.RFC3339),
	}

	if oembed != nil {
		result.Author = oembed.AuthorName
		result.AuthorURL = oembed.AuthorURL
		result.Text = cleanHTMLText(oembed.HTML)
	}

	return result, nil
}

func fetchOEmbed(ctx context.Context, client *http.Client, tweetURL string) (*oembedResponse, error) {
	apiURL := fmt.Sprintf("https://publish.twitter.com/oembed?url=%s", url.QueryEscape(tweetURL))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; taite-fetch/1.0)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oEmbed returned status %d", resp.StatusCode)
	}

	var res oembedResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	return &res, nil
}

func fetchOGImages(ctx context.Context, client *http.Client, tweetURL string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tweetURL, nil)
	if err != nil {
		return nil, err
	}
	// Facebook crawler user agent prompts Twitter to return OGP meta tags including pbs.twimg.com image URLs
	req.Header.Set("User-Agent", "facebookexternalhit/1.1 (+http://www.facebook.com/externalhit_uatext.php)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read limited body (first 256KB is plenty for head meta tags)
	lr := io.LimitReader(resp.Body, 256*1024)
	bodyBytes, err := io.ReadAll(lr)
	if err != nil {
		return nil, err
	}

	bodyStr := string(bodyBytes)
	matches := pbsImageRegex.FindAllString(bodyStr, -1)

	// Deduplicate and format to high quality
	seen := make(map[string]bool)
	var images []string
	for _, m := range matches {
		if !seen[m] {
			seen[m] = true
			// Append format=jpg&name=orig for highest resolution
			highResURL := fmt.Sprintf("%s?format=jpg&name=orig", m)
			images = append(images, highResURL)
		}
	}

	return images, nil
}

func cleanHTMLText(rawHTML string) string {
	// Extract content inside <p>...</p>
	pStart := strings.Index(rawHTML, "<p")
	pEnd := strings.Index(rawHTML, "</p>")
	content := rawHTML
	if pStart != -1 && pEnd != -1 && pEnd > pStart {
		inner := rawHTML[pStart:pEnd]
		if closeTag := strings.Index(inner, ">"); closeTag != -1 {
			content = inner[closeTag+1:]
		}
	}

	// Replace <br> with newline
	content = strings.ReplaceAll(content, "<br>", "\n")
	content = strings.ReplaceAll(content, "<br/>", "\n")
	content = strings.ReplaceAll(content, "<br />", "\n")

	// Strip remaining HTML tags
	content = tagRegex.ReplaceAllString(content, "")

	// Unescape HTML entities (&amp;, &lt;, etc.)
	content = html.UnescapeString(content)

	return strings.TrimSpace(content)
}

func downloadImage(ctx context.Context, imgURL string, destPath string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imgURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("image fetch returned status %d", resp.StatusCode)
	}

	outFile, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, resp.Body)
	return err
}
