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

	// Stage 5: Deduplication Check via SHA-256 Raw Fingerprint
	if existingFingerprints != nil && existingFingerprints[fingerprint] {
		shortFP := fingerprint
		if len(shortFP) > 12 {
			shortFP = shortFP[:12]
		}
		return FilterResult{
			Passed:      false,
			Stage:       StageDuplicate,
			Reason:      fmt.Sprintf("duplicate raw fingerprint already processed (%s)", shortFP),
			Fingerprint: fingerprint,
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

// LoadExistingFingerprints scans a directory for existing candidate JSON files and indexes their fingerprints.
func LoadExistingFingerprints(dir string) (map[string]bool, error) {
	fps := make(map[string]bool)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fps, nil
	}

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".json") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil // Skip unreadable files
		}

		// Try parsing as IntakeResult
		var intake candidate.IntakeResult
		if err := json.Unmarshal(data, &intake); err == nil && intake.RawFingerprint != "" {
			fps[intake.RawFingerprint] = true
			return nil
		}

		// Try parsing generic JSON map for raw_fingerprint field
		var generic map[string]interface{}
		if err := json.Unmarshal(data, &generic); err == nil {
			if fp, ok := generic["raw_fingerprint"].(string); ok && fp != "" {
				fps[fp] = true
			}
		}
		return nil
	})

	return fps, err
}
