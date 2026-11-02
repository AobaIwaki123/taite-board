package filter

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AobaIwaki123/taite-board/internal/candidate"
	"github.com/AobaIwaki123/taite-board/internal/raw"
)

// Stage indicates which validation stage rejected or evaluated the post.
type Stage string

const (
	StageMedia     Stage = "media"
	StageAge       Stage = "age"
	StageExclude   Stage = "exclude_keyword"
	StageTarget    Stage = "target_keyword"
	StageDuplicate Stage = "duplicate_fingerprint"
)

// FilterRules specifies the criteria for the multi-stage local pre-filter (ADR-0018).
type FilterRules struct {
	RequireMedia    bool     `json:"require_media"`
	TargetKeywords  []string `json:"target_keywords"`
	ExcludeKeywords []string `json:"exclude_keywords"`
	MaxAgeHours     int      `json:"max_age_hours"`
}

// DefaultRules defines standard production heuristics for idol timetable announcement tweets.
var DefaultRules = FilterRules{
	RequireMedia: true,
	TargetKeywords: []string{
		"リボンクロック", "リボクロ",
		"タイムテーブル", "タイテ", "出演", "特典会", "ライブ", "フェス", "対バン", "単独",
	},
	ExcludeKeywords: []string{
		"配信", "チェキチャ", "オンラインサイン会", "通販", "SHOWROOM", "スペース", "おやすみ", "おはよう", "ありがとうございま",
	},
	MaxAgeHours: 168, // Within 7 days
}

// FilterResult represents the evaluation decision.
type FilterResult struct {
	Passed      bool   `json:"passed"`
	Stage       Stage  `json:"stage,omitempty"`
	Reason      string `json:"reason"`
	Fingerprint string `json:"fingerprint,omitempty"`
}

// Evaluate applies the 3-stage local pre-filter rules against a raw post snapshot.
func Evaluate(post *raw.Post, rules FilterRules, existingFingerprints map[string]bool, now time.Time) FilterResult {
	if post == nil {
		return FilterResult{
			Passed: false,
			Stage:  StageMedia,
			Reason: "nil post snapshot provided",
		}
	}

	fingerprint := CalculateRawFingerprint(post)

	// Stage 1: Media Presence Check
	if rules.RequireMedia && len(post.Images) == 0 {
		return FilterResult{
			Passed:      false,
			Stage:       StageMedia,
			Reason:      "no flyer or timetable images attached",
			Fingerprint: fingerprint,
		}
	}

	// Stage 2: Post Age Verification
	if rules.MaxAgeHours > 0 && post.FetchedAt != "" {
		fetchedTime, err := time.Parse(time.RFC3339, post.FetchedAt)
		if err == nil && now.Sub(fetchedTime) > time.Duration(rules.MaxAgeHours)*time.Hour {
			return FilterResult{
				Passed:      false,
				Stage:       StageAge,
				Reason:      fmt.Sprintf("post age (%s) exceeds maximum allowed age limit (%dh)", post.FetchedAt, rules.MaxAgeHours),
				Fingerprint: fingerprint,
			}
		}
	}

	// Stage 3: Negative Keywords (Spam / Everyday Chatter / Streaming Exclusion)
	for _, kw := range rules.ExcludeKeywords {
		if kw != "" && strings.Contains(post.Text, kw) {
			return FilterResult{
				Passed:      false,
				Stage:       StageExclude,
				Reason:      fmt.Sprintf("contains excluded keyword %q", kw),
				Fingerprint: fingerprint,
			}
		}
	}

	// Stage 4: Positive Keywords (Timetable / Live Relevancy)
	if len(rules.TargetKeywords) > 0 {
		hasTarget := false
		for _, kw := range rules.TargetKeywords {
			if kw != "" && strings.Contains(post.Text, kw) {
				hasTarget = true
				break
			}
		}
		if !hasTarget {
			return FilterResult{
				Passed:      false,
				Stage:       StageTarget,
				Reason:      "does not contain any target live or timetable keywords",
				Fingerprint: fingerprint,
			}
		}
	}

	// Stage 5: Deduplication Check via SHA-256 Raw Fingerprint or Source ID
	if existingFingerprints != nil {
		if existingFingerprints[fingerprint] || (post.SourceID != "" && existingFingerprints[post.SourceID]) {
			shortFP := fingerprint
			if len(shortFP) > 12 {
				shortFP = shortFP[:12]
			}
			return FilterResult{
				Passed:      false,
				Stage:       StageDuplicate,
				Reason:      fmt.Sprintf("duplicate raw fingerprint or post ID already processed (%s)", shortFP),
				Fingerprint: fingerprint,
			}
		}
	}

	return FilterResult{
		Passed:      true,
		Reason:      "all multi-stage filter criteria passed",
		Fingerprint: fingerprint,
	}
}

// CalculateRawFingerprint calculates the canonical SHA-256 hash for raw post idempotency.
func CalculateRawFingerprint(post *raw.Post) string {
	if post == nil {
		return ""
	}
	return candidate.ComputeRawFingerprint(post.Source, post.SourceID, post.Text, post.Images)
}

// CalculateParseCacheKey calculates the cache key for Cloud Run AI parser caching.
func CalculateParseCacheKey(post *raw.Post, schemaVer int, parserVer, modelID, promptVer string) string {
	fp := CalculateRawFingerprint(post)
	return candidate.ComputeParseCacheKey(fp, schemaVer, parserVer, modelID, promptVer)
}

// Checkpoint tracks the cursor of the latest processed post ID and indexed processed history.
type Checkpoint struct {
	LatestSinceID string   `json:"latest_since_id"`
	UpdatedAt     string   `json:"updated_at"`
	ProcessedIDs  []string `json:"processed_ids,omitempty"`
}

// LoadCheckpoint reads the checkpoint file from disk if it exists.
func LoadCheckpoint(path string) (*Checkpoint, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var cp Checkpoint
	if err := json.Unmarshal(data, &cp); err != nil {
		return nil, err
	}
	return &cp, nil
}

// SaveCheckpoint writes the checkpoint atomically to disk.
func SaveCheckpoint(path string, cp *Checkpoint) error {
	if path == "" || cp == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	cp.UpdatedAt = time.Now().Format(time.RFC3339)
	data, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		return err
	}
	tmpFile := fmt.Sprintf("%s.tmp.%d", path, time.Now().UnixNano())
	if err := os.WriteFile(tmpFile, append(data, '\n'), 0644); err != nil {
		return err
	}
	return os.Rename(tmpFile, path)
}

// LoadKnownDeduplicationKeys loads known fingerprints and post IDs across candidate dir, schedule.json, and checkpoint.
func LoadKnownDeduplicationKeys(candDir, schedulePath, checkpointPath string) (map[string]bool, error) {
	keys := make(map[string]bool)

	// 1. Scan candidate directory
	if candDir != "" {
		if _, err := os.Stat(candDir); err == nil {
			_ = filepath.WalkDir(candDir, func(path string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".json") {
					return nil
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return nil
				}
				var intake candidate.IntakeResult
				if err := json.Unmarshal(data, &intake); err == nil {
					if intake.RawFingerprint != "" {
						keys[intake.RawFingerprint] = true
					}
					if intake.SourceID != "" {
						keys[intake.SourceID] = true
					}
				}
				return nil
			})
		}
	}

	// 2. Scan schedule.json (Master schedule)
	if schedulePath != "" {
		if data, err := os.ReadFile(schedulePath); err == nil {
			var doc struct {
				Sessions []struct {
					SourceTweetURL string `json:"source_tweet_url"`
					SourceRefs     []struct {
						SourceID string `json:"source_id"`
					} `json:"source_refs"`
				} `json:"sessions"`
			}
			if err := json.Unmarshal(data, &doc); err == nil {
				for _, s := range doc.Sessions {
					for _, ref := range s.SourceRefs {
						if ref.SourceID != "" {
							keys[ref.SourceID] = true
						}
					}
					if s.SourceTweetURL != "" {
						// Extract numeric status ID from URL
						parts := strings.Split(strings.TrimRight(s.SourceTweetURL, "/"), "/")
						if len(parts) > 0 {
							last := parts[len(parts)-1]
							if len(last) > 8 && strings.Trim(last, "0123456789") == "" {
								keys[last] = true
							}
						}
					}
				}
			}
		}
	}

	// 3. Scan checkpoint file
	if checkpointPath != "" {
		if cp, err := LoadCheckpoint(checkpointPath); err == nil && cp != nil {
			if cp.LatestSinceID != "" {
				keys[cp.LatestSinceID] = true
			}
			for _, id := range cp.ProcessedIDs {
				if id != "" {
					keys[id] = true
				}
			}
		}
	}

	return keys, nil
}

// LoadExistingFingerprints scans a directory for existing candidate JSON files and indexes their fingerprints.
func LoadExistingFingerprints(dir string) (map[string]bool, error) {
	schedulePath := "data/generated/schedule.json"
	checkpointPath := "data/checkpoint.json"
	if _, err := os.Stat("/var/log/ribonclock/checkpoint.json"); err == nil {
		checkpointPath = "/var/log/ribonclock/checkpoint.json"
	}
	return LoadKnownDeduplicationKeys(dir, schedulePath, checkpointPath)
}
