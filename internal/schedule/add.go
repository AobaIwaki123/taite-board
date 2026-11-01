package schedule

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

type AddSessionOptions struct {
	Date             string
	Title            string
	SessionName      string
	VenueName        string
	LiveStartTime    string
	LiveEndTime      string
	BenefitType      string
	BenefitStartTime string
	BenefitEndTime   string
	Booth            string
	TicketPlatform   string
	TicketURL        string
	ImageURL         string
	SourceTweetURL   string
	SpecialNotes     string
	FilePath         string
	SyncTargets      []string
}

// AddSession appends a new session with an auto-derived ID (YYYYMMDD-N),
// sorts all sessions chronologically, and syncs copies.
func AddSession(opts AddSessionOptions) (string, error) {
	if opts.Date == "" || opts.Title == "" || opts.VenueName == "" {
		return "", fmt.Errorf("date, title, and venue name are required")
	}
	if opts.LiveStartTime == "" {
		opts.LiveStartTime = "--:--"
	}
	if opts.FilePath == "" {
		opts.FilePath = "data/generated/schedule.json"
	}

	parsedDate, err := time.Parse("2006-01-02", opts.Date)
	if err != nil {
		return "", fmt.Errorf("invalid date format '%s' (expected YYYY-MM-DD): %w", opts.Date, err)
	}

	days := []string{"日", "月", "火", "水", "木", "金", "土"}
	dayOfWeek := days[parsedDate.Weekday()]

	raw, err := os.ReadFile(opts.FilePath)
	if err != nil {
		return "", fmt.Errorf("reading schedule file: %w", err)
	}

	var doc rootDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return "", fmt.Errorf("unmarshaling schedule json: %w", err)
	}

	// Calculate next session index for this date
	datePrefix := parsedDate.Format("20060102")
	count := 0
	for _, sRaw := range doc.Sessions {
		var s struct {
			Date string `json:"date"`
		}
		_ = json.Unmarshal(sRaw, &s)
		if s.Date == opts.Date {
			count++
		}
	}
	newID := fmt.Sprintf("%s-%d", datePrefix, count+1)

	// Build Venue object
	venueID := fmt.Sprintf("venue:%s", strings.ToLower(strings.ReplaceAll(opts.VenueName, " ", "-")))
	venue := map[string]interface{}{
		"canonical_id": venueID,
		"name":         opts.VenueName,
		"map_query":    opts.VenueName,
		"map_url":      fmt.Sprintf("https://maps.google.com/?q=%s", opts.VenueName),
	}

	// Build Live object
	live := map[string]interface{}{
		"start_time": opts.LiveStartTime,
		"end_time":   opts.LiveEndTime,
		"stage":      nil,
	}

	// Build Benefit object
	var benefit map[string]interface{}
	if opts.BenefitStartTime != "" || opts.BenefitType != "" {
		benType := opts.BenefitType
		if benType == "" {
			benType = "並行"
		}
		benefit = map[string]interface{}{
			"type":       benType,
			"start_time": opts.BenefitStartTime,
			"end_time":   opts.BenefitEndTime,
			"booth":      opts.Booth,
		}
	}

	// Build Ticket object
	var ticket map[string]interface{}
	if opts.TicketPlatform != "" || opts.TicketURL != "" {
		ticket = map[string]interface{}{
			"platform": opts.TicketPlatform,
			"url":      opts.TicketURL,
			"prices":   []interface{}{},
		}
	}

	// Build Source Refs
	var sourceRefs []map[string]interface{}
	if opts.SourceTweetURL != "" {
		matches := statusIDRegex.FindStringSubmatch(opts.SourceTweetURL)
		if len(matches) > 1 {
			sourceRefs = append(sourceRefs, map[string]interface{}{
				"type":      "x_post",
				"source_id": matches[1],
				"url":       opts.SourceTweetURL,
			})
		}
	}

	newSession := map[string]interface{}{
		"id":               newID,
		"title":            opts.Title,
		"date":             opts.Date,
		"day_of_week":      dayOfWeek,
		"session_name":     opts.SessionName,
		"venue":            venue,
		"live":             live,
		"benefit":          benefit,
		"photo":            nil,
		"special_notes":    opts.SpecialNotes,
		"admission_bonus":  nil,
		"ticket":           ticket,
		"image_url":        opts.ImageURL,
		"source_tweet_url": opts.SourceTweetURL,
		"images":           []interface{}{},
		"source_refs":      sourceRefs,
	}

	sessBytes, err := json.Marshal(newSession)
	if err != nil {
		return "", fmt.Errorf("marshaling new session: %w", err)
	}

	doc.Sessions = append(doc.Sessions, json.RawMessage(sessBytes))

	// Write back temporarily
	updatedRaw, err := json.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("marshaling updated doc: %w", err)
	}
	if err := os.WriteFile(opts.FilePath, updatedRaw, 0644); err != nil {
		return "", fmt.Errorf("writing schedule file: %w", err)
	}

	// Run SortSchedule to guarantee chronological sort and sync
	if _, err := SortSchedule(opts.FilePath, opts.SyncTargets...); err != nil {
		return "", fmt.Errorf("sorting after add: %w", err)
	}

	return newID, nil
}
