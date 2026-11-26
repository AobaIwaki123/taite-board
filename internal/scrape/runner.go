package scrape

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/AobaIwaki123/taite-board/internal/candidate"
	"github.com/AobaIwaki123/taite-board/internal/filter"
	"github.com/AobaIwaki123/taite-board/internal/parser"
	"github.com/AobaIwaki123/taite-board/internal/raw"
	"github.com/AobaIwaki123/taite-board/internal/resolve"
	"github.com/AobaIwaki123/taite-board/internal/schedule"
	"github.com/AobaIwaki123/taite-board/internal/validate"
)

// AIParserClient optionally enriches ambiguous/timetable-image candidates using Cloud Run Vision AI.
type AIParserClient interface {
	Parse(ctx context.Context, post *raw.Post) (*parser.ParseResponse, error)
}

// HTTPAIParserClient forwards posts to a deployed Cloud Run taite-parser service.
type HTTPAIParserClient struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

// Parse submits the post text and attached images to the Cloud Run /parse endpoint.
func (c *HTTPAIParserClient) Parse(ctx context.Context, post *raw.Post) (*parser.ParseResponse, error) {
	if strings.TrimSpace(c.BaseURL) == "" {
		return nil, fmt.Errorf("ai parser URL is empty")
	}
	media := make([]parser.MediaItem, 0, len(post.Images))
	for _, img := range post.Images {
		media = append(media, parser.MediaItem{InputURL: img})
	}
	reqPayload := parser.ParseRequest{
		RequestID: fmt.Sprintf("scrape-%d", time.Now().UnixNano()),
		Source:    post.Source,
		SourceID:  post.SourceID,
		Text:      post.Text,
		Media:     media,
	}
	body, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, err
	}

	endpoint := strings.TrimRight(c.BaseURL, "/") + "/parse"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(c.Token) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.Token))
	}

	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 35 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling ai parser: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ai parser status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBytes)))
	}
	var out parser.ParseResponse
	if err := json.Unmarshal(respBytes, &out); err != nil {
		return nil, fmt.Errorf("parsing ai parser response: %w", err)
	}
	return &out, nil
}

// ScheduleAccessor reads and writes a group's schedule JSON bytes.
type ScheduleAccessor interface {
	ReadSchedule(ctx context.Context, group string) ([]byte, error)
	WriteSchedule(ctx context.Context, group string, data []byte) error
}

// FileScheduleAccessor reads and writes a local schedule.json file.
type FileScheduleAccessor struct {
	Path string
}

// ReadSchedule reads schedule JSON from disk.
func (f *FileScheduleAccessor) ReadSchedule(_ context.Context, _ string) ([]byte, error) {
	return os.ReadFile(f.Path)
}

// WriteSchedule persists schedule JSON to disk cleanly.
func (f *FileScheduleAccessor) WriteSchedule(_ context.Context, _ string, data []byte) error {
	return os.WriteFile(f.Path, data, 0644)
}

// GroupConfig holds X target metadata from data/groups.json.
type GroupConfig struct {
	XAccount string `json:"x_account"`
	XUserID  string `json:"x_user_id,omitempty"`
}

// LoadGroupConfig reads x_account and optional x_user_id for a group from groups.json.
func LoadGroupConfig(groupsJSONPath, groupID string) GroupConfig {
	if groupsJSONPath == "" {
		groupsJSONPath = "data/groups.json"
	}
	data, err := os.ReadFile(groupsJSONPath)
	if err != nil {
		return GroupConfig{}
	}
	var root struct {
		Groups map[string]GroupConfig `json:"groups"`
	}
	if err := json.Unmarshal(data, &root); err != nil {
		return GroupConfig{}
	}
	return root.Groups[groupID]
}

// RunOptions configures one execution of the X scrape pipeline.
type RunOptions struct {
	Group            string
	UserID           string // Override numeric X user ID when querying X API v2
	SinceID          string // Explicit override; empty uses stored checkpoint
	IgnoreCheckpoint bool
	MaxResults       int
	RequireMedia     *bool
	MaxAgeHours      int
	AutoUpsert       bool
	DryRun           bool
	GroupsJSONPath   string
	EntitiesPath     string
	Source           TimelineSource
	State            StateStore
	Schedule         ScheduleAccessor
	AIParser         AIParserClient
	NowFunc          func() time.Time
}

// ItemOutcome captures the pipeline decision for a single fetched post.
type ItemOutcome struct {
	SourceID  string       `json:"source_id"`
	URL       string       `json:"url"`
	Status    string       `json:"status"` // "staged" | "auto_upserted" | "ambiguous" | "skipped"
	Stage     filter.Stage `json:"stage,omitempty"`
	Reason    string       `json:"reason,omitempty"`
	SessionID string       `json:"session_id,omitempty"`
	NeedsAI   bool         `json:"needs_ai,omitempty"`
}

// RunReport summarizes counts and per-item outcomes from a scrape pipeline run.
type RunReport struct {
	Group             string         `json:"group"`
	SourceType        string         `json:"source_type"`
	StartedAt         string         `json:"started_at"`
	CompletedAt       string         `json:"completed_at"`
	DryRun            bool           `json:"dry_run"`
	AutoUpsert        bool           `json:"auto_upsert"`
	UsedSinceID       string         `json:"used_since_id,omitempty"`
	NewestSinceID     string         `json:"newest_since_id,omitempty"`
	FetchedCount      int            `json:"fetched_count"`
	PassedCount       int            `json:"passed_count"`
	SkippedCount      int            `json:"skipped_count"`
	SkippedByStage    map[string]int `json:"skipped_by_stage,omitempty"`
	StagedCount       int            `json:"staged_count"`
	AutoUpsertedCount int            `json:"auto_upserted_count"`
	AmbiguousCount    int            `json:"ambiguous_count"`
	FetchErrors       map[string]string `json:"fetch_errors,omitempty"`
	Outcomes          []ItemOutcome  `json:"outcomes,omitempty"`
}

// Runner executes fetch -> multi-stage filter -> resolve (+ optional AI) -> stage / safe upsert.
type Runner struct {
	Resolver *resolve.Resolver
}

// NewRunner initializes a scrape Runner with an entity resolver.
func NewRunner(entitiesPath string) (*Runner, error) {
	res, err := resolve.NewResolver(entitiesPath)
	if err != nil {
		return nil, fmt.Errorf("initializing resolver: %w", err)
	}
	return &Runner{Resolver: res}, nil
}

// Run executes one full scrape cycle for opts.Group without charging or advancing cursors on fetch errors.
func (r *Runner) Run(ctx context.Context, opts RunOptions) (*RunReport, error) {
	if opts.Source == nil {
		return nil, fmt.Errorf("timeline source is required")
	}
	group := strings.TrimSpace(strings.ToLower(opts.Group))
	if group == "" || group == "rincent" {
		group = "ribonclock"
	}

	nowFn := opts.NowFunc
	if nowFn == nil {
		nowFn = time.Now
	}
	now := nowFn()

	stateStore := opts.State
	if stateStore == nil {
		stateStore = &FileStateStore{}
	}
	stateDoc, err := stateStore.Load(ctx)
	if err != nil {
		return nil, err
	}

	cp := stateDoc.Checkpoints[group]
	if cp == nil {
		cp = &GroupCheckpoint{}
		stateDoc.Checkpoints[group] = cp
	}

	sinceID := strings.TrimSpace(opts.SinceID)
	if sinceID == "" && !opts.IgnoreCheckpoint {
		sinceID = cp.LatestSinceID
	}

	var schedBytes []byte
	knownKeys := make(map[string]bool)
	if opts.Schedule != nil {
		if b, sErr := opts.Schedule.ReadSchedule(ctx, group); sErr == nil {
			schedBytes = b
			for k, v := range LoadKnownKeysFromScheduleBytes(b) {
				knownKeys[k] = v
			}
		}
	}
	if !opts.IgnoreCheckpoint {
		for _, id := range cp.ProcessedIDs {
			knownKeys[id] = true
		}
		for _, fp := range cp.ProcessedFingerprints {
			knownKeys[fp] = true
		}
	}

	gCfg := LoadGroupConfig(opts.GroupsJSONPath, group)
	targetUserID := strings.TrimSpace(opts.UserID)
	if targetUserID == "" {
		targetUserID = gCfg.XUserID
	}
	tlRes, err := opts.Source.FetchTimeline(ctx, TimelineRequest{
		Group:      group,
		Account:    gCfg.XAccount,
		UserID:     targetUserID,
		SinceID:    sinceID,
		MaxResults: opts.MaxResults,
	})
	if err != nil {
		return nil, err
	}

	rules := filter.LoadRulesForGroup(opts.GroupsJSONPath, group)
	if opts.RequireMedia != nil {
		rules.RequireMedia = *opts.RequireMedia
	}
	if opts.MaxAgeHours > 0 {
		rules.MaxAgeHours = opts.MaxAgeHours
	} else if opts.MaxAgeHours < 0 {
		rules.MaxAgeHours = 0
	}

	resolver := r.Resolver
	if resolver == nil {
		resolver, _ = resolve.NewResolver(opts.EntitiesPath)
	}

	report := &RunReport{
		Group:          group,
		SourceType:     tlRes.SourceType,
		StartedAt:      now.UTC().Format(time.RFC3339),
		DryRun:         opts.DryRun,
		AutoUpsert:     opts.AutoUpsert,
		UsedSinceID:    sinceID,
		NewestSinceID:  cp.LatestSinceID,
		FetchedCount:   len(tlRes.Posts),
		SkippedByStage: make(map[string]int),
		FetchErrors:    tlRes.FetchErrors,
	}

	scheduleDirty := false
	for _, post := range tlRes.Posts {
		if post == nil {
			continue
		}
		fRes := filter.Evaluate(post, rules, knownKeys, now)
		if !fRes.Passed {
			report.SkippedCount++
			report.SkippedByStage[string(fRes.Stage)]++
			report.Outcomes = append(report.Outcomes, ItemOutcome{
				SourceID: post.SourceID,
				URL:      post.URL,
				Status:   "skipped",
				Stage:    fRes.Stage,
				Reason:   fRes.Reason,
			})
			cp.ProcessedIDs = appendBoundedUnique(cp.ProcessedIDs, post.SourceID, maxTrackedIDs)
			cp.ProcessedFingerprints = appendBoundedUnique(cp.ProcessedFingerprints, fRes.Fingerprint, maxTrackedIDs)
			continue
		}

		report.PassedCount++
		cand, evidences, needsAI := resolver.Resolve(post.Text, post.Images)
		if needsAI && opts.AIParser != nil {
			if aiResp, aiErr := opts.AIParser.Parse(ctx, post); aiErr == nil && aiResp != nil {
				cand = mergeAICandidate(cand, aiResp.Candidate)
				evidences = append(evidences, aiResp.Evidence...)
				needsAI = false
			}
		}

		outcome := ItemOutcome{
			SourceID: post.SourceID,
			URL:      post.URL,
			NeedsAI:  needsAI,
		}

		stagedStatus := StatusPending
		stagedErr := ""
		committed := false

		canAutoUpsert := opts.AutoUpsert && opts.Schedule != nil && len(schedBytes) > 0 && cand.Date != "" && (!needsAI || cand.IsPatch)
		if canAutoUpsert {
			updatedBytes, sessID, uErr := applyUpsertFromCandidate(schedBytes, cand, post)
			switch {
			case uErr == nil:
				if rep, vErr := validate.ValidateBytes(updatedBytes); vErr == nil && rep != nil && rep.IsValid {
					schedBytes = updatedBytes
					scheduleDirty = true
					committed = true
					outcome.Status = "auto_upserted"
					outcome.SessionID = sessID
					report.AutoUpsertedCount++
				} else if vErr != nil {
					stagedErr = vErr.Error()
				} else if rep != nil && len(rep.Errors) > 0 {
					stagedErr = strings.Join(rep.Errors, "; ")
				}
			case errors.Is(uErr, schedule.ErrAmbiguousSession):
				stagedStatus = StatusAmbiguous
				stagedErr = uErr.Error()
			default:
				stagedErr = uErr.Error()
			}
		}

		if !committed {
			if stagedStatus == StatusAmbiguous {
				report.AmbiguousCount++
				outcome.Status = "ambiguous"
			} else {
				report.StagedCount++
				outcome.Status = "staged"
			}
			outcome.Reason = stagedErr

			nowISO := now.UTC().Format(time.RFC3339)
			stateDoc.UpsertPending(PendingItem{
				ID:             fmt.Sprintf("%s:%s", group, post.SourceID),
				Group:          group,
				Status:         stagedStatus,
				CreatedAt:      nowISO,
				UpdatedAt:      nowISO,
				RawFingerprint: fRes.Fingerprint,
				NeedsAI:        needsAI,
				Post:           post,
				Candidate:      cand,
				Evidence:       evidences,
				Error:          stagedErr,
			})
		}

		cp.ProcessedIDs = appendBoundedUnique(cp.ProcessedIDs, post.SourceID, maxTrackedIDs)
		cp.ProcessedFingerprints = appendBoundedUnique(cp.ProcessedFingerprints, fRes.Fingerprint, maxTrackedIDs)
		knownKeys[post.SourceID] = true
		knownKeys[fRes.Fingerprint] = true
		report.Outcomes = append(report.Outcomes, outcome)
	}

	if len(tlRes.FetchErrors) == 0 && tlRes.NewestID != "" {
		report.NewestSinceID = MaxSnowflakeID(cp.LatestSinceID, tlRes.NewestID)
		cp.LatestSinceID = report.NewestSinceID
		cp.LastError = ""
	} else if len(tlRes.FetchErrors) > 0 {
		cp.LastError = fmt.Sprintf("%d URL(s) failed; since_id preserved", len(tlRes.FetchErrors))
	}
	cp.LastRunSource = tlRes.SourceType
	cp.UpdatedAt = now.UTC().Format(time.RFC3339)
	report.CompletedAt = time.Now().UTC().Format(time.RFC3339)

	if len(report.SkippedByStage) == 0 {
		report.SkippedByStage = nil
	}

	if !opts.DryRun {
		if scheduleDirty && opts.Schedule != nil {
			if err := opts.Schedule.WriteSchedule(ctx, group, schedBytes); err != nil {
				return report, fmt.Errorf("writing updated schedule: %w", err)
			}
		}
		if err := stateStore.Save(ctx, stateDoc); err != nil {
			return report, fmt.Errorf("saving scrape state: %w", err)
		}
	}

	return report, nil
}

func mergeAICandidate(base, ai candidate.EventCandidate) candidate.EventCandidate {
	if ai.Title != "" && base.Title == "" {
		base.Title = ai.Title
	}
	if ai.Date != "" {
		base.Date = ai.Date
	}
	if ai.VenueRaw != "" && (base.VenueRaw == "" || base.VenueRaw == "未定") {
		base.VenueRaw = ai.VenueRaw
	}
	if ai.LiveStartRaw != "" {
		base.LiveStartRaw = ai.LiveStartRaw
	}
	if ai.LiveEndRaw != "" {
		base.LiveEndRaw = ai.LiveEndRaw
	}
	if ai.BenefitStartRaw != "" {
		base.BenefitStartRaw = ai.BenefitStartRaw
	}
	if ai.BenefitEndRaw != "" {
		base.BenefitEndRaw = ai.BenefitEndRaw
	}
	if ai.BenefitBoothRaw != "" {
		base.BenefitBoothRaw = ai.BenefitBoothRaw
	}
	if ai.AdmissionBonus != "" && base.AdmissionBonus == "" {
		base.AdmissionBonus = ai.AdmissionBonus
	}
	if ai.TicketURL != "" && base.TicketURL == "" {
		base.TicketURL = ai.TicketURL
	}
	if ai.PhotoSong != "" && base.PhotoSong == "" {
		base.PhotoSong = ai.PhotoSong
	}
	if ai.PhotoSongNumber > 0 && base.PhotoSongNumber == 0 {
		base.PhotoSongNumber = ai.PhotoSongNumber
	}
	if ai.PhotoDevice != "" && base.PhotoDevice == "" {
		base.PhotoDevice = ai.PhotoDevice
	}
	return base
}

func applyUpsertFromCandidate(schedBytes []byte, cand candidate.EventCandidate, post *raw.Post) ([]byte, string, error) {
	photoRule := cand.PhotoNotes
	if photoRule == "" && cand.PhotoSong != "" {
		if cand.PhotoSongNumber > 0 {
			photoRule = fmt.Sprintf("%d曲目『%s』撮影可能", cand.PhotoSongNumber, cand.PhotoSong)
		} else {
			photoRule = fmt.Sprintf("『%s』撮影可能", cand.PhotoSong)
		}
	}
	return schedule.ApplyAddSession(schedBytes, schedule.AddSessionOptions{
		Date:             cand.Date,
		Title:            cand.Title,
		VenueName:        cand.VenueRaw,
		VenueCanonicalID: cand.VenueCanonicalID,
		LiveStartTime:    cand.LiveStartRaw,
		LiveEndTime:      cand.LiveEndRaw,
		BenefitStartTime: cand.BenefitStartRaw,
		BenefitEndTime:   cand.BenefitEndRaw,
		Booth:            cand.BenefitBoothRaw,
		TicketURL:        cand.TicketURL,
		ImageURLs:        post.Images,
		SourceTweetURL:   post.URL,
		AdmissionBonus:   cand.AdmissionBonus,
		PhotoRule:        photoRule,
		PhotoSong:        cand.PhotoSong,
		PhotoSongNumber:  cand.PhotoSongNumber,
		PhotoDevice:      cand.PhotoDevice,
		PhotoAvailable:   cand.PhotoAllowed,
	})
}
