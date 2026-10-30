package validate

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sync"
	"time"
	"unicode"
)

var (
	dateRegex   = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	timeRegex   = regexp.MustCompile(`^(\d{2}:\d{2}|--:--)$`)
	xStatusRegex = regexp.MustCompile(`^https?://(x\.com|twitter\.com)/[^/]+/status/\d+`)
)

type ValidationReport struct {
	TotalSessions int      `json:"total_sessions"`
	Errors        []string `json:"errors"`
	Warnings      []string `json:"warnings"`
	IsValid       bool     `json:"is_valid"`
}

type Options struct {
	CheckRemoteURLs bool
	HTTPTimeout     time.Duration
}

// containsEmoji reports whether a string contains any emoji runes.
func containsEmoji(s string) bool {
	for _, r := range s {
		if unicode.In(r, unicode.Variation_Selector) {
			continue
		}
		// Common emoji ranges
		if (r >= 0x1F600 && r <= 0x1F64F) || // Emoticons
			(r >= 0x1F300 && r <= 0x1F5FF) || // Misc Symbols and Pictographs
			(r >= 0x1F680 && r <= 0x1F6FF) || // Transport and Map
			(r >= 0x1F700 && r <= 0x1F77F) || // Alchemical Symbols
			(r >= 0x1F780 && r <= 0x1F7FF) || // Geometric Shapes Extended
			(r >= 0x1F800 && r <= 0x1F8FF) || // Supplemental Arrows-C
			(r >= 0x1F900 && r <= 0x1F9FF) || // Supplemental Symbols and Pictographs
			(r >= 0x1FA00 && r <= 0x1FA6F) || // Chess Symbols
			(r >= 0x1FA70 && r <= 0x1FAFF) || // Symbols and Pictographs Extended-A
			(r >= 0x2600 && r <= 0x26FF) || // Misc symbols
			(r >= 0x2700 && r <= 0x27BF) { // Dingbats
			return true
		}
	}
	return false
}

// ValidateFile checks the given schedule JSON file against ADR-0001 rules and integrity constraints.
func ValidateFile(filePath string) (*ValidationReport, error) {
	return ValidateFileWithOptions(filePath, Options{})
}

// ValidateFileWithOptions checks the given schedule JSON file with configurable validation options.
func ValidateFileWithOptions(filePath string, opts Options) (*ValidationReport, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	var root struct {
		SchemaVersion int                      `json:"schema_version"`
		GeneratedAt   string                   `json:"generated_at"`
		Timezone      string                   `json:"timezone"`
		TargetGroup   *struct{ Name string }   `json:"target_group"`
		Sessions      []map[string]interface{} `json:"sessions"`
	}

	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parsing json: %w", err)
	}

	report := &ValidationReport{
		TotalSessions: len(root.Sessions),
		Errors:        []string{},
		Warnings:      []string{},
		IsValid:       true,
	}

	if root.SchemaVersion < 1 {
		report.Errors = append(report.Errors, "schema_version must be >= 1")
	}
	if root.GeneratedAt == "" {
		report.Errors = append(report.Errors, "generated_at is required")
	}
	if root.Timezone == "" {
		report.Errors = append(report.Errors, "timezone is required")
	}
	if root.TargetGroup == nil || root.TargetGroup.Name == "" {
		report.Errors = append(report.Errors, "target_group.name is required")
	}

	seenIDs := make(map[string]bool)
	var remoteURLsToCheck []string

	for i, s := range root.Sessions {
		id, _ := s["id"].(string)
		title, _ := s["title"].(string)
		date, _ := s["date"].(string)

		prefix := fmt.Sprintf("session[%d] (id=%s)", i, id)

		if id == "" {
			report.Errors = append(report.Errors, fmt.Sprintf("%s: missing id", prefix))
		} else if seenIDs[id] {
			report.Errors = append(report.Errors, fmt.Sprintf("%s: duplicate session id '%s'", prefix, id))
		} else {
			seenIDs[id] = true
		}

		if title == "" {
			report.Errors = append(report.Errors, fmt.Sprintf("%s: missing title", prefix))
		}

		if !dateRegex.MatchString(date) {
			report.Errors = append(report.Errors, fmt.Sprintf("%s: invalid date format '%s' (expected YYYY-MM-DD)", prefix, date))
		}

		// Check for forbidden emojis in text fields
		if containsEmoji(title) {
			report.Warnings = append(report.Warnings, fmt.Sprintf("%s: title contains emoji (violates text integrity rule)", prefix))
		}
		if sn, ok := s["special_notes"].(string); ok && containsEmoji(sn) {
			report.Warnings = append(report.Warnings, fmt.Sprintf("%s: special_notes contains emoji (violates text integrity rule)", prefix))
		}
		if ab, ok := s["admission_bonus"].(string); ok && containsEmoji(ab) {
			report.Warnings = append(report.Warnings, fmt.Sprintf("%s: admission_bonus contains emoji (violates text integrity rule)", prefix))
		}

		// Venue
		venue, _ := s["venue"].(map[string]interface{})
		if venue == nil || venue["name"] == "" {
			report.Errors = append(report.Errors, fmt.Sprintf("%s: missing venue.name", prefix))
		}

		// Live
		live, _ := s["live"].(map[string]interface{})
		var liveStart, liveEnd string
		if live == nil {
			report.Errors = append(report.Errors, fmt.Sprintf("%s: missing live object", prefix))
		} else {
			liveStart, _ = live["start_time"].(string)
			liveEnd, _ = live["end_time"].(string)
			if !timeRegex.MatchString(liveStart) {
				report.Warnings = append(report.Warnings, fmt.Sprintf("%s: unusual live.start_time '%s'", prefix, liveStart))
			}
			if liveEnd != "" && !timeRegex.MatchString(liveEnd) {
				report.Warnings = append(report.Warnings, fmt.Sprintf("%s: unusual live.end_time '%s'", prefix, liveEnd))
			}
			if liveStart != "" && liveEnd != "" && liveStart != "--:--" && liveEnd != "--:--" && liveEnd <= liveStart {
				report.Errors = append(report.Errors, fmt.Sprintf("%s: live.end_time '%s' must be after start_time '%s'", prefix, liveEnd, liveStart))
			}
		}

		// Benefit
		benefit, _ := s["benefit"].(map[string]interface{})
		if benefit != nil {
			benStart, _ := benefit["start_time"].(string)
			benEnd, _ := benefit["end_time"].(string)
			if benStart != "" && !timeRegex.MatchString(benStart) {
				report.Warnings = append(report.Warnings, fmt.Sprintf("%s: unusual benefit.start_time '%s'", prefix, benStart))
			}
			if benEnd != "" && !timeRegex.MatchString(benEnd) {
				report.Warnings = append(report.Warnings, fmt.Sprintf("%s: unusual benefit.end_time '%s'", prefix, benEnd))
			}
			if liveStart == "--:--" && benStart != "" && benStart != "--:--" {
				report.Warnings = append(report.Warnings, fmt.Sprintf("%s: live.start_time is pending (--:--) but benefit.start_time is set to '%s'", prefix, benStart))
			}
		}

		// Ticket
		ticket, _ := s["ticket"].(map[string]interface{})
		if ticket == nil {
			report.Errors = append(report.Errors, fmt.Sprintf("%s: missing ticket object", prefix))
		}

		// Source Tweet URL verification (prevent non-X URLs in source_tweet_url)
		if srcTweet, ok := s["source_tweet_url"].(string); ok && srcTweet != "" {
			if !xStatusRegex.MatchString(srcTweet) {
				report.Errors = append(report.Errors, fmt.Sprintf("%s: source_tweet_url '%s' is not a valid X/Twitter status URL", prefix, srcTweet))
			}
		}

		// Images validation
		if images, ok := s["images"].([]interface{}); ok {
			for imgIdx, imgItem := range images {
				imgMap, _ := imgItem.(map[string]interface{})
				if imgMap == nil {
					continue
				}
				imgType, _ := imgMap["type"].(string)
				imgURL, _ := imgMap["url"].(string)
				imgLabel, _ := imgMap["label"].(string)

				if imgType != "flyer" && imgType != "timetable" {
					report.Warnings = append(report.Warnings, fmt.Sprintf("%s: image[%d] type '%s' is unknown (expected 'flyer' or 'timetable')", prefix, imgIdx, imgType))
				}
				if imgLabel == "" {
					report.Warnings = append(report.Warnings, fmt.Sprintf("%s: image[%d] is missing label", prefix, imgIdx))
				}

				if imgURL == "" {
					report.Errors = append(report.Errors, fmt.Sprintf("%s: image[%d] url is empty", prefix, imgIdx))
				} else {
					parsedURL, parseErr := url.Parse(imgURL)
					if parseErr != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
						report.Errors = append(report.Errors, fmt.Sprintf("%s: image[%d] url '%s' must be an absolute HTTP/HTTPS URL (local/relative paths forbidden)", prefix, imgIdx, imgURL))
					} else if opts.CheckRemoteURLs {
						remoteURLsToCheck = append(remoteURLsToCheck, imgURL)
					}
				}
			}
		}
	}

	// Remote URL reachability check
	if opts.CheckRemoteURLs && len(remoteURLsToCheck) > 0 {
		timeout := opts.HTTPTimeout
		if timeout <= 0 {
			timeout = 5 * time.Second
		}
		client := &http.Client{Timeout: timeout}

		var mu sync.Mutex
		var wg sync.WaitGroup
		sem := make(chan struct{}, 5)

		for _, rawURL := range remoteURLsToCheck {
			u := rawURL
			wg.Add(1)
			go func() {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, u, nil)
				if err != nil {
					mu.Lock()
					report.Errors = append(report.Errors, fmt.Sprintf("invalid remote image request '%s': %v", u, err))
					mu.Unlock()
					return
				}
				// Set Range to avoid downloading whole image
				req.Header.Set("Range", "bytes=0-1024")
				req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; TaiteBoardValidator/1.0)")

				resp, err := client.Do(req)
				if err != nil {
					mu.Lock()
					report.Errors = append(report.Errors, fmt.Sprintf("remote image unreachable '%s': %v", u, err))
					mu.Unlock()
					return
				}
				defer resp.Body.Close()

				if resp.StatusCode < 200 || (resp.StatusCode >= 400 && resp.StatusCode != http.StatusRequestedRangeNotSatisfiable) {
					mu.Lock()
					report.Errors = append(report.Errors, fmt.Sprintf("remote image '%s' returned HTTP %d", u, resp.StatusCode))
					mu.Unlock()
				}
			}()
		}
		wg.Wait()
	}

	if len(report.Errors) > 0 {
		report.IsValid = false
	}

	return report, nil
}
