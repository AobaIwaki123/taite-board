package promote

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/AobaIwaki123/taite-board/internal/candidate"
	"github.com/AobaIwaki123/taite-board/internal/resolve"
	"github.com/AobaIwaki123/taite-board/internal/schedule"
	"github.com/AobaIwaki123/taite-board/internal/validate"
)

var (
	daysOfWeek      = []string{"日", "月", "火", "水", "木", "金", "土"}
	statusIDRegex   = regexp.MustCompile(`/status/(\d+)`)
)

// PromoteOptions contains options for candidate promotion to schedule.json.
type PromoteOptions struct {
	CandidateFile       string   `json:"candidate_file"`
	TargetScheduleFile  string   `json:"target_schedule_file"`
	CuratedEntitiesFile string   `json:"curated_entities_file"`
	SyncTargets         []string `json:"sync_targets"`
	NoSync              bool     `json:"no_sync"`
	DryRun              bool     `json:"dry_run"`
}

// PromotedSessionSummary summarizes a promoted session for CLI diff display.
type PromotedSessionSummary struct {
	ID        string `json:"id"`
	Date      string `json:"date"`
	Title     string `json:"title"`
	Venue     string `json:"venue"`
	LiveTime  string `json:"live_time"`
	Action    string `json:"action"` // "added" or "updated"
}

// PromoteResult holds the outcome of candidate promotion.
type PromoteResult struct {
	Sessions    []PromotedSessionSummary `json:"sessions"`
	TotalAdded  int                      `json:"total_added"`
	TotalUpdated int                     `json:"total_updated"`
	DryRun      bool                     `json:"dry_run"`
	Message     string                   `json:"message"`
}

// PromoteCandidate reads a candidate JSON file, canonicalizes its sessions, validates them,
// merges them into the target schedule, sorts, and synchronizes copies.
func PromoteCandidate(opts PromoteOptions) (*PromoteResult, error) {
	if opts.CandidateFile == "" {
		return nil, fmt.Errorf("candidate file path is required")
	}
	if opts.TargetScheduleFile == "" {
		opts.TargetScheduleFile = "data/generated/schedule.json"
	}
	if !opts.NoSync && len(opts.SyncTargets) == 0 {
		opts.SyncTargets = schedule.DefaultSyncTargets("")
	}

	candBytes, err := os.ReadFile(opts.CandidateFile)
	if err != nil {
		return nil, fmt.Errorf("reading candidate file: %w", err)
	}

	resolver, err := resolve.NewResolver(opts.CuratedEntitiesFile)
	if err != nil {
		return nil, fmt.Errorf("initializing entity resolver: %w", err)
	}

	// 1. Parse candidate input (Supports candidate contract, IntakeResult, or direct sessions)
	rawSessions, err := extractCandidateSessions(candBytes)
	if err != nil {
		return nil, fmt.Errorf("extracting candidate sessions: %w", err)
	}
	if len(rawSessions) == 0 {
		return nil, fmt.Errorf("no candidate sessions found in %s", opts.CandidateFile)
	}

	// 2. Read existing schedule
	schedBytes, err := os.ReadFile(opts.TargetScheduleFile)
	if err != nil {
		return nil, fmt.Errorf("reading target schedule %s: %w", opts.TargetScheduleFile, err)
	}

	var root map[string]interface{}
	if err := json.Unmarshal(schedBytes, &root); err != nil {
		return nil, fmt.Errorf("parsing target schedule json: %w", err)
	}

	existingSessionsRaw, ok := root["sessions"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("target schedule 'sessions' array missing or invalid")
	}

	// 3. Process and normalize each candidate session
	var summaries []PromotedSessionSummary
	addedCount := 0
	updatedCount := 0

	for _, candSess := range rawSessions {
		normalizedSess, summary, err := normalizeAndMergeSession(candSess, existingSessionsRaw, resolver)
		if err != nil {
			return nil, fmt.Errorf("normalizing candidate session: %w", err)
		}

		if summary.Action == "updated" {
			// Replace in existing list
			for i, ex := range existingSessionsRaw {
				exMap, _ := ex.(map[string]interface{})
				if exMap["id"] == summary.ID {
					existingSessionsRaw[i] = normalizedSess
					break
				}
			}
			updatedCount++
		} else {
			// Append new session
			existingSessionsRaw = append(existingSessionsRaw, normalizedSess)
			addedCount++
		}
		summaries = append(summaries, summary)
	}

	root["sessions"] = existingSessionsRaw
	root["generated_at"] = time.Now().Format(time.RFC3339)

	res := &PromoteResult{
		Sessions:     summaries,
		TotalAdded:   addedCount,
		TotalUpdated: updatedCount,
		DryRun:       opts.DryRun,
		Message:      fmt.Sprintf("Promoted %d session(s) (%d added, %d updated)", len(summaries), addedCount, updatedCount),
	}

	if opts.DryRun {
		return res, nil
	}

	// 4. Save and Sort
	updatedBytes, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshaling updated schedule: %w", err)
	}
	if err := os.WriteFile(opts.TargetScheduleFile, append(updatedBytes, '\n'), 0644); err != nil {
		return nil, fmt.Errorf("writing updated schedule: %w", err)
	}

	// Chronological sort and sync
	var targets []string
	if !opts.NoSync {
		targets = opts.SyncTargets
	}
	if _, err := schedule.SortSchedule(opts.TargetScheduleFile, targets...); err != nil {
		return nil, fmt.Errorf("sorting and syncing schedule: %w", err)
	}

	// 5. Final Schema and Integrity Validation
	report, err := validate.ValidateFile(opts.TargetScheduleFile)
	if err != nil {
		return nil, fmt.Errorf("validating target schedule: %w", err)
	}
	if !report.IsValid {
		return nil, fmt.Errorf("schema validation failed after promotion: %s", strings.Join(report.Errors, "; "))
	}

	return res, nil
}

func extractCandidateSessions(data []byte) ([]map[string]interface{}, error) {
	// Try parsing as candidate intake contract
	var contractDoc struct {
		Contract string                   `json:"contract"`
		Sessions []map[string]interface{} `json:"sessions"`
	}
	if err := json.Unmarshal(data, &contractDoc); err == nil && len(contractDoc.Sessions) > 0 {
		return contractDoc.Sessions, nil
	}

	// Try parsing as IntakeResult
	var intake candidate.IntakeResult
	if err := json.Unmarshal(data, &intake); err == nil && (intake.Candidate.Date != "" || intake.Candidate.GroupRaw != "") {
		sess := convertIntakeToSession(intake)
		return []map[string]interface{}{sess}, nil
	}

	// Try parsing as direct array of sessions
	var sessionArray []map[string]interface{}
	if err := json.Unmarshal(data, &sessionArray); err == nil && len(sessionArray) > 0 {
		return sessionArray, nil
	}

	// Try parsing as single session
	var singleSession map[string]interface{}
	if err := json.Unmarshal(data, &singleSession); err == nil && singleSession["title"] != nil {
		return []map[string]interface{}{singleSession}, nil
	}

	return nil, fmt.Errorf("unrecognized candidate format")
}

func convertIntakeToSession(intake candidate.IntakeResult) map[string]interface{} {
	venueName := intake.Candidate.VenueRaw
	if venueName == "" {
		venueName = "会場未定"
	}

	sess := map[string]interface{}{
		"title":       intake.Text,
		"date":        intake.Candidate.Date,
		"venue": map[string]interface{}{
			"name":         venueName,
			"canonical_id": intake.Candidate.VenueCanonicalID,
		},
		"live": map[string]interface{}{
			"start_time": intake.Candidate.LiveStartRaw,
			"end_time":   intake.Candidate.LiveEndRaw,
			"stage":      nil,
		},
		"source_tweet_url": intake.URL,
	}

	if intake.Candidate.BenefitStartRaw != "" {
		sess["benefit"] = map[string]interface{}{
			"type":       "並行",
			"start_time": intake.Candidate.BenefitStartRaw,
			"end_time":   intake.Candidate.BenefitEndRaw,
		}
	}

	if len(intake.Images) > 0 {
		sess["image_url"] = intake.Images[0]
	}

	if intake.URL != "" {
		sess["source_refs"] = []interface{}{
			map[string]interface{}{
				"type":      "x_post",
				"source_id": intake.SourceID,
				"url":       intake.URL,
			},
		}
	}

	return sess
}

func normalizeAndMergeSession(
	cand map[string]interface{},
	existingSessions []interface{},
	resolver *resolve.Resolver,
) (map[string]interface{}, PromotedSessionSummary, error) {
	// Date validation & Day of Week calculation
	dateStr, ok := cand["date"].(string)
	if !ok || dateStr == "" {
		return nil, PromotedSessionSummary{}, fmt.Errorf("session date is required")
	}
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return nil, PromotedSessionSummary{}, fmt.Errorf("invalid date format %q: %w", dateStr, err)
	}
	dayOfWeek := daysOfWeek[t.Weekday()]

	// Title
	title, _ := cand["title"].(string)
	if title == "" {
		title = "未定イベント"
	}

	// Venue normalization
	var venue map[string]interface{}
	if v, ok := cand["venue"].(map[string]interface{}); ok {
		venue = v
	} else {
		venue = make(map[string]interface{})
	}
	venueName, _ := venue["name"].(string)
	if venueName == "" {
		venueName = "会場未定"
		venue["name"] = venueName
	}
	canonicalVenueID, _ := venue["canonical_id"].(string)
	if canonicalVenueID == "" {
		cid, displayName := resolver.ResolveVenueName(venueName)
		venue["canonical_id"] = cid
		venue["name"] = displayName
		venueName = displayName
	}
	if _, ok := venue["map_query"]; !ok {
		venue["map_query"] = venueName
	}
	if _, ok := venue["map_url"]; !ok {
		venue["map_url"] = fmt.Sprintf("https://maps.google.com/?q=%s", venueName)
	}

	// Live normalization
	var live map[string]interface{}
	if l, ok := cand["live"].(map[string]interface{}); ok {
		live = l
	} else {
		live = make(map[string]interface{})
	}
	liveStart, _ := live["start_time"].(string)
	if liveStart == "" {
		liveStart = "--:--"
		live["start_time"] = liveStart
	}
	if _, hasStage := live["stage"]; !hasStage {
		live["stage"] = nil
	}

	// Check if this matches an existing session
	existingID := ""
	candID, _ := cand["id"].(string)
	for _, ex := range existingSessions {
		exMap, ok := ex.(map[string]interface{})
		if !ok {
			continue
		}
		exID, _ := exMap["id"].(string)
		exDate, _ := exMap["date"].(string)
		exTitle, _ := exMap["title"].(string)

		if candID != "" && !strings.HasPrefix(candID, "candidate-") && exID == candID {
			existingID = exID
			break
		}
		if exDate == dateStr && exTitle == title {
			existingID = exID
			break
		}
	}

	action := "added"
	sessionID := existingID
	if sessionID != "" {
		action = "updated"
	} else {
		// Auto derive session ID: YYYYMMDD-N
		datePrefix := t.Format("20060102")
		count := 0
		for _, ex := range existingSessions {
			exMap, ok := ex.(map[string]interface{})
			if !ok {
				continue
			}
			if exDate, _ := exMap["date"].(string); exDate == dateStr {
				count++
			}
		}
		sessionID = fmt.Sprintf("%s-%d", datePrefix, count+1)
	}

	// Source Refs & Source Tweet URL
	var sourceRefs []interface{}
	if srefs, ok := cand["source_refs"].([]interface{}); ok && len(srefs) > 0 {
		sourceRefs = srefs
	}
	sourceTweetURL, _ := cand["source_tweet_url"].(string)
	if sourceTweetURL == "" && len(sourceRefs) > 0 {
		for _, sr := range sourceRefs {
			if srm, ok := sr.(map[string]interface{}); ok {
				if srm["type"] == "x_post" {
					if u, ok := srm["url"].(string); ok {
						sourceTweetURL = u
						break
					}
				}
			}
		}
	}
	if len(sourceRefs) == 0 && sourceTweetURL != "" {
		matches := statusIDRegex.FindStringSubmatch(sourceTweetURL)
		sid := ""
		if len(matches) > 1 {
			sid = matches[1]
		}
		sourceRefs = []interface{}{
			map[string]interface{}{
				"type":      "x_post",
				"source_id": sid,
				"url":       sourceTweetURL,
			},
		}
	}

	// Construct standardized session
	sess := map[string]interface{}{
		"id":               sessionID,
		"title":            title,
		"date":             dateStr,
		"day_of_week":      dayOfWeek,
		"venue":            venue,
		"live":             live,
		"source_tweet_url": sourceTweetURL,
		"source_refs":      sourceRefs,
	}

	// Carry over optional attributes safely
	if sessName, ok := cand["session_name"]; ok {
		sess["session_name"] = sessName
	}
	if ben, ok := cand["benefit"]; ok {
		sess["benefit"] = ben
	}
	if photo, ok := cand["photo"]; ok {
		sess["photo"] = photo
	}
	if notes, ok := cand["special_notes"]; ok {
		sess["special_notes"] = notes
	}
	if bonus, ok := cand["admission_bonus"]; ok {
		sess["admission_bonus"] = bonus
	}
	if ticket, ok := cand["ticket"]; ok && ticket != nil {
		sess["ticket"] = ticket
	} else {
		sess["ticket"] = map[string]interface{}{
			"platform": nil,
			"url":      nil,
			"prices":   []interface{}{},
		}
	}
	if imgURL, ok := cand["image_url"]; ok {
		sess["image_url"] = imgURL
	}
	if imgs, ok := cand["images"]; ok {
		sess["images"] = imgs
	} else {
		sess["images"] = []interface{}{}
	}

	summary := PromotedSessionSummary{
		ID:       sessionID,
		Date:     dateStr,
		Title:    title,
		Venue:    venueName,
		LiveTime: liveStart,
		Action:   action,
	}

	return sess, summary, nil
}
