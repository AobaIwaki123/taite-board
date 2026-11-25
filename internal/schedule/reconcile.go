package schedule

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/AobaIwaki123/taite-board/internal/resolve"
)

// ReconcileReport summarizes entity resolution, normalization, and deduplication changes.
type ReconcileReport struct {
	BeforeCount       int      `json:"before_count"`
	AfterCount        int      `json:"after_count"`
	MergedCount       int      `json:"merged_count"`
	VenuesNormalized  int      `json:"venues_normalized"`
	TitlesCleaned     int      `json:"titles_cleaned"`
	BoothsNormalized  int      `json:"booths_normalized"`
	TicketsNormalized int      `json:"tickets_normalized"`
	SourcesNormalized int      `json:"sources_normalized"`
	Changed           bool     `json:"changed"`
	Changes           []string `json:"changes,omitempty"`
}

var (
	katakanaBoothRegex = regexp.MustCompile(`ロビ-([A-Za-z0-9ａ-ｚＡ-Ｚ０-９]*)`)
	strayBenefitRegex  = regexp.MustCompile(`特典会.*?\d{1,2}:\d{2}`)
)

// ApplyReconcileSchedule normalizes venue canonical IDs/names, cleans titles/booths/tickets/sources,
// merges duplicate sessions on the same date using deterministic entity resolution rules,
// and sorts sessions chronologically.
func ApplyReconcileSchedule(raw []byte, resolver *resolve.Resolver) ([]byte, ReconcileReport, error) {
	var report ReconcileReport
	if resolver == nil {
		var err error
		resolver, err = resolve.NewResolver("")
		if err != nil {
			return nil, report, fmt.Errorf("initializing default resolver: %w", err)
		}
	}

	var doc rootDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, report, fmt.Errorf("unmarshaling schedule json: %w", err)
	}
	if doc.SchemaVersion < 1 {
		doc.SchemaVersion = 1
	}
	if doc.Timezone == "" {
		doc.Timezone = "Asia/Tokyo"
	}

	report.BeforeCount = len(doc.Sessions)

	sessions := make([]map[string]interface{}, 0, len(doc.Sessions))
	for _, sRaw := range doc.Sessions {
		var sMap map[string]interface{}
		if err := json.Unmarshal(sRaw, &sMap); err != nil {
			continue
		}
		if reconcileSingleSession(sMap, resolver, &report) {
			report.Changed = true
		}
		sessions = append(sessions, sMap)
	}

	// Merge duplicate sessions on the same date into surviving sessions.
	survivors := make([]map[string]interface{}, 0, len(sessions))
	for _, candidate := range sessions {
		merged := false
		for idx, existing := range survivors {
			if shouldMergeSessions(existing, candidate) {
				survivorID, _ := existing["id"].(string)
				mergedID, _ := candidate["id"].(string)
				mergeReconciledSessions(existing, candidate, resolver)
				survivors[idx] = existing
				report.MergedCount++
				report.Changed = true
				report.Changes = append(report.Changes, fmt.Sprintf("merged %s into %s (%s)", mergedID, survivorID, stringField(existing, "date")))
				merged = true
				break
			}
		}
		if !merged {
			survivors = append(survivors, candidate)
		}
	}

	report.AfterCount = len(survivors)

	// Marshal survivors and run ApplySortSchedule to ensure deterministic chronological order.
	rawSessions := make([]json.RawMessage, 0, len(survivors))
	for _, s := range survivors {
		b, err := json.Marshal(s)
		if err != nil {
			return nil, report, fmt.Errorf("marshaling session %s: %w", stringField(s, "id"), err)
		}
		rawSessions = append(rawSessions, json.RawMessage(b))
	}

	doc.Sessions = rawSessions
	updatedRaw, err := json.Marshal(doc)
	if err != nil {
		return nil, report, fmt.Errorf("marshaling reconciled doc: %w", err)
	}

	sortedRaw, _, err := ApplySortSchedule(updatedRaw)
	if err != nil {
		return nil, report, fmt.Errorf("sorting reconciled schedule: %w", err)
	}

	// Preserve existing order/content check cleanly; if nothing changed logically, check whether sort itself reordered.
	if !report.Changed && orderChanged(doc.Sessions, sortedRaw) {
		report.Changed = true
		report.Changes = append(report.Changes, "reordered sessions chronologically")
	}

	if !report.Changed {
		return raw, report, nil
	}
	return sortedRaw, report, nil
}

func reconcileSingleSession(sMap map[string]interface{}, resolver *resolve.Resolver, report *ReconcileReport) bool {
	changed := false
	id := stringField(sMap, "id")

	// 1. Clean decorative title emojis and wrapping brackets
	if title := stringField(sMap, "title"); title != "" {
		cleaned := cleanTitleForReconcile(title)
		if cleaned != "" && cleaned != title {
			sMap["title"] = cleaned
			report.TitlesCleaned++
			report.Changes = append(report.Changes, fmt.Sprintf("%s: title %q -> %q", id, title, cleaned))
			changed = true
		}
	}

	// 2. Canonicalize venue via resolver
	if vMap, ok := sMap["venue"].(map[string]interface{}); ok {
		vName := stringField(vMap, "name")
		vCID := stringField(vMap, "canonical_id")
		vMapQuery := stringField(vMap, "map_query")
		cid, display, mapQuery, matched := resolver.ResolveVenueDetail(vName, vCID)
		if matched {
			if cid != "" && vCID != cid {
				vMap["canonical_id"] = cid
				report.VenuesNormalized++
				report.Changes = append(report.Changes, fmt.Sprintf("%s: venue canonical_id %q -> %q", id, vCID, cid))
				changed = true
			}
			if display != "" && vName != display {
				vMap["name"] = display
				report.VenuesNormalized++
				report.Changes = append(report.Changes, fmt.Sprintf("%s: venue name %q -> %q", id, vName, display))
				changed = true
			}
			if mapQuery != "" && vMapQuery != mapQuery {
				vMap["map_query"] = mapQuery
				vMap["map_url"] = fmt.Sprintf("https://maps.google.com/?q=%s", strings.ReplaceAll(mapQuery, " ", "+"))
				changed = true
			}
		} else if vCID == "" && cid != "" {
			vMap["canonical_id"] = cid
			report.VenuesNormalized++
			report.Changes = append(report.Changes, fmt.Sprintf("%s: assigned fallback venue canonical_id %q", id, cid))
			changed = true
		}
		if stringField(vMap, "map_query") == "" && stringField(vMap, "name") != "" {
			q := stringField(vMap, "name")
			vMap["map_query"] = q
			vMap["map_url"] = fmt.Sprintf("https://maps.google.com/?q=%s", strings.ReplaceAll(q, " ", "+"))
			changed = true
		}
		sMap["venue"] = vMap
	}

	// 3. Fix benefit booth katakana hyphenation ("ロビ-B" -> "ロビーB")
	if benMap, ok := sMap["benefit"].(map[string]interface{}); ok {
		booth := strings.TrimSpace(stringField(benMap, "booth"))
		if booth != "" {
			fixed := katakanaBoothRegex.ReplaceAllString(booth, "ロビー$1")
			if fixed != booth {
				benMap["booth"] = fixed
				report.BoothsNormalized++
				report.Changes = append(report.Changes, fmt.Sprintf("%s: benefit.booth %q -> %q", id, booth, fixed))
				changed = true
			}
		}
		sMap["benefit"] = benMap
	}

	// 4. Clear stray admission_bonus mistakenly extracted from a benefit time line
	if bonus := strings.TrimSpace(stringField(sMap, "admission_bonus")); bonus != "" {
		if strayBenefitRegex.MatchString(bonus) && !hasAdmissionBonusKeywords(bonus) {
			sMap["admission_bonus"] = ""
			report.Changes = append(report.Changes, fmt.Sprintf("%s: cleared stray admission_bonus %q", id, bonus))
			changed = true
		}
	}

	// 5. Normalize photo regulation (backfill typed song_number & allowed_songs from rule/raw_text)
	if pMap, ok := sMap["photo"].(map[string]interface{}); ok && pMap != nil {
		if reconcilePhotoRegulation(pMap, id, report) {
			sMap["photo"] = pMap
			changed = true
		}
	}

	// 6. Normalize ticket platform & ensure prices array exists
	if tMap, ok := sMap["ticket"].(map[string]interface{}); ok {
		tURL := strings.TrimSpace(stringField(tMap, "url"))
		if tURL != stringField(tMap, "url") {
			tMap["url"] = tURL
			changed = true
		}
		if tURL != "" {
			detected := detectTicketPlatform(tURL)
			existingPlatform := strings.TrimSpace(stringField(tMap, "platform"))
			if detected != "" && existingPlatform != detected {
				tMap["platform"] = detected
				report.TicketsNormalized++
				report.Changes = append(report.Changes, fmt.Sprintf("%s: ticket.platform %q -> %q", id, existingPlatform, detected))
				changed = true
			}
		}
		if _, hasPrices := tMap["prices"]; !hasPrices || tMap["prices"] == nil {
			tMap["prices"] = []interface{}{}
			changed = true
		}
		sMap["ticket"] = tMap
	}

	// 6. Normalize source_tweet_url & source_refs & images
	sourceURL := strings.TrimSpace(stringField(sMap, "source_tweet_url"))
	if sourceURL != "" {
		ref, isX := buildSourceRefFromURL(sourceURL)
		if !isX {
			if ref != nil {
				addUniqueSourceRef(sMap, ref)
			}
			sMap["source_tweet_url"] = ""
			report.SourcesNormalized++
			report.Changes = append(report.Changes, fmt.Sprintf("%s: moved non-X source_tweet_url to source_refs", id))
			changed = true
		} else if ref != nil {
			if addUniqueSourceRef(sMap, ref) {
				report.SourcesNormalized++
				changed = true
			}
		}
	}
	if dedupeSourceRefs(sMap) {
		report.SourcesNormalized++
		changed = true
	}
	if dedupeImages(sMap) {
		changed = true
	}

	return changed
}

func reconcilePhotoRegulation(pMap map[string]interface{}, id string, report *ReconcileReport) bool {
	changed := false
	combined := strings.TrimSpace(stringField(pMap, "raw_text") + "\n" + stringField(pMap, "rule"))
	song, songNum, _, _ := resolve.ExtractPhotoSongDetails(combined)

	// Normalize / backfill song_number (strictly positive integer or omitted)
	if rawNum, exists := pMap["song_number"]; exists {
		if rawNum == nil {
			delete(pMap, "song_number")
			changed = true
		} else if f, ok := rawNum.(float64); !ok || f != float64(int(f)) || int(f) < 1 {
			delete(pMap, "song_number")
			changed = true
		}
	}
	if _, hasValid := pMap["song_number"]; !hasValid && songNum > 0 {
		pMap["song_number"] = songNum
		report.Changes = append(report.Changes, fmt.Sprintf("%s: photo.song_number = %d", id, songNum))
		changed = true
	}

	// Backfill allowed_songs if song extracted cleanly from rule/raw_text
	if existingSongs, _ := pMap["allowed_songs"].([]interface{}); len(existingSongs) == 0 && song != "" {
		pMap["allowed_songs"] = []string{song}
		report.Changes = append(report.Changes, fmt.Sprintf("%s: photo.allowed_songs = [%q]", id, song))
		changed = true
	}
	return changed
}

func cleanTitleForReconcile(title string) string {
	t := strings.TrimSpace(title)
	trimDecor := func(s string) string {
		return strings.TrimFunc(s, func(r rune) bool {
			return unicode.IsSpace(r) || r == '🎀' || r == '✨' || r == '🎂' || r == '🤍' || r == '🕰' || r == '\ufe0f'
		})
	}
	t = trimDecor(t)
	if (strings.HasPrefix(t, "『") && strings.HasSuffix(t, "』")) ||
		(strings.HasPrefix(t, "「") && strings.HasSuffix(t, "」")) {
		runes := []rune(t)
		if len(runes) >= 2 {
			t = trimDecor(string(runes[1 : len(runes)-1]))
		}
	}
	return t
}

func hasAdmissionBonusKeywords(s string) bool {
	for _, kw := range []string{"チェキ", "動画", "トーク", "写メ", "サイン", "お目当て", "指名", "入場特典", "券"} {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}

func shouldMergeSessions(a, b map[string]interface{}) bool {
	if !sessionsCompatible(a, b) {
		return false
	}

	venueIDA := nestedStringField(a, "venue", "canonical_id")
	venueIDB := nestedStringField(b, "venue", "canonical_id")
	liveStartA := nestedStringField(a, "live", "start_time")
	liveStartB := nestedStringField(b, "live", "start_time")

	// Signal 1: Overlapping normalized titles
	titleKeyA := normalizeMatchKey(stringField(a, "title"))
	titleKeyB := normalizeMatchKey(stringField(b, "title"))
	if keysOverlap(titleKeyA, titleKeyB) {
		return true
	}
	if a, ok := parseHHMM(liveStartA); ok {
		if b, ok := parseHHMM(liveStartB); ok && a == b {
			return true
		}
	}
	if shareSourceIdentity(a, b) {
		return true
	}
	sameVenue := venueIDA != "" && venueIDA != "venue:未定" && venueIDA == venueIDB
	if !sameVenue {
		return false
	}
	_, okA := parseHHMM(liveStartA)
	_, okB := parseHHMM(liveStartB)
	if okA && okB && liveStartA == liveStartB {
		return true
	}
	return isPlaceholderTitle(stringField(a, "title")) || isPlaceholderTitle(stringField(b, "title"))
}

func sessionsCompatible(a, b map[string]interface{}) bool {
	dateA := stringField(a, "date")
	dateB := stringField(b, "date")
	if dateA == "" || dateA != dateB {
		return false
	}

	// Guard 1: Separate multi-part shows on the same day (1部 vs 2部, 昼 vs 夜) never merge
	sessNameA := sessionLabel(a)
	sessNameB := sessionLabel(b)
	if sessNameA != "" && sessNameB != "" && sessNameA != sessNameB {
		if isDistinctSessionLabel(sessNameA, sessNameB) {
			return false
		}
	}

	// Guard 2: Concrete different venues (excluding placeholder venue:未定) never merge
	venueIDA := nestedStringField(a, "venue", "canonical_id")
	venueIDB := nestedStringField(b, "venue", "canonical_id")
	if venueIDA != "" && venueIDB != "" && venueIDA != "venue:未定" && venueIDB != "venue:未定" && venueIDA != venueIDB {
		return false
	}
	nameA, nameB := nestedStringField(a, "venue", "name"), nestedStringField(b, "venue", "name")
	if (venueIDA == "" || venueIDB == "") && concreteVenue(nameA) && concreteVenue(nameB) {
		a, b := normalizeMatchKey(nameA), normalizeMatchKey(nameB)
		if a != b && !keysOverlap(a, b) {
			return false
		}
	}

	// Guard 3: Concrete different live start times (>15 min apart) never merge
	liveStartA := nestedStringField(a, "live", "start_time")
	liveStartB := nestedStringField(b, "live", "start_time")
	if minA, okA := parseHHMM(liveStartA); okA {
		if minB, okB := parseHHMM(liveStartB); okB {
			diff := minA - minB
			if diff < 0 {
				diff = -diff
			}
			if diff > 15 {
				return false
			}
		}
	}

	return true
}

func sessionLabel(s map[string]interface{}) string {
	if label := strings.TrimSpace(stringField(s, "session_name")); label != "" {
		return label
	}
	title := resolve.NormalizeText(stringField(s, "title"))
	if strings.Contains(title, "1部") && !strings.Contains(title, "2部") {
		return "1部"
	}
	if strings.Contains(title, "2部") && !strings.Contains(title, "1部") {
		return "2部"
	}
	return ""
}

func concreteVenue(s string) bool {
	s = strings.TrimSpace(s)
	return s != "" && s != "未定" && s != "告知待ち"
}

func mergeReconciledSessions(dst, src map[string]interface{}, resolver *resolve.Resolver) {
	recordSourceScore(src, sourcePriority(src))
	selectSource(dst, src, nil)
	// Title: prefer non-placeholder, longer/clean title
	dstTitle := stringField(dst, "title")
	srcTitle := stringField(src, "title")
	if srcTitle != "" && (dstTitle == "" || isPlaceholderTitle(dstTitle) || (!isPlaceholderTitle(srcTitle) && len([]rune(srcTitle)) > len([]rune(dstTitle)) && strings.Contains(srcTitle, "〜"))) {
		dst["title"] = srcTitle
	}
	if stringField(dst, "session_name") == "" && stringField(src, "session_name") != "" {
		dst["session_name"] = src["session_name"]
	}

	// Venue: prefer known canonical venue over placeholder
	srcVenue, _ := src["venue"].(map[string]interface{})
	dstVenue, _ := dst["venue"].(map[string]interface{})
	if srcVenue != nil {
		if dstVenue == nil || stringField(dstVenue, "name") == "" || stringField(dstVenue, "name") == "未定" {
			dst["venue"] = srcVenue
		} else {
			_, _, _, srcCanonical := resolver.ResolveVenueDetail(stringField(srcVenue, "name"), stringField(srcVenue, "canonical_id"))
			_, _, _, dstCanonical := resolver.ResolveVenueDetail(stringField(dstVenue, "name"), stringField(dstVenue, "canonical_id"))
			if srcCanonical && !dstCanonical {
				dst["venue"] = srcVenue
			}
		}
	}

	// Live times: prefer concrete HH:MM
	dstLive, _ := dst["live"].(map[string]interface{})
	srcLive, _ := src["live"].(map[string]interface{})
	if dstLive == nil {
		dstLive = map[string]interface{}{"start_time": "--:--", "end_time": "", "stage": nil}
	}
	if srcLive != nil {
		if _, ok := parseHHMM(stringField(dstLive, "start_time")); !ok {
			if st := stringField(srcLive, "start_time"); st != "" && st != "--:--" {
				dstLive["start_time"] = st
			}
		}
		if stringField(dstLive, "end_time") == "" && stringField(srcLive, "end_time") != "" {
			dstLive["end_time"] = srcLive["end_time"]
		}
		if dstLive["stage"] == nil && srcLive["stage"] != nil {
			dstLive["stage"] = srcLive["stage"]
		}
	}
	dst["live"] = dstLive

	// Benefit: prefer concrete HH:MM and non-placeholder booth
	dstBen, _ := dst["benefit"].(map[string]interface{})
	srcBen, _ := src["benefit"].(map[string]interface{})
	if dstBen == nil {
		dstBen = map[string]interface{}{"type": "並行", "start_time": "--:--", "end_time": "", "booth": ""}
	}
	if srcBen != nil {
		if _, ok := parseHHMM(stringField(dstBen, "start_time")); !ok {
			if st := stringField(srcBen, "start_time"); st != "" && st != "--:--" {
				dstBen["start_time"] = st
				if stringField(srcBen, "type") != "" {
					dstBen["type"] = srcBen["type"]
				}
			}
		}
		if stringField(dstBen, "end_time") == "" && stringField(srcBen, "end_time") != "" {
			dstBen["end_time"] = srcBen["end_time"]
		}
		dstBooth := stringField(dstBen, "booth")
		srcBooth := stringField(srcBen, "booth")
		if srcBooth != "" && (dstBooth == "" || dstBooth == "告知待ち" || dstBooth == "通常") && srcBooth != "告知待ち" {
			dstBen["booth"] = srcBooth
		}
	}
	dst["benefit"] = dstBen

	// Photo: prefer non-nil
	if dst["photo"] == nil && src["photo"] != nil {
		dst["photo"] = src["photo"]
	}

	// Admission bonus & special notes
	if stringField(dst, "admission_bonus") == "" && stringField(src, "admission_bonus") != "" {
		dst["admission_bonus"] = src["admission_bonus"]
	}
	dstNotes := stringField(dst, "special_notes")
	srcNotes := stringField(src, "special_notes")
	if dstNotes == "" && srcNotes != "" {
		dst["special_notes"] = srcNotes
	} else if srcNotes != "" && !strings.Contains(dstNotes, srcNotes) {
		dst["special_notes"] = dstNotes + " / " + srcNotes
	}

	// Ticket: fill missing URL/platform/prices
	dstTicket, _ := dst["ticket"].(map[string]interface{})
	srcTicket, _ := src["ticket"].(map[string]interface{})
	if srcTicket != nil {
		if dstTicket == nil {
			dst["ticket"] = srcTicket
		} else {
			if stringField(dstTicket, "url") == "" && stringField(srcTicket, "url") != "" {
				dstTicket["url"] = srcTicket["url"]
			}
			if stringField(dstTicket, "platform") == "" && stringField(srcTicket, "platform") != "" {
				dstTicket["platform"] = srcTicket["platform"]
			}
			dstPrices, _ := dstTicket["prices"].([]interface{})
			srcPrices, _ := srcTicket["prices"].([]interface{})
			if len(dstPrices) == 0 && len(srcPrices) > 0 {
				dstTicket["prices"] = srcPrices
			}
			dst["ticket"] = dstTicket
		}
	}

	// Images & source refs
	if stringField(dst, "image_url") == "" && stringField(src, "image_url") != "" {
		dst["image_url"] = src["image_url"]
	}
	if srcImgs, ok := src["images"].([]interface{}); ok {
		dstImgs, _ := dst["images"].([]interface{})
		dst["images"] = append(dstImgs, srcImgs...)
		dedupeImages(dst)
	}

	if srcRefs, ok := src["source_refs"].([]interface{}); ok {
		for _, r := range srcRefs {
			if rm, ok := r.(map[string]interface{}); ok {
				addUniqueSourceRef(dst, rm)
			}
		}
	}
}

func shareSourceIdentity(a, b map[string]interface{}) bool {
	aTweet := stringField(a, "source_tweet_url")
	bTweet := stringField(b, "source_tweet_url")
	if aTweet != "" && aTweet == bTweet {
		return true
	}

	idsA := collectSourceKeys(a)
	idsB := collectSourceKeys(b)
	for k := range idsA {
		if idsB[k] {
			return true
		}
	}
	return false
}

func collectSourceKeys(sMap map[string]interface{}) map[string]bool {
	keys := make(map[string]bool)
	if u := stringField(sMap, "source_tweet_url"); u != "" {
		keys["url:"+u] = true
		if m := statusIDRegex.FindStringSubmatch(u); len(m) > 1 {
			keys["id:"+m[1]] = true
		}
	}
	refs, _ := sMap["source_refs"].([]interface{})
	for _, r := range refs {
		rm, _ := r.(map[string]interface{})
		if rm == nil {
			continue
		}
		if sid := stringField(rm, "source_id"); sid != "" {
			keys["id:"+sid] = true
		}
		if u := stringField(rm, "url"); u != "" {
			keys["url:"+u] = true
		}
	}
	return keys
}

func isDistinctSessionLabel(a, b string) bool {
	for _, pair := range [][2]string{{"1部", "2部"}, {"昼", "夜"}, {"第1部", "第2部"}} {
		if (strings.Contains(a, pair[0]) && strings.Contains(b, pair[1])) ||
			(strings.Contains(a, pair[1]) && strings.Contains(b, pair[0])) {
			return true
		}
	}
	return false
}

func isPlaceholderTitle(title string) bool {
	t := strings.TrimSpace(title)
	return t == "" || t == "未定" || strings.HasPrefix(t, "ライブ出演 (")
}

func parseHHMM(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if len(s) != 5 || s[2] != ':' {
		return 0, false
	}
	h, err1 := strconv.Atoi(s[:2])
	m, err2 := strconv.Atoi(s[3:])
	if err1 != nil || err2 != nil || h < 0 || h > 29 || m < 0 || m > 59 {
		return 0, false
	}
	return h*60 + m, true
}

func stringField(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	s, _ := m[key].(string)
	return s
}

func nestedStringField(m map[string]interface{}, parent, key string) string {
	if m == nil {
		return ""
	}
	sub, _ := m[parent].(map[string]interface{})
	return stringField(sub, key)
}

func addUniqueSourceRef(sMap map[string]interface{}, ref map[string]interface{}) bool {
	if ref == nil {
		return false
	}
	nID := stringField(ref, "source_id")
	nURL := stringField(ref, "url")
	refs, _ := sMap["source_refs"].([]interface{})
	for _, er := range refs {
		em, _ := er.(map[string]interface{})
		if em == nil {
			continue
		}
		eID := stringField(em, "source_id")
		eURL := stringField(em, "url")
		if (nID != "" && eID == nID) || (nURL != "" && eURL == nURL) {
			return false
		}
	}
	sMap["source_refs"] = append(refs, ref)
	return true
}

func dedupeSourceRefs(sMap map[string]interface{}) bool {
	refs, _ := sMap["source_refs"].([]interface{})
	if len(refs) <= 1 {
		return false
	}
	seenID := make(map[string]bool)
	seenURL := make(map[string]bool)
	out := make([]interface{}, 0, len(refs))
	changed := false
	for _, r := range refs {
		rm, _ := r.(map[string]interface{})
		if rm == nil {
			changed = true
			continue
		}
		sid := stringField(rm, "source_id")
		u := stringField(rm, "url")
		if (sid != "" && seenID[sid]) || (u != "" && seenURL[u]) {
			changed = true
			continue
		}
		if sid != "" {
			seenID[sid] = true
		}
		if u != "" {
			seenURL[u] = true
		}
		out = append(out, rm)
	}
	if changed {
		sMap["source_refs"] = out
	}
	return changed
}

func imageKey(raw string) (string, int) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return raw, 0
	}
	u.Host = strings.ToLower(u.Host)
	if u.Host == "pbs.twimg.com" && strings.HasPrefix(u.Path, "/media/") {
		id := strings.TrimPrefix(strings.TrimSuffix(u.Path, "/"), "/media/")
		if i := strings.LastIndex(id, "."); i >= 0 {
			id = id[:i]
		}
		quality := map[string]int{"small": 1, "medium": 2, "large": 3, "orig": 4}[u.Query().Get("name")]
		return "twimg:" + id, quality
	}
	u.Scheme = "https"
	u.Path = strings.TrimSuffix(u.Path, "/")
	u.Fragment = ""
	u.RawQuery = u.Query().Encode()
	return u.String(), 0
}

func dedupeImages(sMap map[string]interface{}) bool {
	imgs, _ := sMap["images"].([]interface{})
	if len(imgs) <= 1 {
		return false
	}
	seen := make(map[string]int)
	out := make([]interface{}, 0, len(imgs))
	changed := false
	for _, img := range imgs {
		im, _ := img.(map[string]interface{})
		if im == nil {
			changed = true
			continue
		}
		u := stringField(im, "url")
		if u != "" {
			key, quality := imageKey(u)
			if idx, exists := seen[key]; exists {
				old, _ := out[idx].(map[string]interface{})
				_, oldQuality := imageKey(stringField(old, "url"))
				if quality > oldQuality {
					out[idx] = im
				}
				changed = true
				continue
			}
			seen[key] = len(out)
		}
		out = append(out, im)
	}
	if changed {
		sMap["images"] = out
	}
	return changed
}

func orderChanged(before []json.RawMessage, afterBytes []byte) bool {
	var afterDoc struct {
		Sessions []json.RawMessage `json:"sessions"`
	}
	if err := json.Unmarshal(afterBytes, &afterDoc); err != nil {
		return false
	}
	if len(before) != len(afterDoc.Sessions) {
		return true
	}
	for i := range before {
		var a, b struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(before[i], &a)
		_ = json.Unmarshal(afterDoc.Sessions[i], &b)
		if !bytes.Equal([]byte(a.ID), []byte(b.ID)) {
			return true
		}
	}
	return false
}
