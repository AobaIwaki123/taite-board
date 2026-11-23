package schedule

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

var (
	statusIDRegex        = regexp.MustCompile(`status/(\d+)`)
	timetreeEventIDRegex = regexp.MustCompile(`(?:timetreeapp\.com/public_calendars/[^/]+/events|timetr\.ee/p/[^/]+)/([^/?#]+)`)
)

func buildSourceRefFromURL(rawURL string) (ref map[string]interface{}, isXPost bool) {
	u := strings.TrimSpace(rawURL)
	if u == "" {
		return nil, false
	}
	if matches := statusIDRegex.FindStringSubmatch(u); len(matches) > 1 {
		return map[string]interface{}{
			"type":      "x_post",
			"source_id": matches[1],
			"url":       u,
		}, true
	}
	if matches := timetreeEventIDRegex.FindStringSubmatch(u); len(matches) > 1 {
		return map[string]interface{}{
			"type":      "timetree_event",
			"source_id": matches[1],
			"url":       u,
		}, false
	}
	return map[string]interface{}{
		"type": "web",
		"url":  u,
	}, false
}

type UpdateSessionOptions struct {
	SessionID         string
	PhotoRule         string
	PhotoRawText      string
	PhotoAvailable    *bool
	PhotoSong         string
	PhotoDevice       string
	AdmissionBonus    string
	SourceTweetURL    string
	UpdateOfficialURL bool
	SpecialNotes      string
	FilePath          string
	SyncTargets       []string
}

type rootDoc struct {
	TargetGroup    json.RawMessage   `json:"target_group,omitempty"`
	FeaturedMember json.RawMessage   `json:"featured_member,omitempty"`
	Promotions     json.RawMessage   `json:"promotions,omitempty"`
	CurrentNextID  string            `json:"current_next_id,omitempty"`
	Sessions       []json.RawMessage `json:"sessions"`
	SchemaVersion  int               `json:"schema_version"`
	GeneratedAt    string            `json:"generated_at"`
	Timezone       string            `json:"timezone"`
}

// UpdateSession updates a session's photo rules, special notes, and source tweet URL,
// updates the root generated_at timestamp, writes back formatted JSON, and syncs copies.
// It preserves unedited sessions as raw bytes to prevent unnecessary key reordering diffs.
func UpdateSession(opts UpdateSessionOptions) error {
	if opts.FilePath == "" {
		opts.FilePath = "data/generated/schedule.json"
	}

	raw, err := os.ReadFile(opts.FilePath)
	if err != nil {
		return fmt.Errorf("reading schedule file: %w", err)
	}

	updatedBytes, err := ApplyUpdateSession(raw, opts)
	if err != nil {
		return err
	}

	if err := os.WriteFile(opts.FilePath, updatedBytes, 0644); err != nil {
		return fmt.Errorf("writing updated schedule file: %w", err)
	}

	// Sync to targets if configured
	if len(opts.SyncTargets) > 0 {
		if _, err := Sync(opts.FilePath, opts.SyncTargets...); err != nil {
			return fmt.Errorf("syncing after update: %w", err)
		}
	}

	return nil
}

// ApplyUpdateSession updates a session inside schedule JSON bytes and returns formatted JSON bytes.
func ApplyUpdateSession(raw []byte, opts UpdateSessionOptions) ([]byte, error) {
	if opts.SessionID == "" {
		return nil, fmt.Errorf("session ID is required")
	}

	var doc rootDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("unmarshaling schedule json: %w", err)
	}
	if doc.SchemaVersion < 1 {
		doc.SchemaVersion = 1
	}
	if doc.Timezone == "" {
		doc.Timezone = "Asia/Tokyo"
	}

	found := false
	for i, sessRaw := range doc.Sessions {
		var sMap map[string]interface{}
		if err := json.Unmarshal(sessRaw, &sMap); err != nil {
			continue
		}
		if sID, _ := sMap["id"].(string); sID == opts.SessionID {
			found = true

			// 1. Update Photo Rule if specified
			if opts.PhotoRule != "" || opts.PhotoSong != "" || opts.PhotoAvailable != nil {
				available := true
				if opts.PhotoAvailable != nil {
					available = *opts.PhotoAvailable
				}
				rawText := opts.PhotoRawText
				if rawText == "" {
					rawText = opts.PhotoRule
				}
				if rawText == "" && opts.PhotoSong != "" {
					rawText = fmt.Sprintf("撮可: %s", opts.PhotoSong)
				}
				photoMap := map[string]interface{}{
					"available": available,
					"rule":      opts.PhotoRule,
					"raw_text":  rawText,
				}
				if opts.PhotoSong != "" {
					photoMap["allowed_songs"] = []string{opts.PhotoSong}
				}
				if opts.PhotoDevice != "" {
					photoMap["device"] = opts.PhotoDevice
				}
				sMap["photo"] = photoMap
			}

			// 2. Update Admission Bonus if specified
			if opts.AdmissionBonus != "" {
				sMap["admission_bonus"] = opts.AdmissionBonus
			}

			// 3. Update Special Notes if specified
			if opts.SpecialNotes != "" {
				existing, _ := sMap["special_notes"].(string)
				if existing == "" {
					sMap["special_notes"] = opts.SpecialNotes
				} else if !strings.Contains(existing, opts.SpecialNotes) {
					sMap["special_notes"] = fmt.Sprintf("%s / %s", opts.SpecialNotes, existing)
				}
			}

			// 4. Update Source Tweet URL and Source Refs
			if opts.SourceTweetURL != "" {
				newRef, isX := buildSourceRefFromURL(opts.SourceTweetURL)
				if opts.UpdateOfficialURL && isX {
					sMap["source_tweet_url"] = opts.SourceTweetURL
				}

				if newRef != nil {
					refsRaw, _ := sMap["source_refs"].([]interface{})
					newID, _ := newRef["source_id"].(string)
					newURL, _ := newRef["url"].(string)
					hasRef := false
					for _, r := range refsRaw {
						rMap, ok := r.(map[string]interface{})
						if !ok {
							continue
						}
						rID, _ := rMap["source_id"].(string)
						rURL, _ := rMap["url"].(string)
						if (newID != "" && rID == newID) || (newURL != "" && rURL == newURL) {
							hasRef = true
							break
						}
					}
					if !hasRef {
						sMap["source_refs"] = append(refsRaw, newRef)
					}
				}
			}

			// Re-encode only this modified session
			sessBuf := &bytes.Buffer{}
			sessEnc := json.NewEncoder(sessBuf)
			sessEnc.SetEscapeHTML(false)
			sessEnc.SetIndent("      ", "  ")
			if err := sessEnc.Encode(sMap); err != nil {
				return nil, fmt.Errorf("encoding updated session %s: %w", opts.SessionID, err)
			}
			doc.Sessions[i] = bytes.TrimSpace(sessBuf.Bytes())
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("session ID '%s' not found", opts.SessionID)
	}

	// Update root generated_at
	doc.GeneratedAt = time.Now().UTC().Format("2006-01-02T15:04:05.000Z")

	// Serialize formatted JSON with SetEscapeHTML(false)
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return nil, fmt.Errorf("marshaling updated schedule: %w", err)
	}

	return buf.Bytes(), nil
}
