package schedule

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"
)

type sessionSortKey struct {
	Index     int
	Date      string
	LiveStart string
	ID        string
}

// SortSchedule reads the schedule file, sorts sessions chronologically (date, live.start_time, id),
// updates generated_at, writes formatted JSON, and syncs to syncTargets.
func SortSchedule(filePath string, syncTargets ...string) (int, error) {
	if filePath == "" {
		filePath = "data/generated/schedule.json"
	}

	raw, err := os.ReadFile(filePath)
	if err != nil {
		return 0, fmt.Errorf("reading schedule file: %w", err)
	}

	var doc rootDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return 0, fmt.Errorf("unmarshaling schedule json: %w", err)
	}

	keys := make([]sessionSortKey, len(doc.Sessions))
	for i, sRaw := range doc.Sessions {
		var s struct {
			ID   string `json:"id"`
			Date string `json:"date"`
			Live struct {
				StartTime string `json:"start_time"`
			} `json:"live"`
		}
		_ = json.Unmarshal(sRaw, &s)
		keys[i] = sessionSortKey{
			Index:     i,
			Date:      s.Date,
			LiveStart: s.Live.StartTime,
			ID:        s.ID,
		}
	}

	sort.SliceStable(keys, func(i, j int) bool {
		if keys[i].Date != keys[j].Date {
			return keys[i].Date < keys[j].Date
		}
		if keys[i].LiveStart != keys[j].LiveStart {
			// Pending '--:--' goes to the end of that day
			if keys[i].LiveStart == "--:--" {
				return false
			}
			if keys[j].LiveStart == "--:--" {
				return true
			}
			return keys[i].LiveStart < keys[j].LiveStart
		}
		return keys[i].ID < keys[j].ID
	})

	sortedSessions := make([]json.RawMessage, len(doc.Sessions))
	for newIdx, key := range keys {
		sortedSessions[newIdx] = doc.Sessions[key.Index]
	}
	doc.Sessions = sortedSessions

	// Update generated_at
	doc.GeneratedAt = time.Now().UTC().Format("2006-01-02T15:04:05.000Z")

	// Encode
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return 0, fmt.Errorf("marshaling sorted schedule: %w", err)
	}

	if err := os.WriteFile(filePath, buf.Bytes(), 0644); err != nil {
		return 0, fmt.Errorf("writing sorted schedule file: %w", err)
	}

	// Sync
	if len(syncTargets) > 0 {
		if _, err := Sync(filePath, syncTargets...); err != nil {
			return 0, fmt.Errorf("syncing sorted schedule: %w", err)
		}
	}

	return len(doc.Sessions), nil
}
