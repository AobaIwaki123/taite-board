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
	VenueCanonicalID string
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
	venueID := strings.TrimSpace(opts.VenueCanonicalID)
	if venueID == "" {
		venueID = fmt.Sprintf("venue:%s", strings.ToLower(strings.ReplaceAll(opts.VenueName, " ", "-")))
	}
	venue := map[string]interface{}{
		"canonical_id": venueID,
		"name":         opts.VenueName,
		"map_query":    opts.VenueName,
		"map_url":      fmt.Sprintf("https://maps.google.com/?q=%s", opts.VenueName),
	}

	// Build Ticket platform & URL
	ticketPlatform := strings.TrimSpace(opts.TicketPlatform)
	if ticketPlatform == "" && opts.TicketURL != "" {
		ticketPlatform = detectTicketPlatform(opts.TicketURL)
	}
	var ticketURL interface{}
	if opts.TicketURL != "" {
		ticketURL = opts.TicketURL
	}
	ticket := map[string]interface{}{
		"platform": ticketPlatform,
		"url":      ticketURL,
		"prices":   []interface{}{},
	}

	// Build Source Refs & official X Tweet URL (validate requires source_tweet_url to be an X status URL when non-empty)
	var sourceRefs []map[string]interface{}
	officialTweetURL := ""
	if ref, isX := buildSourceRefFromURL(opts.SourceTweetURL); ref != nil {
		sourceRefs = append(sourceRefs, ref)
		if isX {
			officialTweetURL = strings.TrimSpace(opts.SourceTweetURL)
		}
	}

	// Dedupe / Upsert: if an existing session on the same date matches by normalized title or canonical venue (especially placeholder --:-- sessions), merge into it instead of duplicating.
	targetTitleKey := normalizeMatchKey(opts.Title)
	targetVenueKey := normalizeMatchKey(opts.VenueName)
	for i, sRaw := range doc.Sessions {
		var sMap map[string]interface{}
		if err := json.Unmarshal(sRaw, &sMap); err != nil {
			continue
		}
		if sDate, _ := sMap["date"].(string); sDate != opts.Date {
			continue
		}

		existingID, _ := sMap["id"].(string)
		existingTitle, _ := sMap["title"].(string)
		existingVenueMap, _ := sMap["venue"].(map[string]interface{})
		existingVenueID, _ := existingVenueMap["canonical_id"].(string)
		existingVenueName, _ := existingVenueMap["name"].(string)
		existingLiveMap, _ := sMap["live"].(map[string]interface{})
		existingLiveStart, _ := existingLiveMap["start_time"].(string)

		titleMatched := keysOverlap(normalizeMatchKey(existingTitle), targetTitleKey)
		venueMatched := (venueID != "" && existingVenueID == venueID) || keysOverlap(normalizeMatchKey(existingVenueName), targetVenueKey)
		isPlaceholderOrSameTime := existingLiveStart == "" || existingLiveStart == "--:--" || existingLiveStart == opts.LiveStartTime

		if titleMatched || (venueMatched && isPlaceholderOrSameTime) {
			mergeSessionFields(sMap, opts, venue, ticketPlatform, officialTweetURL, sourceRefs)
			mergedBytes, mErr := json.Marshal(sMap)
			if mErr != nil {
				return nil, "", fmt.Errorf("marshaling merged session %s: %w", existingID, mErr)
			}
			doc.Sessions[i] = json.RawMessage(mergedBytes)
			updatedRaw, mErr := json.Marshal(doc)
			if mErr != nil {
				return nil, "", fmt.Errorf("marshaling merged doc: %w", mErr)
			}
			sortedRaw, _, mErr := ApplySortSchedule(updatedRaw)
			if mErr != nil {
				return nil, "", fmt.Errorf("sorting after merge: %w", mErr)
			}
			return sortedRaw, existingID, nil
		}
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
		"source_tweet_url": officialTweetURL,
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

func detectTicketPlatform(rawURL string) string {
	u := strings.ToLower(rawURL)
	switch {
	case strings.Contains(u, "ticketdive") || strings.Contains(u, "t-dv.com"):
		return "TicketDive"
	case strings.Contains(u, "livepocket"):
		return "LivePocket"
	case strings.Contains(u, "tiget.net"):
		return "TIGET"
	case strings.Contains(u, "eplus.jp"):
		return "イープラス"
	case strings.Contains(u, "ticketvillage"):
		return "TicketVillage"
	case strings.Contains(u, "pia.jp"):
		return "チケットぴあ"
	case strings.Contains(u, "l-tike.com"):
		return "ローチケ"
	default:
		return ""
	}
}

func normalizeMatchKey(s string) string {
	var sb strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || (r >= 0x3040 && r <= 0x9fff) || (r >= 0xac00 && r <= 0xd7af) {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

func keysOverlap(a, b string) bool {
	if len(a) < 3 || len(b) < 3 {
		return false
	}
	return a == b || strings.Contains(a, b) || strings.Contains(b, a)
}

func mergeSessionFields(sMap map[string]interface{}, opts AddSessionOptions, newVenue map[string]interface{}, ticketPlatform, officialTweetURL string, newRefs []map[string]interface{}) {
	// Venue: prefer resolved canonical venue or replace placeholder
	if vMap, ok := sMap["venue"].(map[string]interface{}); ok {
		exName, _ := vMap["name"].(string)
		if exName == "" || exName == "未定" || exName == "告知待ち" || strings.HasPrefix(opts.VenueCanonicalID, "venue:") {
			if opts.VenueName != "" && opts.VenueName != "未定" {
				sMap["venue"] = newVenue
			}
		}
	} else {
		sMap["venue"] = newVenue
	}

	// Live times: update when existing is placeholder or new time is concrete
	liveMap, _ := sMap["live"].(map[string]interface{})
	if liveMap == nil {
		liveMap = map[string]interface{}{"stage": nil}
	}
	if opts.LiveStartTime != "" && opts.LiveStartTime != "--:--" {
		liveMap["start_time"] = opts.LiveStartTime
		if opts.LiveEndTime != "" {
			liveMap["end_time"] = opts.LiveEndTime
		}
	}
	sMap["live"] = liveMap

	// Benefit: update times and non-placeholder booth
	benMap, _ := sMap["benefit"].(map[string]interface{})
	if benMap == nil {
		benMap = map[string]interface{}{}
	}
	if opts.BenefitStartTime != "" && opts.BenefitStartTime != "--:--" {
		benMap["start_time"] = opts.BenefitStartTime
		if opts.BenefitEndTime != "" {
			benMap["end_time"] = opts.BenefitEndTime
		}
		if exType, _ := benMap["type"].(string); exType == "" || exType == "未定" {
			if opts.BenefitType != "" {
				benMap["type"] = opts.BenefitType
			} else {
				benMap["type"] = "並行"
			}
		}
	}
	if opts.Booth != "" {
		exBooth, _ := benMap["booth"].(string)
		if exBooth == "" || exBooth == "告知待ち" || exBooth == "通常" {
			benMap["booth"] = opts.Booth
		}
	}
	sMap["benefit"] = benMap

	// Ticket: populate missing URL/platform while preserving existing prices
	if opts.TicketURL != "" {
		tMap, _ := sMap["ticket"].(map[string]interface{})
		if tMap == nil {
			tMap = map[string]interface{}{"prices": []interface{}{}}
		}
		exURL, _ := tMap["url"].(string)
		if exURL == "" {
			tMap["url"] = opts.TicketURL
			if ticketPlatform != "" {
				tMap["platform"] = ticketPlatform
			}
			sMap["ticket"] = tMap
		}
	}

	// Admission bonus: fill if missing
	if opts.AdmissionBonus != "" {
		exBonus, _ := sMap["admission_bonus"].(string)
		if exBonus == "" {
			sMap["admission_bonus"] = opts.AdmissionBonus
		}
	}

	// Special notes: preserve existing or set if empty
	if opts.SpecialNotes != "" {
		exNotes, _ := sMap["special_notes"].(string)
		if exNotes == "" {
			sMap["special_notes"] = opts.SpecialNotes
		}
	}

	// Image: populate if empty
	if opts.ImageURL != "" {
		if exImg, _ := sMap["image_url"].(string); exImg == "" {
			sMap["image_url"] = opts.ImageURL
		}
		if imgs, _ := sMap["images"].([]interface{}); len(imgs) == 0 {
			sMap["images"] = []interface{}{
				map[string]interface{}{
					"type":  "flyer",
					"label": "告知画像",
					"url":   opts.ImageURL,
				},
			}
		}
	}

	// Source tweet URL & source_refs merge
	if officialTweetURL != "" {
		if exTweet, _ := sMap["source_tweet_url"].(string); exTweet == "" {
			sMap["source_tweet_url"] = officialTweetURL
		}
	}
	if len(newRefs) > 0 {
		refsRaw, _ := sMap["source_refs"].([]interface{})
		for _, nr := range newRefs {
			nID, _ := nr["source_id"].(string)
			nURL, _ := nr["url"].(string)
			exists := false
			for _, er := range refsRaw {
				em, _ := er.(map[string]interface{})
				if em == nil {
					continue
				}
				eID, _ := em["source_id"].(string)
				eURL, _ := em["url"].(string)
				if (nID != "" && eID == nID) || (nURL != "" && eURL == nURL) {
					exists = true
					break
				}
			}
			if !exists {
				refsRaw = append(refsRaw, nr)
			}
		}
		sMap["source_refs"] = refsRaw
	}
}
