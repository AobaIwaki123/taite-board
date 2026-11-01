package resolve

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/AobaIwaki123/taite-board/internal/candidate"
)

//go:embed entities_default.json
var defaultEntitiesJSON []byte

type EntitiesData struct {
	Entities struct {
		Groups []struct {
			CanonicalID string   `json:"canonical_id"`
			Name        string   `json:"name"`
			ShortName   string   `json:"short_name"`
			Aliases     []string `json:"aliases"`
		} `json:"groups"`
		Venues []struct {
			CanonicalID string   `json:"canonical_id"`
			Name        string   `json:"name"`
			Area        string   `json:"area"`
			Aliases     []string `json:"aliases"`
		} `json:"venues"`
	} `json:"entities"`
}

// Resolver resolves post content into structured event candidates using deterministic rules.
type Resolver struct {
	entities EntitiesData
}

// NewResolver initializes a resolver with curated entities.
func NewResolver(entitiesPath string) (*Resolver, error) {
	r := &Resolver{}
	// First load embedded defaults
	if len(defaultEntitiesJSON) > 0 {
		_ = json.Unmarshal(defaultEntitiesJSON, &r.entities)
	}

	// Override with external file if provided and readable
	if entitiesPath != "" {
		if data, err := os.ReadFile(entitiesPath); err == nil {
			_ = json.Unmarshal(data, &r.entities)
		}
	}

	return r, nil
}

var (
	fullYearDateRegex = regexp.MustCompile(`(20\d\d)[年/.-](\d{1,2})[月/.-](\d{1,2})`)
	monthDayDateRegex = regexp.MustCompile(`(\d{1,2})[月/](\d{1,2})日?`)

	liveTimeRegex    = regexp.MustCompile(`(?i)(?:LIVE|出演|出番|本番|ライブ)\s*[:：]?\s*(\d{1,2}:\d{2})\s*[-~〜]\s*(\d{1,2}:\d{2})`)
	benefitTimeRegex = regexp.MustCompile(`(?i)(?:特典会|物販|終演後物販|並行物販)\s*[:：]?\s*(\d{1,2}:\d{2})\s*[-~〜]\s*(\d{1,2}:\d{2})`)
	venueFallbackRegex = regexp.MustCompile(`(?i)(?:会場|場所)\s*[:：]?\s*([^\s\n\r]+)`)
	genericTimeRegex = regexp.MustCompile(`(\d{1,2}:\d{2})\s*[-~〜]\s*(\d{1,2}:\d{2})`)
)

// NormalizeText standardizes full-width digits, punctuation, and whitespace into standard ASCII.
func NormalizeText(s string) string {
	var sb strings.Builder
	for _, r := range s {
		switch {
		case r >= '０' && r <= '９':
			sb.WriteRune(r - '０' + '0')
		case r == '：':
			sb.WriteRune(':')
		case r == '〜' || r == '～' || r == 'ー' || r == '―':
			sb.WriteRune('-')
		case r == '　':
			sb.WriteRune(' ')
		default:
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

// Resolve analyzes post text and image metadata, returning an EventCandidate and evidence list.
func (r *Resolver) Resolve(text string, images []string) (candidate.EventCandidate, []candidate.Evidence, bool) {
	norm := NormalizeText(text)
	var cand candidate.EventCandidate
	var evidences []candidate.Evidence

	// 1. Resolve Group
	for _, g := range r.entities.Entities.Groups {
		matched := false
		for _, alias := range g.Aliases {
			if strings.Contains(norm, NormalizeText(alias)) {
				matched = true
				break
			}
		}
		if matched {
			cand.GroupRaw = g.Name
			cand.GroupCanonicalID = g.CanonicalID
			evidences = append(evidences, candidate.Evidence{
				Field:      "group_canonical_id",
				Source:     "text",
				Quote:      g.Name,
				Confidence: 1.0,
			})
			break
		}
	}
	// Default to Ribbon Clock if only one group configured
	if cand.GroupCanonicalID == "" && len(r.entities.Entities.Groups) > 0 {
		cand.GroupRaw = r.entities.Entities.Groups[0].Name
		cand.GroupCanonicalID = r.entities.Entities.Groups[0].CanonicalID
	}

	// 2. Resolve Venue
	for _, v := range r.entities.Entities.Venues {
		matched := false
		for _, alias := range v.Aliases {
			if strings.Contains(norm, NormalizeText(alias)) {
				matched = true
				break
			}
		}
		if matched {
			cand.VenueRaw = v.Name
			cand.VenueCanonicalID = v.CanonicalID
			evidences = append(evidences, candidate.Evidence{
				Field:      "venue_canonical_id",
				Source:     "text",
				Quote:      v.Name,
				Confidence: 1.0,
			})
			break
		}
	}
	if cand.VenueRaw == "" {
		if m := venueFallbackRegex.FindStringSubmatch(norm); len(m) == 2 {
			cand.VenueRaw = strings.TrimSpace(m[1])
			evidences = append(evidences, candidate.Evidence{
				Field:      "venue_raw",
				Source:     "text",
				Quote:      m[0],
				Confidence: 0.8,
			})
		}
	}

	// 3. Resolve Date
	if m := fullYearDateRegex.FindStringSubmatch(norm); len(m) == 4 {
		year := m[1]
		month, _ := strconv.Atoi(m[2])
		day, _ := strconv.Atoi(m[3])
		cand.Date = fmt.Sprintf("%s-%02d-%02d", year, month, day)
		evidences = append(evidences, candidate.Evidence{
			Field:      "date",
			Source:     "text",
			Quote:      m[0],
			Confidence: 1.0,
		})
	} else if m := monthDayDateRegex.FindStringSubmatch(norm); len(m) == 3 {
		// Default to year 2026
		month, _ := strconv.Atoi(m[1])
		day, _ := strconv.Atoi(m[2])
		cand.Date = fmt.Sprintf("2026-%02d-%02d", month, day)
		evidences = append(evidences, candidate.Evidence{
			Field:      "date",
			Source:     "text",
			Quote:      m[0],
			Confidence: 0.9,
		})
	}

	// 4. Resolve Live Time
	if m := liveTimeRegex.FindStringSubmatch(norm); len(m) == 3 {
		cand.LiveStartRaw = padTime(m[1])
		cand.LiveEndRaw = padTime(m[2])
		evidences = append(evidences, candidate.Evidence{
			Field:      "live_start_raw",
			Source:     "text",
			Quote:      m[0],
			Confidence: 0.95,
		})
	}

	// 5. Resolve Benefit Time
	if m := benefitTimeRegex.FindStringSubmatch(norm); len(m) == 3 {
		cand.BenefitStartRaw = padTime(m[1])
		cand.BenefitEndRaw = padTime(m[2])
		evidences = append(evidences, candidate.Evidence{
			Field:      "benefit_start_raw",
			Source:     "text",
			Quote:      m[0],
			Confidence: 0.95,
		})
	}

	// If no liveTime found yet, check genericTimeRegex if not benefit
	if cand.LiveStartRaw == "" {
		if m := genericTimeRegex.FindStringSubmatch(norm); len(m) == 3 {
			// Ensure it's not the benefit time we just found
			if padTime(m[1]) != cand.BenefitStartRaw {
				cand.LiveStartRaw = padTime(m[1])
				cand.LiveEndRaw = padTime(m[2])
				evidences = append(evidences, candidate.Evidence{
					Field:      "live_start_raw",
					Source:     "text",
					Quote:      m[0],
					Confidence: 0.8,
				})
			}
		}
	}

	// 6. Evaluate NeedsAI
	// If timetable image exists and live start time is missing or incomplete:
	needsAI := false
	if len(images) > 0 {
		if cand.LiveStartRaw == "" || cand.LiveEndRaw == "" {
			needsAI = true
		}
	}

	return cand, evidences, needsAI
}

func padTime(t string) string {
	parts := strings.Split(t, ":")
	if len(parts) != 2 {
		return t
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil {
		return t
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil {
		return t
	}
	return fmt.Sprintf("%02d:%02d", h, m)
}

// CleanWhitespace trims extra spacing and preserves newlines.
func CleanWhitespace(s string) string {
	lines := strings.Split(s, "\n")
	var cleaned []string
	for _, l := range lines {
		trimmed := strings.TrimRightFunc(l, unicode.IsSpace)
		cleaned = append(cleaned, trimmed)
	}
	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}
