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
	AdmissionBonus   string
	FilePath         string
	SyncTargets      []string
}

// AddSession appends a new session with an auto-derived ID (YYYYMMDD-N),
// sorts all sessions chronologically, and syncs copies.
func AddSession(opts AddSessionOptions) (string, error) {
	if opts.FilePath == "" {
		opts.FilePath = "data/generated/schedule.json"
	}

	raw, err := os.ReadFile(opts.FilePath)
	if err != nil {
		return "", fmt.Errorf("reading schedule file: %w", err)
	}

	sortedRaw, newID, err := ApplyAddSession(raw, opts)
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(opts.FilePath, sortedRaw, 0644); err != nil {
		return "", fmt.Errorf("writing schedule file: %w", err)
	}

	if len(opts.SyncTargets) > 0 {
		if _, err := Sync(opts.FilePath, opts.SyncTargets...); err != nil {
			return "", fmt.Errorf("syncing after add: %w", err)
		}
	}

	return newID, nil
}

// ApplyAddSession appends a new session to schedule JSON bytes, sorts chronologically,
// updates generated_at, and returns the updated JSON bytes and derived session ID.
func ApplyAddSession(raw []byte, opts AddSessionOptions) ([]byte, string, error) {
	if opts.Date == "" || opts.Title == "" || opts.VenueName == "" {
		return nil, "", fmt.Errorf("date, title, and venue name are required")
	}
	if opts.LiveStartTime == "" {
		opts.LiveStartTime = "--:--"
	}

	parsedDate, err := time.Parse("2006-01-02", opts.Date)
	if err != nil {
		return nil, "", fmt.Errorf("invalid date format '%s' (expected YYYY-MM-DD): %w", opts.Date, err)
	}

	days := []string{"日", "月", "火", "水", "木", "金", "土"}
	dayOfWeek := days[parsedDate.Weekday()]

	var doc rootDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, "", fmt.Errorf("unmarshaling schedule json: %w", err)
	}
	if doc.SchemaVersion < 1 {
		doc.SchemaVersion = 1
	}
	if doc.Timezone == "" {
		doc.Timezone = "Asia/Tokyo"
	}

	// Calculate next unique session index for this date
	datePrefix := parsedDate.Format("20060102")
	existingIDs := make(map[string]bool)
	count := 0
	for _, sRaw := range doc.Sessions {
		var s struct {
			ID   string `json:"id"`
			Date string `json:"date"`
		}
		_ = json.Unmarshal(sRaw, &s)
		if s.ID != "" {
			existingIDs[s.ID] = true
		}
		if s.Date == opts.Date {
			count++
		}
	}
	idx := count + 1
	newID := fmt.Sprintf("%s-%d", datePrefix, idx)
	for existingIDs[newID] {
		idx++
		newID = fmt.Sprintf("%s-%d", datePrefix, idx)
	}

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

	// Build Benefit object (required by schema/schedule.schema.json)
	benType := opts.BenefitType
	if benType == "" {
		benType = "並行"
	}
	benStart := opts.BenefitStartTime
	if benStart == "" {
		benStart = "--:--"
	}
	benefit := map[string]interface{}{
		"type":       benType,
		"start_time": benStart,
		"end_time":   opts.BenefitEndTime,
		"booth":      opts.Booth,
	}

	// Build Ticket object (required by schema/schedule.schema.json)
	var ticketURL interface{}
	if opts.TicketURL != "" {
		ticketURL = opts.TicketURL
	}
	ticket := map[string]interface{}{
		"platform": opts.TicketPlatform,
		"url":      ticketURL,
		"prices":   []interface{}{},
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

	images := []interface{}{}
	if opts.ImageURL != "" {
		images = append(images, map[string]interface{}{
			"type":  "flyer",
			"label": "告知画像",
			"url":   opts.ImageURL,
		})
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
		"admission_bonus":  opts.AdmissionBonus,
		"ticket":           ticket,
		"image_url":        opts.ImageURL,
		"source_tweet_url": opts.SourceTweetURL,
		"images":           images,
		"source_refs":      sourceRefs,
	}

	sessBytes, err := json.Marshal(newSession)
	if err != nil {
		return nil, "", fmt.Errorf("marshaling new session: %w", err)
	}

	doc.Sessions = append(doc.Sessions, json.RawMessage(sessBytes))

	updatedRaw, err := json.Marshal(doc)
	if err != nil {
		return nil, "", fmt.Errorf("marshaling updated doc: %w", err)
	}

	sortedRaw, _, err := ApplySortSchedule(updatedRaw)
	if err != nil {
		return nil, "", fmt.Errorf("sorting after add: %w", err)
	}

	return sortedRaw, newID, nil
}
