package validate

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
)

var (
	dateRegex = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	timeRegex = regexp.MustCompile(`^(\d{2}:\d{2}|--:--)$`)
)

type ValidationReport struct {
	TotalSessions int      `json:"total_sessions"`
	Errors        []string `json:"errors"`
	Warnings      []string `json:"warnings"`
	IsValid       bool     `json:"is_valid"`
}

// ValidateFile checks the given schedule JSON file against ADR-0001 rules.
func ValidateFile(filePath string) (*ValidationReport, error) {
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

		// Venue
		venue, _ := s["venue"].(map[string]interface{})
		if venue == nil || venue["name"] == "" {
			report.Errors = append(report.Errors, fmt.Sprintf("%s: missing venue.name", prefix))
		}

		// Live
		live, _ := s["live"].(map[string]interface{})
		if live == nil {
			report.Errors = append(report.Errors, fmt.Sprintf("%s: missing live object", prefix))
		} else {
			st, _ := live["start_time"].(string)
			if !timeRegex.MatchString(st) {
				report.Warnings = append(report.Warnings, fmt.Sprintf("%s: unusual live.start_time '%s'", prefix, st))
			}
		}

		// Ticket
		ticket, _ := s["ticket"].(map[string]interface{})
		if ticket == nil {
			report.Errors = append(report.Errors, fmt.Sprintf("%s: missing ticket object", prefix))
		}
	}

	if len(report.Errors) > 0 {
		report.IsValid = false
	}

	return report, nil
}
