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

	admissionBonusRegex = regexp.MustCompile(`(?i)(?:入場特典|お目当て特典|指名入場特典|動員特典|来場特典|🎁|🎀入場特典決定🎀)\s*[:：]?\s*([^\n\r]+)`)
	photoSongRegex      = regexp.MustCompile(`(?i)(?:(?:撮影可能曲|撮可曲|撮影曲|撮可)\s*[:：]?\s*[【『「｢]([^】』」｣\n\r]+)[】』」｣])|(?:[【『「｢]([^】』」｣\n\r]+)[】』」｣]\s*(?:のみ)?\s*(?:動画|静止画|写真)?\s*(?:の)?\s*(?:撮影可能|撮影OK|撮可))`)
	photoGeneralRegex   = regexp.MustCompile(`(?i)(?:全曲)?(?:動画|静止画|写真)(?:の)?(?:撮影可能|撮影OK|撮可)`)
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

	// 6. Extract Admission Bonus & Photo Regulations (Patch Detection)
	if m := admissionBonusRegex.FindStringSubmatch(text); len(m) > 1 {
		bonus := strings.TrimSpace(m[1])
		bonus = strings.TrimPrefix(bonus, "🎁")
		bonus = strings.TrimSpace(bonus)
		if bonus != "" {
			cand.AdmissionBonus = bonus
			evidences = append(evidences, candidate.Evidence{
				Field:      "admission_bonus",
				Source:     "text",
				Quote:      m[0],
				Confidence: 0.9,
			})
		}
	}

	// Photo song extraction (e.g. 『シャイニーデイズ』のみ動画撮影可能 / 撮影可能曲：【未来へのファンファーレ】動画撮影OK)
	if m := photoSongRegex.FindStringSubmatch(text); len(m) > 1 {
		song := m[1]
		if song == "" && len(m) > 2 {
			song = m[2]
		}
		cand.PhotoSong = strings.TrimSpace(song)
		allowed := true
		cand.PhotoAllowed = &allowed

		matchLine := m[0]
		for _, line := range strings.Split(text, "\n") {
			if strings.Contains(line, m[0]) || (cand.PhotoSong != "" && strings.Contains(line, cand.PhotoSong)) {
				matchLine = line
				break
			}
		}

		if strings.Contains(matchLine, "動画") {
			cand.PhotoDevice = "video_ok"
		} else {
			cand.PhotoDevice = "photo_ok"
		}
		evidences = append(evidences, candidate.Evidence{
			Field:      "photo_song",
			Source:     "text",
			Quote:      strings.TrimSpace(matchLine),
			Confidence: 0.95,
		})
	} else if m := photoGeneralRegex.FindStringSubmatch(text); len(m) > 0 {
		allowed := true
		cand.PhotoAllowed = &allowed
		if strings.Contains(m[0], "全曲") {
			cand.PhotoSong = "全曲"
		}
		if strings.Contains(m[0], "動画") {
			cand.PhotoDevice = "video_ok"
		} else {
			cand.PhotoDevice = "photo_ok"
		}
		evidences = append(evidences, candidate.Evidence{
			Field:      "photo_allowed",
			Source:     "text",
			Quote:      m[0],
			Confidence: 0.9,
		})
	}

	// If no live start time but perk or photo info is present, mark as patch
	if cand.LiveStartRaw == "" && (cand.AdmissionBonus != "" || cand.PhotoSong != "" || cand.PhotoAllowed != nil) {
		cand.IsPatch = true
	}

	// 7. Evaluate NeedsAI
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

// ResolveVenueName resolves a raw venue string against canonical entities.
func (r *Resolver) ResolveVenueName(venueName string) (canonicalID, displayName string) {
	if venueName == "" {
		return "", ""
	}
	norm := NormalizeText(venueName)
	for _, v := range r.entities.Entities.Venues {
		if NormalizeText(v.Name) == norm {
			return v.CanonicalID, v.Name
		}
		for _, alias := range v.Aliases {
			if strings.EqualFold(NormalizeText(alias), norm) || strings.Contains(norm, NormalizeText(alias)) {
				return v.CanonicalID, v.Name
			}
		}
	}
	slug := strings.ToLower(strings.ReplaceAll(venueName, " ", "-"))
	return fmt.Sprintf("venue:%s", slug), venueName
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
