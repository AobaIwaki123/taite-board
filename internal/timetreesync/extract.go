package timetreesync

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/AobaIwaki123/taite-board/internal/resolve"
	"github.com/AobaIwaki123/taite-board/internal/schedule"
	"github.com/AobaIwaki123/taite-board/internal/source/timetree"
)

var timeRange = regexp.MustCompile(`(\d{1,2}):(\d{2})\s*[-~〜～ー―]\s*(\d{1,2}):(\d{2})`)
var ticketLink = regexp.MustCompile(`(?i)(?:https?://)?(?:www\.)?(?:tiget\.net|t-dv\.com|ticketdive\.com|livepocket\.jp|eplus\.jp|ticketvillage\.jp|pia\.jp|l-tike\.com)/[^\s）)]+`)

// Extract only uses explicitly labelled performance information. Calendar
// startAt determines the date, never the performance start time.
func Extract(calendarID string, event timetree.Event) (schedule.AddSessionOptions, error) {
	var opts schedule.AddSessionOptions
	start, err := time.Parse(time.RFC3339, event.StartAt)
	if err != nil {
		return opts, fmt.Errorf("event %s: invalid startAt: %w", event.ID, err)
	}
	if event.ID == "" {
		return opts, fmt.Errorf("event ID is empty")
	}
	opts.Date = start.In(time.FixedZone("JST", 9*60*60)).Format("2006-01-02")
	opts.Title = strings.Trim(strings.TrimSpace(event.Title), "『』「」｢｣ ")
	firstLine := strings.SplitN(event.Description, "\n", 2)[0]
	labelText := resolve.NormalizeText(event.Title + " " + firstLine)
	firstPart, secondPart := strings.Contains(labelText, "1部"), strings.Contains(labelText, "2部")
	if firstPart && secondPart {
		return opts, fmt.Errorf("event %s: multiple session labels", event.ID)
	}
	if firstPart {
		opts.SessionName = "1部"
	} else if secondPart {
		opts.SessionName = "2部"
	}
	if first := firstLine; strings.Contains(first, "『") && strings.Contains(first, "』") {
		_, after, _ := strings.Cut(first, "『")
		title, _, _ := strings.Cut(after, "』")
		if strings.TrimSpace(title) != "" {
			opts.Title = strings.TrimSpace(title)
		}
	}
	if opts.Title == "" {
		opts.Title = "ライブ出演 (" + opts.Date + ")"
	}
	opts.VenueName = strings.TrimSpace(event.Location)
	if opts.VenueName == "" {
		opts.VenueName = "未定"
	}
	for _, raw := range strings.Split(event.Description, "\n") {
		line := strings.TrimSpace(raw)
		norm := resolve.NormalizeText(line)
		switch {
		case strings.HasPrefix(line, "📍"):
			if opts.VenueName == "未定" {
				opts.VenueName = strings.TrimSpace(strings.TrimLeft(strings.TrimPrefix(line, "📍"), " :："))
			}
		case strings.HasPrefix(line, "🎪") || strings.HasPrefix(norm, "会場:") || strings.HasPrefix(norm, "場所:"):
			if opts.VenueName == "未定" {
				_, value, ok := strings.Cut(norm, ":")
				if ok {
					opts.VenueName = strings.TrimSpace(value)
				}
			}
		}
		if strings.HasPrefix(line, "🎤") || strings.HasPrefix(norm, "出演") || strings.HasPrefix(norm, "出番") || strings.HasPrefix(strings.ToUpper(norm), "LIVE") {
			if a, b := explicitRange(norm); a != "" {
				opts.LiveStartTime, opts.LiveEndTime = a, b
			}
		}
		if strings.HasPrefix(line, "📸") || strings.HasPrefix(norm, "特典会") || strings.HasPrefix(norm, "物販") {
			if a, b := explicitRange(norm); a != "" {
				opts.BenefitStartTime, opts.BenefitEndTime = a, b
				loc := timeRange.FindStringIndex(norm)
				if loc != nil {
					opts.Booth = strings.Trim(strings.TrimSpace(norm[loc[1]:]), "（）() ")
				}
			}
		}
		if strings.HasPrefix(line, "🎁") {
			opts.AdmissionBonus = strings.TrimSpace(strings.TrimPrefix(line, "🎁"))
		}
	}
	if m := ticketLink.FindString(event.Description); m != "" {
		if !strings.Contains(m, "://") {
			m = "https://" + m
		}
		opts.TicketURL = m
	}
	if opts.TicketURL == "" && ticketLink.MatchString(event.URL) {
		opts.TicketURL = event.URL
	}
	opts.Title = cleanText(opts.Title)
	opts.AdmissionBonus = cleanText(opts.AdmissionBonus)
	if opts.Title == "" {
		opts.Title = "ライブ出演 (" + opts.Date + ")"
	}
	if opts.VenueName == "" {
		opts.VenueName = "未定"
	}
	// Event.url can be a ticket link, so construct a stable TimeTree source URL.
	opts.SourceTweetURL = fmt.Sprintf("https://timetr.ee/p/%s/%s", calendarID, event.ID)
	opts.PreserveExisting = true
	opts.RequireMatchForFallback = true
	return opts, nil
}

func cleanText(s string) string {
	return strings.TrimSpace(strings.Map(func(r rune) rune {
		if (r >= 0x1f300 && r <= 0x1faff) || (r >= 0x2600 && r <= 0x27bf) || unicode.In(r, unicode.Variation_Selector) {
			return -1
		}
		return r
	}, s))
}

func explicitRange(line string) (string, string) {
	m := timeRange.FindStringSubmatch(line)
	if len(m) != 5 {
		return "", ""
	}
	var h1, m1, h2, m2 int
	_, err := fmt.Sscanf(m[1]+":"+m[2]+" "+m[3]+":"+m[4], "%d:%d %d:%d", &h1, &m1, &h2, &m2)
	if err != nil || h1 > 29 || h2 > 29 || m1 > 59 || m2 > 59 || h2*60+m2 <= h1*60+m1 {
		return "", ""
	}
	return fmt.Sprintf("%02d:%02d", h1, m1), fmt.Sprintf("%02d:%02d", h2, m2)
}
