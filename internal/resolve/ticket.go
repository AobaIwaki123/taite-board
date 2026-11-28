package resolve

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/AobaIwaki123/taite-board/internal/candidate"
)

var (
	ticketSectionStartRx = regexp.MustCompile(`(?i)(?:🎫|🎟|💰|💵|チケット|TICKET|料金|入場料|PRICE|ENTRY)`)
	ticketSectionStopRx  = regexp.MustCompile(`(?i)^(?:🎤|📸|🎁|📍|🎪|⏰|🎬|📹|出演|出番|特典会|物販|入場特典|お目当て特典|動員特典|来場特典|会場|場所|OPEN|START|撮影)`)
	ticketSkipLineRx     = regexp.MustCompile(`(?i)(?:🎁|入場特典|お目当て特典|動員特典|来場特典|チェキ券|ランチェキ|物販|グッズ|写メ券|動画券|ポイント|相当)`)
	ticketHeaderStripRx  = regexp.MustCompile(`(?i)^(?:[🎫🎟💰💵■◆▼●★☆・※\s]+|【\s*(?:チケット(?:料金|代|情報)?|料金|入場料|TICKET|PRICE)\s*】\s*|(?:チケット(?:料金|代|情報)?|料金|入場料|TICKET|PRICE)\s*[:：]\s*)+`)
	ticketCurrencyRx     = regexp.MustCompile(`(?:¥\s*\d|\d[\d,]*\s*円)`)
	ticketTierHintRx     = regexp.MustCompile(`(?:前方|前売|一般|当日|優先|VIP|SS|S席|A席|女性|学生|カメコ|撮影|観覧|入場).*無料`)
	priceTokenRx         = regexp.MustCompile(`(?:\+\s*)?(?:¥\s*\d{1,3}(?:,\d{3})+|¥\s*\d+|\d{1,3}(?:,\d{3})+\s*円|\d+\s*円|(?:入場|観覧|チケット)?無料)`)
	drinkSuffixRx        = regexp.MustCompile(`^\s*(?:\([^)]*(?:1D|ドリンク|D代|別途)[^)]*\)|\+?\s*1D(?:代)?(?:別|込)?)`)
	trailingTierParenRx  = regexp.MustCompile(`^\s*\(([^()]{1,20})\)`)
	drinkOnlyTierRx      = regexp.MustCompile(`(?i)(?:1D|ドリンク|D代|別途|手数料|送料)`)
	ticketTierKeywordRx  = regexp.MustCompile(`(?i)(?:前売|前方|一般|当日|優先|VIP|SS|S席|A席|B席|女性|学生|通常|入場|観覧|チケット|FREE|フリー)`)
)

func normalizeTicketLine(s string) string {
	norm := NormalizeText(s)
	return strings.NewReplacer(
		"￥", "¥",
		"／", "/",
		"（", "(",
		"）", ")",
		"＋", "+",
		"，", ",",
	).Replace(norm)
}

// ExtractTicketPrices extracts deterministic tier/price entries from post or TimeTree text.
func ExtractTicketPrices(text string) []candidate.TicketPrice {
	var results []candidate.TicketPrice
	seen := make(map[string]bool)
	inSection := false

	for _, rawLine := range strings.Split(text, "\n") {
		line := strings.TrimSpace(normalizeTicketLine(rawLine))
		if line == "" {
			inSection = false
			continue
		}
		if strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://") {
			inSection = false
			continue
		}
		if ticketSectionStopRx.MatchString(line) && !ticketSectionStartRx.MatchString(line) {
			inSection = false
			continue
		}
		if ticketSkipLineRx.MatchString(line) {
			if ticketSectionStopRx.MatchString(line) {
				inSection = false
			}
			continue
		}
		if ticketSectionStartRx.MatchString(line) {
			inSection = true
		}
		if !inSection && !ticketCurrencyRx.MatchString(line) && !ticketTierHintRx.MatchString(line) {
			continue
		}

		work := strings.TrimSpace(ticketHeaderStripRx.ReplaceAllString(line, ""))
		if work == "" {
			continue
		}

		matches := priceTokenRx.FindAllStringIndex(work, -1)
		if len(matches) == 0 {
			continue
		}

		cursor := 0
		for _, m := range matches {
			start, end := m[0], m[1]
			if start < cursor {
				continue
			}

			token := strings.TrimSpace(work[start:end])
			// Skip approximate bonus value mentions like "¥1,500相当"
			afterToken := work[end:]
			if strings.HasPrefix(strings.TrimSpace(afterToken), "相当") {
				cursor = end
				continue
			}

			rawBefore := work[cursor:start]
			tier := cleanTicketTierLabel(rawBefore)

			if drinkOnlyTierRx.MatchString(tier) && !ticketTierKeywordRx.MatchString(tier) {
				cursor = end
				continue
			}

			suffix := ""
			consumedEnd := end
			if ds := drinkSuffixRx.FindStringIndex(work[end:]); ds != nil {
				suffix = strings.TrimSpace(work[end+ds[0] : end+ds[1]])
				consumedEnd = end + ds[1]
			} else if tier == "" {
				if tp := trailingTierParenRx.FindStringSubmatchIndex(work[end:]); tp != nil {
					candTier := cleanTicketTierLabel(work[end+tp[2] : end+tp[3]])
					if candTier != "" && !drinkOnlyTierRx.MatchString(candTier) {
						tier = candTier
						consumedEnd = end + tp[1]
					}
				}
			}

			amount, formattedPrice, ok := parsePriceToken(token)
			if !ok {
				cursor = consumedEnd
				continue
			}
			if suffix != "" {
				formattedPrice = formattedPrice + " " + suffix
			}
			if tier == "" {
				tier = inferDefaultTier(token)
			}

			key := tier + "|" + formattedPrice
			if !seen[key] {
				seen[key] = true
				results = append(results, candidate.TicketPrice{
					Tier:   tier,
					Price:  formattedPrice,
					Amount: amount,
				})
			}
			cursor = consumedEnd
		}
	}

	return results
}

func cleanTicketTierLabel(raw string) string {
	s := strings.TrimSpace(ticketHeaderStripRx.ReplaceAllString(strings.TrimSpace(raw), ""))
	s = strings.Trim(s, "/・|｜,、:：- \t")
	// Strip leading standalone punctuation segments like "各"
	if s == "各" || s == "各チケット" {
		return ""
	}
	// If multiple slash-delimited fragments exist before the price, keep the last non-empty tier segment
	if idx := strings.LastIndexAny(s, "/・|｜"); idx >= 0 {
		tail := strings.TrimSpace(s[idx+1:])
		if tail != "" {
			s = tail
		}
	}
	s = strings.TrimSpace(s)
	for _, pair := range [][2]string{{"【", "】"}, {"[", "]"}, {"「", "」"}, {"『", "』"}, {"(", ")"}} {
		if strings.HasPrefix(s, pair[0]) && strings.HasSuffix(s, pair[1]) {
			inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(s, pair[0]), pair[1]))
			if inner != "" {
				s = inner
			}
		}
	}
	return strings.Trim(s, "/・|｜,、:：- \t")
}

func parsePriceToken(token string) (int, string, bool) {
	t := strings.TrimSpace(token)
	if strings.Contains(t, "無料") {
		return 0, "無料", true
	}
	isPlus := strings.HasPrefix(t, "+")
	var digits strings.Builder
	for _, r := range t {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	if digits.Len() == 0 {
		return 0, "", false
	}
	amount, err := strconv.Atoi(digits.String())
	if err != nil || amount < 0 || amount > 100000 {
		return 0, "", false
	}
	formatted := formatYen(amount)
	if isPlus {
		formatted = "+" + formatted
	}
	return amount, formatted, true
}

func formatYen(amount int) string {
	s := strconv.Itoa(amount)
	if len(s) <= 3 {
		return "¥" + s
	}
	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	if s != "" {
		parts = append([]string{s}, parts...)
	}
	return "¥" + strings.Join(parts, ",")
}

func inferDefaultTier(token string) string {
	switch {
	case strings.Contains(token, "観覧無料"):
		return "観覧"
	case strings.Contains(token, "入場無料"):
		return "入場"
	default:
		return "通常"
	}
}
