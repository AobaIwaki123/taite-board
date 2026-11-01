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

var statusIDRegex = regexp.MustCompile(`status/(\d+)`)

type UpdateSessionOptions struct {
	SessionID         string
	PhotoRule         string
	PhotoRawText      string
	PhotoAvailable    *bool
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
	if opts.SessionID == "" {
		return fmt.Errorf("session ID is required")
	}
	if opts.FilePath == "" {
		opts.FilePath = "data/generated/schedule.json"
	}

	raw, err := os.ReadFile(opts.FilePath)
	if err != nil {
		return fmt.Errorf("reading schedule file: %w", err)
	}

	var doc rootDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("unmarshaling schedule json: %w", err)
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
			if opts.PhotoRule != "" {
				available := true
				if opts.PhotoAvailable != nil {
					available = *opts.PhotoAvailable
				}
				rawText := opts.PhotoRawText
				if rawText == "" {
					rawText = opts.PhotoRule
				}
				sMap["photo"] = map[string]interface{}{
					"available": available,
					"rule":      opts.PhotoRule,
					"raw_text":  rawText,
				}
			}

			// 2. Update Special Notes if specified
			if opts.SpecialNotes != "" {
				existing, _ := sMap["special_notes"].(string)
				if existing == "" {
					sMap["special_notes"] = opts.SpecialNotes
				} else if !strings.Contains(existing, opts.SpecialNotes) {
					sMap["special_notes"] = fmt.Sprintf("%s / %s", opts.SpecialNotes, existing)
				}
			}

			// 3. Update Source Tweet URL and Source Refs
			if opts.SourceTweetURL != "" {
				if opts.UpdateOfficialURL {
					sMap["source_tweet_url"] = opts.SourceTweetURL
				}

				matches := statusIDRegex.FindStringSubmatch(opts.SourceTweetURL)
				if len(matches) > 1 {
					sourceID := matches[1]
					refsRaw, _ := sMap["source_refs"].([]interface{})
					hasRef := false
					for _, r := range refsRaw {
						rMap, ok := r.(map[string]interface{})
						if !ok {
							continue
						}
						if rID, _ := rMap["source_id"].(string); rID == sourceID {
							hasRef = true
							break
						}
					}
					if !hasRef {
						newRef := map[string]interface{}{
							"type":      "x_post",
							"source_id": sourceID,
							"url":       opts.SourceTweetURL,
						}
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
				return fmt.Errorf("encoding updated session %s: %w", opts.SessionID, err)
			}
			doc.Sessions[i] = bytes.TrimSpace(sessBuf.Bytes())
			break
		}
	}

	if !found {
		return fmt.Errorf("session ID '%s' not found in %s", opts.SessionID, opts.FilePath)
	}

	// Update root generated_at
	doc.GeneratedAt = time.Now().UTC().Format("2006-01-02T15:04:05.000Z")

	// Serialize formatted JSON with SetEscapeHTML(false)
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return fmt.Errorf("marshaling updated schedule: %w", err)
	}

	if err := os.WriteFile(opts.FilePath, buf.Bytes(), 0644); err != nil {
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
