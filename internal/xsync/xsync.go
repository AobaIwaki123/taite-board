package xsync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/AobaIwaki123/taite-board/internal/candidate"
	"github.com/AobaIwaki123/taite-board/internal/gcs"
	"github.com/AobaIwaki123/taite-board/internal/raw"
	"github.com/AobaIwaki123/taite-board/internal/resolve"
	"github.com/AobaIwaki123/taite-board/internal/schedule"
	"github.com/AobaIwaki123/taite-board/internal/scrape"
	"github.com/AobaIwaki123/taite-board/internal/validate"
)

// PostFetcher fetches an X post by URL.
type PostFetcher interface {
	FetchPost(ctx context.Context, postURL string) (*raw.Post, error)
}

// Options configures bulk X-link re-synchronization with explicit cost guards.
type Options struct {
	Fetcher            PostFetcher
	Resolver           *resolve.Resolver
	ImageFingerprinter schedule.ImageFingerprinter
	AIParser           scrape.AIParserClient // optional; only called when WithAI=true and session passes cost guards
	WithAI             bool                  // defaults to false ($0 AI cost)
	MinImagesToSkip    int                   // skip X fetch when session already has >= this many images (default 2; 0 disables)
	IncludePast        bool                  // if true, process past sessions too (for backfilling 11.25 etc.)
	RequestDelay       time.Duration         // polite pause between X post fetches
	NowFunc            func() time.Time
}

// SessionOutcome records what happened for a single session.
type SessionOutcome struct {
	SessionID     string   `json:"session_id"`
	Date          string   `json:"date"`
	Title         string   `json:"title"`
	Status        string   `json:"status"` // updated | unchanged | skipped | fetch_error
	Reason        string   `json:"reason,omitempty"`
	XLinks        []string `json:"x_links,omitempty"`
	ImagesBefore  int      `json:"images_before"`
	ImagesAfter   int      `json:"images_after"`
	AICalled      bool     `json:"ai_called,omitempty"`
	AISkipReason  string   `json:"ai_skip_reason,omitempty"`
}

// Report summarizes a group's X-link re-sync run.
type Report struct {
	Group        string           `json:"group"`
	Total        int              `json:"total_sessions"`
	Updated      int              `json:"updated"`
	Skipped      int              `json:"skipped"`
	FetchErrors  int              `json:"fetch_errors"`
	AICalls      int              `json:"ai_calls"`
	Changed      bool             `json:"changed"`
	Written      bool             `json:"written,omitempty"`
	Outcomes     []SessionOutcome `json:"outcomes"`
}

type sessionView struct {
	ID             string `json:"id"`
	Date           string `json:"date"`
	Title          string `json:"title"`
	SourceTweetURL string `json:"source_tweet_url"`
	Live           struct {
		StartTime string `json:"start_time"`
	} `json:"live"`
	Benefit struct {
		StartTime string `json:"start_time"`
	} `json:"benefit"`
	Images []struct {
		URL string `json:"url"`
	} `json:"images"`
	SourceRefs []struct {
		Type string `json:"type"`
		URL  string `json:"url"`
	} `json:"source_refs"`
}

func isXURL(rawURL string) bool {
	u := strings.ToLower(strings.TrimSpace(rawURL))
	return strings.Contains(u, "x.com/") || strings.Contains(u, "twitter.com/")
}

func collectSessionXLinks(s sessionView) []string {
	seen := make(map[string]bool)
	var out []string
	add := func(u string) {
		trimmed := strings.TrimSpace(u)
		if trimmed == "" || !isXURL(trimmed) || seen[trimmed] {
			return
		}
		seen[trimmed] = true
		out = append(out, trimmed)
	}
	add(s.SourceTweetURL)
	for _, ref := range s.SourceRefs {
		if ref.Type == "x_post" || isXURL(ref.URL) {
			add(ref.URL)
		}
	}
	return out
}

func hasTimeValue(v string) bool {
	t := strings.TrimSpace(v)
	return t != "" && t != "--:--"
}

// ShouldSkipAI applies cost protection: never pay for AI parsing when a session already
// has >= 2 images (flyer + timetable) or already has both live and benefit start times.
func ShouldSkipAI(imageCount int, liveStart, benefitStart string) (bool, string) {
	if imageCount >= 2 {
		return true, "already has >=2 images"
	}
	if hasTimeValue(liveStart) && hasTimeValue(benefitStart) {
		return true, "already has live and benefit times"
	}
	return false, ""
}

// ReSyncScheduleBytes re-fetches existing X links to backfill missing images (and optional missing fields)
// while applying PreserveExisting, VisualDedupeScheduleBytes, and AI cost guards.
func ReSyncScheduleBytes(ctx context.Context, group string, rawBytes []byte, opts Options) ([]byte, *Report, error) {
	if opts.Fetcher == nil {
		return nil, nil, errors.New("x post fetcher is required")
	}
	nowFn := opts.NowFunc
	if nowFn == nil {
		nowFn = time.Now
	}
	jst := time.FixedZone("JST", 9*60*60)
	today := nowFn().In(jst).Format("2006-01-02")

	var doc struct {
		Sessions []sessionView `json:"sessions"`
	}
	if err := json.Unmarshal(rawBytes, &doc); err != nil {
		return nil, nil, fmt.Errorf("unmarshaling schedule: %w", err)
	}

	report := &Report{Group: group, Total: len(doc.Sessions)}
	updated := rawBytes
	postCache := make(map[string]*raw.Post)
	fetchedAny := false

	for _, s := range doc.Sessions {
		out := SessionOutcome{
			SessionID:    s.ID,
			Date:         s.Date,
			Title:        s.Title,
			ImagesBefore: len(s.Images),
			ImagesAfter:  len(s.Images),
			XLinks:       collectSessionXLinks(s),
		}

		if len(out.XLinks) == 0 {
			out.Status = "skipped"
			out.Reason = "no x links"
			report.Skipped++
			report.Outcomes = append(report.Outcomes, out)
			continue
		}
		if !opts.IncludePast && s.Date < today {
			out.Status = "skipped"
			out.Reason = "past session"
			report.Skipped++
			report.Outcomes = append(report.Outcomes, out)
			continue
		}
		if opts.MinImagesToSkip > 0 && len(s.Images) >= opts.MinImagesToSkip {
			out.Status = "skipped"
			out.Reason = fmt.Sprintf("already has %d images (>= %d)", len(s.Images), opts.MinImagesToSkip)
			report.Skipped++
			report.Outcomes = append(report.Outcomes, out)
			continue
		}

		var mergedPhotos []string
		seenPhoto := make(map[string]bool)
		var fetchedPosts []*raw.Post
		fetchErrs := 0

		for _, link := range out.XLinks {
			post, ok := postCache[link]
			if !ok {
				if fetchedAny && opts.RequestDelay > 0 {
					select {
					case <-ctx.Done():
						return nil, nil, ctx.Err()
					case <-time.After(opts.RequestDelay):
					}
				}
				fetchedAny = true
				var err error
				post, err = fetchWithRetry(ctx, opts.Fetcher, link, opts.RequestDelay)
				if err != nil || post == nil {
					fetchErrs++
					continue
				}
				postCache[link] = post
			}
			fetchedPosts = append(fetchedPosts, post)
			for _, img := range post.Images {
				trimmed := strings.TrimSpace(img)
				if trimmed != "" && !seenPhoto[trimmed] {
					seenPhoto[trimmed] = true
					mergedPhotos = append(mergedPhotos, trimmed)
				}
			}
		}

		if len(fetchedPosts) == 0 && fetchErrs > 0 {
			out.Status = "fetch_error"
			out.Reason = "all x link fetches failed"
			report.FetchErrors++
			report.Outcomes = append(report.Outcomes, out)
			continue
		}

		addOpts := schedule.AddSessionOptions{
			Date:             s.Date,
			TargetSessionID:  s.ID,
			PreserveExisting: true,
			ImageURLs:        mergedPhotos,
		}

		if opts.Resolver != nil && len(fetchedPosts) > 0 {
			for _, post := range fetchedPosts {
				cand, _, needsAI := opts.Resolver.Resolve(post.Text, post.Images)
				applyResolvedIfMissing(&addOpts, cand)
				if needsAI && opts.WithAI && opts.AIParser != nil {
					skip, reason := ShouldSkipAI(len(s.Images)+len(mergedPhotos), effectiveTime(s.Live.StartTime, addOpts.LiveStartTime), effectiveTime(s.Benefit.StartTime, addOpts.BenefitStartTime))
					if skip {
						if out.AISkipReason == "" {
							out.AISkipReason = reason
						}
						continue
					}
					if aiResp, aiErr := opts.AIParser.Parse(ctx, post); aiErr == nil && aiResp != nil {
						out.AICalled = true
						report.AICalls++
						applyResolvedIfMissing(&addOpts, aiResp.Candidate)
					}
				}
			}
		}

		candidateBytes, _, err := schedule.ApplyAddSession(updated, addOpts)
		if err != nil {
			out.Status = "fetch_error"
			out.Reason = err.Error()
			report.FetchErrors++
			report.Outcomes = append(report.Outcomes, out)
			continue
		}

		if opts.ImageFingerprinter != nil {
			deduped, _, dedupeErr := schedule.VisualDedupeScheduleBytes(ctx, candidateBytes, opts.ImageFingerprinter)
			if dedupeErr != nil {
				return nil, nil, dedupeErr
			}
			candidateBytes = deduped
		}

		valRep, err := validate.ValidateBytes(candidateBytes)
		if err != nil || (valRep != nil && !valRep.IsValid) {
			out.Status = "fetch_error"
			if err != nil {
				out.Reason = err.Error()
			} else {
				out.Reason = strings.Join(valRep.Errors, "; ")
			}
			report.FetchErrors++
			report.Outcomes = append(report.Outcomes, out)
			continue
		}

		afterCount := countSessionImages(candidateBytes, s.ID)
		out.ImagesAfter = afterCount
		if string(candidateBytes) != string(updated) && afterCount != len(s.Images) {
			out.Status = "updated"
			report.Updated++
			report.Changed = true
		} else if !sameJSONSessions(updated, candidateBytes) {
			out.Status = "updated"
			report.Updated++
			report.Changed = true
		} else {
			out.Status = "unchanged"
		}
		updated = candidateBytes
		report.Outcomes = append(report.Outcomes, out)
	}

	return updated, report, nil
}

func fetchWithRetry(ctx context.Context, fetcher PostFetcher, link string, delay time.Duration) (*raw.Post, error) {
	post, err := fetcher.FetchPost(ctx, link)
	if err == nil {
		return post, nil
	}
	wait := delay
	if wait <= 0 {
		wait = 250 * time.Millisecond
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(wait):
	}
	return fetcher.FetchPost(ctx, link)
}

func effectiveTime(existing, incoming string) string {
	if hasTimeValue(existing) {
		return existing
	}
	return incoming
}

func applyResolvedIfMissing(dst *schedule.AddSessionOptions, c candidate.EventCandidate) {
	if dst.LiveStartTime == "" && hasTimeValue(c.LiveStartRaw) {
		dst.LiveStartTime = c.LiveStartRaw
	}
	if dst.LiveEndTime == "" && hasTimeValue(c.LiveEndRaw) {
		dst.LiveEndTime = c.LiveEndRaw
	}
	if dst.BenefitStartTime == "" && hasTimeValue(c.BenefitStartRaw) {
		dst.BenefitStartTime = c.BenefitStartRaw
	}
	if dst.BenefitEndTime == "" && hasTimeValue(c.BenefitEndRaw) {
		dst.BenefitEndTime = c.BenefitEndRaw
	}
	if dst.Booth == "" {
		dst.Booth = c.BenefitBoothRaw
	}
	if dst.AdmissionBonus == "" {
		dst.AdmissionBonus = c.AdmissionBonus
	}
	if dst.TicketURL == "" {
		dst.TicketURL = c.TicketURL
	}
	if dst.PhotoSong == "" {
		dst.PhotoSong = c.PhotoSong
	}
	if dst.PhotoSongNumber == 0 {
		dst.PhotoSongNumber = c.PhotoSongNumber
	}
	if dst.PhotoDevice == "" {
		dst.PhotoDevice = c.PhotoDevice
	}
}

func countSessionImages(rawBytes []byte, id string) int {
	var doc struct {
		Sessions []sessionView `json:"sessions"`
	}
	if json.Unmarshal(rawBytes, &doc) != nil {
		return 0
	}
	for _, s := range doc.Sessions {
		if s.ID == id {
			return len(s.Images)
		}
	}
	return 0
}

func sameJSONSessions(a, b []byte) bool {
	var left, right map[string]interface{}
	if json.Unmarshal(a, &left) != nil || json.Unmarshal(b, &right) != nil {
		return false
	}
	return reflect.DeepEqual(left["sessions"], right["sessions"])
}

// VersionedStore matches gcs.ObjectStore for generation-safe GCS schedule updates.
type VersionedStore interface {
	GetObjectVersion(ctx context.Context, bucket, object string) ([]byte, string, error)
	PutObjectIfGeneration(ctx context.Context, bucket, object, contentType, cacheControl string, data []byte, generation string) error
}

// RunGCSGroup re-syncs a group schedule stored in GCS and updates ribonclock mirrors atomically.
func RunGCSGroup(ctx context.Context, store VersionedStore, bucket, group string, dryRun bool, opts Options) (*Report, error) {
	object := fmt.Sprintf("data/groups/%s/schedule.json", group)
	for attempt := 0; attempt < 3; attempt++ {
		rawBytes, gen, err := store.GetObjectVersion(ctx, bucket, object)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", object, err)
		}
		updated, report, err := ReSyncScheduleBytes(ctx, group, rawBytes, opts)
		if err != nil {
			return nil, err
		}
		if dryRun || !report.Changed {
			return report, nil
		}
		var mirrorGens map[string]string
		if group == "ribonclock" {
			mirrorGens = make(map[string]string)
			for _, m := range []string{"schedule.json", "mock_schedule.json"} {
				_, mg, mErr := store.GetObjectVersion(ctx, bucket, m)
				if mErr != nil {
					return nil, fmt.Errorf("read mirror %s: %w", m, mErr)
				}
				mirrorGens[m] = mg
			}
		}
		err = store.PutObjectIfGeneration(ctx, bucket, object, "application/json; charset=utf-8", "no-cache, max-age=0", updated, gen)
		if errors.Is(err, gcs.ErrGenerationMismatch) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("write %s: %w", object, err)
		}
		report.Written = true
		if group == "ribonclock" {
			for m, mg := range mirrorGens {
				if err := store.PutObjectIfGeneration(ctx, bucket, m, "application/json; charset=utf-8", "no-cache, max-age=0", updated, mg); err != nil {
					return nil, fmt.Errorf("write mirror %s: %w", m, err)
				}
			}
		}
		return report, nil
	}
	return nil, errors.New("schedule changed during all retry attempts")
}
