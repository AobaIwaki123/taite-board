package schedule

// sourceScore records the information present in a single input, not the
// combined session. This avoids attributing fields from one post to another.
func sourceScore(s map[string]interface{}) int {
	score := 0
	if title := stringField(s, "title"); title != "" && !isPlaceholderTitle(title) {
		score++
	}
	if venue := nestedStringField(s, "venue", "name"); venue != "" && venue != "未定" && venue != "告知待ち" {
		score++
	}
	for _, field := range []string{"start_time", "end_time"} {
		if _, ok := parseHHMM(nestedStringField(s, "live", field)); ok {
			score += 2
		}
		if _, ok := parseHHMM(nestedStringField(s, "benefit", field)); ok {
			score++
		}
	}
	if nestedStringField(s, "ticket", "url") != "" {
		score++
	}
	if stringField(s, "admission_bonus") != "" {
		score++
	}
	if photo, ok := s["photo"].(map[string]interface{}); ok && len(photo) > 0 {
		score++
	}
	return score
}

func sessionFromAddOptions(o AddSessionOptions) map[string]interface{} {
	s := map[string]interface{}{
		"title": o.Title, "venue": map[string]interface{}{"name": o.VenueName},
		"live":    map[string]interface{}{"start_time": o.LiveStartTime, "end_time": o.LiveEndTime},
		"benefit": map[string]interface{}{"start_time": o.BenefitStartTime, "end_time": o.BenefitEndTime},
		"ticket":  map[string]interface{}{"url": o.TicketURL}, "admission_bonus": o.AdmissionBonus,
	}
	if o.PhotoRule != "" || o.PhotoSong != "" || o.PhotoSongNumber > 0 || o.PhotoAvailable != nil {
		s["photo"] = map[string]interface{}{"present": true}
	}
	if _, isX := buildSourceRefFromURL(o.SourceTweetURL); isX {
		s["source_tweet_url"] = o.SourceTweetURL
	}
	return s
}

func sourcePriority(s map[string]interface{}) int {
	url := stringField(s, "source_tweet_url")
	refs, _ := s["source_refs"].([]interface{})
	for _, raw := range refs {
		ref, _ := raw.(map[string]interface{})
		if stringField(ref, "url") == url {
			switch v := ref["information_score"].(type) {
			case float64:
				return int(v)
			case int:
				return v
			}
		}
	}
	// Legacy sessions have no per-source history; the pre-merge snapshot is
	// the best available estimate and is persisted before combining records.
	return sourceScore(s)
}

func recordSourceScore(s map[string]interface{}, score int) {
	u := stringField(s, "source_tweet_url")
	if u == "" {
		return
	}
	ref, ok := buildSourceRefFromURL(u)
	if !ok || ref == nil {
		return
	}
	refs, _ := s["source_refs"].([]interface{})
	for _, raw := range refs {
		existing, _ := raw.(map[string]interface{})
		if stringField(existing, "url") == u {
			old := -1
			switch v := existing["information_score"].(type) {
			case float64:
				old = int(v)
			case int:
				old = v
			}
			if score > old {
				existing["information_score"] = score
			}
			return
		}
	}
	ref["information_score"] = score
	s["source_refs"] = append(refs, ref)
}

func selectSource(dst, src map[string]interface{}, refs []map[string]interface{}) {
	dstURL, srcURL := stringField(dst, "source_tweet_url"), stringField(src, "source_tweet_url")
	dstScore, srcScore := sourcePriority(dst), sourcePriority(src)
	if dstURL != "" {
		recordSourceScore(dst, dstScore)
	}
	for _, ref := range refs {
		if stringField(ref, "url") == srcURL {
			ref["information_score"] = srcScore
		}
	}
	if srcURL != "" && (dstURL == "" || srcScore > dstScore) {
		dst["source_tweet_url"] = srcURL
	}
	if srcURL != "" && stringField(dst, "source_tweet_url") == srcURL {
		recordSourceScore(dst, srcScore)
	}
}
