package monitor

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/AobaIwaki123/taite-board/internal/candidate"
	"github.com/AobaIwaki123/taite-board/internal/gcs"
	"github.com/AobaIwaki123/taite-board/internal/parser"
	"github.com/AobaIwaki123/taite-board/internal/raw"
	"github.com/AobaIwaki123/taite-board/internal/reconcile"
	"github.com/AobaIwaki123/taite-board/internal/resolve"
	"github.com/AobaIwaki123/taite-board/internal/schedule"
	"github.com/AobaIwaki123/taite-board/internal/scrape"
	"github.com/AobaIwaki123/taite-board/internal/validate"
)

//go:embed dashboard.html
var DashboardHTML []byte

//go:embed admin.html
var AdminHTML []byte

// HandlerConfig configures optional cloud storage, notifications, and Cloud Run Jobs for the monitor server.
type HandlerConfig struct {
	Store               gcs.ObjectStore
	DataBucket          string
	StateBucket         string
	WebhookURL          string
	NotifyHTTPClient    *http.Client
	NotifySync          bool
	CloudRunJob         string
	CloudRunAPIBase     string
	ScrapeStatePath     string
	TriggerReconcileJob func(ctx context.Context, group string, dryRun bool) (string, error)
}

type validationError struct {
	Errors []string
}

func (e *validationError) Error() string {
	return "schedule validation failed: " + strings.Join(e.Errors, "; ")
}

// PostFetcher defines the interface for retrieving raw post data.
type PostFetcher interface {
	FetchPost(ctx context.Context, postURL string) (*raw.Post, error)
}

// IntakeResolver defines the interface for resolving post text to candidates.
type IntakeResolver interface {
	Resolve(text string, images []string) (candidate.EventCandidate, []candidate.Evidence, bool)
}

// Handler creates an http.Handler that serves the dashboard, API, metrics, and intake endpoint.
func NewHandler(collector *Collector) http.Handler {
	dataBucket := strings.TrimSpace(os.Getenv("GCS_DATA_BUCKET"))
	stateBucket := strings.TrimSpace(os.Getenv("GCS_STATE_BUCKET"))
	webhookURL := strings.TrimSpace(os.Getenv("DISCORD_WEBHOOK_URL"))
	var store gcs.ObjectStore
	if dataBucket != "" || stateBucket != "" {
		if client, err := gcs.NewClientFromEnv(); err == nil && client != nil {
			store = client
			if collector != nil {
				collector.SetCloudStorage(store, dataBucket, stateBucket)
			}
		}
	}
	if collector != nil {
		collector.SetDiscordNotifyEnabled(webhookURL != "")
	}
	return NewHandlerWithConfig(collector, nil, nil, HandlerConfig{
		Store:           store,
		DataBucket:      dataBucket,
		StateBucket:     stateBucket,
		WebhookURL:      webhookURL,
		CloudRunJob:     strings.TrimSpace(os.Getenv("CLOUD_RUN_RECONCILE_JOB")),
		CloudRunAPIBase: strings.TrimSpace(os.Getenv("CLOUD_RUN_API_BASE")),
	})
}

// NewHandlerWithDependencies provides dependency injection for testing in local filesystem mode.
func NewHandlerWithDependencies(collector *Collector, fetcher PostFetcher, resolver IntakeResolver) http.Handler {
	return NewHandlerWithConfig(collector, fetcher, resolver, HandlerConfig{})
}

// NewHandlerWithConfig provides full dependency and storage injection for testing and production.
func NewHandlerWithConfig(collector *Collector, fetcher PostFetcher, resolver IntakeResolver, cfg HandlerConfig) http.Handler {
	if fetcher == nil {
		fetcher = newDefaultSourceFetcher()
	}
	if resolver == nil {
		resolver, _ = resolve.NewResolver("")
	}
	if collector != nil {
		collector.SetDiscordNotifyEnabled(strings.TrimSpace(cfg.WebhookURL) != "")
	}
	mux := http.NewServeMux()

	// 1. Dashboard UI (observability & pipeline monitoring)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(DashboardHTML)
	})

	// 1.5 Admin Registration & Operations UI (/register and /admin for Tailscale access)
	serveAdmin := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(AdminHTML)
	}
	mux.HandleFunc("/register", serveAdmin)
	mux.HandleFunc("/register/", serveAdmin)
	mux.HandleFunc("/admin", serveAdmin)
	mux.HandleFunc("/admin/", serveAdmin)

	// 2. JSON Stats API for Dashboard
	mux.HandleFunc("/api/stats", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.WriteHeader(http.StatusOK)
		snap := collector.Snapshot()
		_ = json.NewEncoder(w).Encode(snap)
	})

	// 3. Prometheus /metrics Endpoint
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		metrics := collector.PrometheusMetrics()
		fmt.Fprint(w, metrics)
	})

	// 4. Health check
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})

	// 5. Intake API (Fetch & Resolve X Post)
	type intakeRequest struct {
		URL string `json:"url"`
	}

	mux.HandleFunc("/api/intake", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req intakeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json body", http.StatusBadRequest)
			return
		}

		targetURL := strings.TrimSpace(req.URL)
		if targetURL == "" {
			http.Error(w, "url is required", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()

		post, err := fetcher.FetchPost(ctx, targetURL)
		if err != nil {
			errMsg := fmt.Sprintf("failed to fetch post: %v", err)
			recordAndNotifyFailure(collector, cfg, "intake_error", "ribonclock", "", "", "", targetURL, errMsg)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnprocessableEntity)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": errMsg,
			})
			return
		}

		cand, evidences, needsAI := resolver.Resolve(post.Text, post.Images)
		fp := candidate.ComputeRawFingerprint(post.Source, post.SourceID, post.Text, post.Images)

		status := candidate.StatusSkippedDeterministic
		if (cand.Date != "" && cand.LiveStartRaw != "") || cand.IsPatch {
			status = candidate.StatusCompleted
		} else if needsAI {
			status = candidate.StatusNeedsReview
		}

		result := candidate.IntakeResult{
			Source:         post.Source,
			SourceID:       post.SourceID,
			URL:            post.URL,
			Author:         post.Author,
			Text:           post.Text,
			Images:         post.Images,
			RawFingerprint: fp,
			NeedsAI:        needsAI,
			Status:         status,
			Candidate:      cand,
			Evidence:       evidences,
		}

		collector.AddIntake(result)

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(result)
	})

	// 5.5 Sessions list API (for selecting patch target session on mobile)
	mux.HandleFunc("/api/sessions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		group := normalizeGroupName(r.URL.Query().Get("group"))
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()

		rawBytes, _, err := readGroupSchedule(ctx, cfg, group)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("reading schedule for %s: %v", group, err))
			return
		}

		var doc struct {
			CurrentNextID string `json:"current_next_id"`
			Sessions      []struct {
				ID    string `json:"id"`
				Date  string `json:"date"`
				Title string `json:"title"`
				Venue struct {
					Name string `json:"name"`
				} `json:"venue"`
				Live struct {
					StartTime string `json:"start_time"`
					EndTime   string `json:"end_time"`
				} `json:"live"`
				Benefit *struct {
					StartTime string `json:"start_time"`
					EndTime   string `json:"end_time"`
					Booth     string `json:"booth"`
				} `json:"benefit"`
				AdmissionBonus string `json:"admission_bonus"`
				Photo          *struct {
					Rule string `json:"rule"`
				} `json:"photo"`
			} `json:"sessions"`
		}
		if err := json.Unmarshal(rawBytes, &doc); err != nil {
			writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("parsing schedule json: %v", err))
			return
		}

		type sessionSummary struct {
			ID             string `json:"id"`
			Date           string `json:"date"`
			Title          string `json:"title"`
			Venue          string `json:"venue"`
			LiveStart      string `json:"live_start"`
			LiveEnd        string `json:"live_end"`
			BenefitStart   string `json:"benefit_start"`
			BenefitEnd     string `json:"benefit_end"`
			AdmissionBonus string `json:"admission_bonus,omitempty"`
			PhotoRule      string `json:"photo_rule,omitempty"`
		}
		items := make([]sessionSummary, 0, len(doc.Sessions))
		for _, s := range doc.Sessions {
			item := sessionSummary{
				ID:             s.ID,
				Date:           s.Date,
				Title:          s.Title,
				Venue:          s.Venue.Name,
				LiveStart:      s.Live.StartTime,
				LiveEnd:        s.Live.EndTime,
				AdmissionBonus: s.AdmissionBonus,
			}
			if s.Benefit != nil {
				item.BenefitStart = s.Benefit.StartTime
				item.BenefitEnd = s.Benefit.EndTime
			}
			if s.Photo != nil {
				item.PhotoRule = s.Photo.Rule
			}
			items = append(items, item)
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"group":           group,
			"current_next_id": doc.CurrentNextID,
			"storage":         storageModeLabel(cfg),
			"sessions":        items,
		})
	})

	// 6. Intake Promote API (Commit candidate to GCS or local schedule.json)
	type promoteRequest struct {
		TargetSessionID string `json:"target_session_id"`
		SessionName     string `json:"session_name"`
		Group           string `json:"group"`
		SourceID        string `json:"source_id"`
		SourceURL       string `json:"source_url"`
		Author          string `json:"author"`
		Title           string `json:"title"`
		Date            string `json:"date"`
		Venue           string `json:"venue"`
		LiveStart       string `json:"live_start"`
		LiveEnd         string `json:"live_end"`
		BenefitStart    string `json:"benefit_start"`
		BenefitEnd      string `json:"benefit_end"`
		Booth           string `json:"booth"`
		TicketURL       string `json:"ticket_url"`
		ImageURL        string `json:"image_url"`
		AdmissionBonus  string `json:"admission_bonus"`
		SpecialNotes    string `json:"special_notes"`
		PhotoSong       string `json:"photo_song"`
		PhotoSongNumber int    `json:"photo_song_number"`
		PhotoDevice     string `json:"photo_device"`
		PhotoRule       string `json:"photo_rule"`
	}

	mux.HandleFunc("/api/intake/promote", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req promoteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, fmt.Sprintf("invalid json body: %v", err), http.StatusBadRequest)
			return
		}

		group := normalizeGroupName(req.Group)
		title := strings.TrimSpace(req.Title)
		title = strings.Trim(title, "🎀✨🎂🤍🕰️ \t")
		title = strings.NewReplacer("『", "", "』", "", "｢", "", "｣", "").Replace(title)
		title = strings.TrimSpace(title)
		if title == "" {
			title = fmt.Sprintf("ライブ出演 (%s)", req.Date)
		}
		venue := strings.TrimSpace(req.Venue)
		venueCanonicalID := ""
		if venue == "" {
			venue = "未定"
		} else if vr, rErr := resolve.NewResolver(""); rErr == nil && vr != nil {
			cid, display := vr.ResolveVenueName(venue)
			if cid != "" {
				venueCanonicalID = cid
			}
			if display != "" {
				venue = display
			}
		}

		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()

		currentBytes, _, err := readGroupSchedule(ctx, cfg, group)
		if err != nil {
			errMsg := fmt.Sprintf("reading schedule: %v", err)
			recordAndNotifyFailure(collector, cfg, "promote_error", group, strings.TrimSpace(req.Date), title, venue, strings.TrimSpace(req.SourceURL), errMsg)
			writeJSONError(w, http.StatusInternalServerError, errMsg)
			return
		}

		updatedBytes, sessionID, err := schedule.ApplyAddSession(currentBytes, schedule.AddSessionOptions{
			TargetSessionID:  strings.TrimSpace(req.TargetSessionID),
			SessionName:      strings.TrimSpace(req.SessionName),
			Date:             strings.TrimSpace(req.Date),
			Title:            title,
			VenueName:        venue,
			VenueCanonicalID: venueCanonicalID,
			LiveStartTime:    strings.TrimSpace(req.LiveStart),
			LiveEndTime:      strings.TrimSpace(req.LiveEnd),
			BenefitStartTime: strings.TrimSpace(req.BenefitStart),
			BenefitEndTime:   strings.TrimSpace(req.BenefitEnd),
			Booth:            strings.TrimSpace(req.Booth),
			TicketURL:        strings.TrimSpace(req.TicketURL),
			ImageURL:         strings.TrimSpace(req.ImageURL),
			SourceTweetURL:   strings.TrimSpace(req.SourceURL),
			SpecialNotes:     strings.TrimSpace(req.SpecialNotes),
			AdmissionBonus:   strings.TrimSpace(req.AdmissionBonus),
			PhotoSong:        strings.TrimSpace(req.PhotoSong),
			PhotoSongNumber:  req.PhotoSongNumber,
			PhotoDevice:      strings.TrimSpace(req.PhotoDevice),
			PhotoRule:        strings.TrimSpace(req.PhotoRule),
		})
		if err != nil {
			errMsg := fmt.Sprintf("failed to add session: %v", err)
			recordAndNotifyFailure(collector, cfg, "promote_error", group, strings.TrimSpace(req.Date), title, venue, strings.TrimSpace(req.SourceURL), errMsg)
			status := http.StatusUnprocessableEntity
			if errors.Is(err, schedule.ErrAmbiguousSession) {
				status = http.StatusConflict
			}
			writeJSONError(w, status, errMsg)
			return
		}
		var previous struct {
			Sessions []struct {
				ID string `json:"id"`
			} `json:"sessions"`
		}
		if err := json.Unmarshal(currentBytes, &previous); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "invalid existing schedule")
			return
		}
		action := "created"
		for _, session := range previous.Sessions {
			if session.ID == sessionID {
				action = "merged"
				break
			}
		}

		if err := writeGroupSchedule(ctx, cfg, group, updatedBytes); err != nil {
			var vErr *validationError
			if errors.As(err, &vErr) {
				recordAndNotifyFailure(collector, cfg, "promote_error", group, strings.TrimSpace(req.Date), title, venue, strings.TrimSpace(req.SourceURL), vErr.Error())
				writeJSONError(w, http.StatusUnprocessableEntity, vErr.Error())
				return
			}
			errMsg := fmt.Sprintf("saving schedule: %v", err)
			recordAndNotifyFailure(collector, cfg, "promote_error", group, strings.TrimSpace(req.Date), title, venue, strings.TrimSpace(req.SourceURL), errMsg)
			writeJSONError(w, http.StatusInternalServerError, errMsg)
			return
		}

		if collector != nil {
			collector.IncrementPromoted()
		}

		if sid := strings.TrimSpace(req.SourceID); sid != "" && (cfg.ScrapeStatePath != "" || (cfg.Store != nil && cfg.StateBucket != "")) {
			stStore := scrapeStateStore(cfg)
			if stDoc, stErr := stStore.Load(ctx); stErr == nil && stDoc != nil {
				if stDoc.UpdatePendingStatus(fmt.Sprintf("%s:%s", group, sid), scrape.StatusPromoted) {
					_ = stStore.Save(ctx, stDoc)
				}
			}
		}

		domain := fmt.Sprintf("%s.iorin.net", group)
		summaryParts := []string{}
		if strings.TrimSpace(req.LiveStart) != "" {
			t := strings.TrimSpace(req.LiveStart)
			if strings.TrimSpace(req.LiveEnd) != "" {
				t += "-" + strings.TrimSpace(req.LiveEnd)
			}
			summaryParts = append(summaryParts, "出演 "+t)
		}
		if strings.TrimSpace(req.BenefitStart) != "" {
			b := strings.TrimSpace(req.BenefitStart)
			if strings.TrimSpace(req.BenefitEnd) != "" {
				b += "-" + strings.TrimSpace(req.BenefitEnd)
			}
			summaryParts = append(summaryParts, "特典会 "+b)
		}
		if strings.TrimSpace(req.AdmissionBonus) != "" {
			summaryParts = append(summaryParts, "入場特典: "+strings.TrimSpace(req.AdmissionBonus))
		}
		if strings.TrimSpace(req.PhotoSong) != "" {
			summaryParts = append(summaryParts, "撮可: "+strings.TrimSpace(req.PhotoSong))
		} else if strings.TrimSpace(req.PhotoRule) != "" {
			summaryParts = append(summaryParts, strings.TrimSpace(req.PhotoRule))
		}
		act := recordAndNotifyActivity(collector, cfg, ActivityEntry{
			ID:        fmt.Sprintf("act-%d", time.Now().UnixNano()),
			Timestamp: time.Now().In(time.FixedZone("Asia/Tokyo", 9*60*60)).Format(time.RFC3339),
			Action:    "promote",
			Group:     group,
			SessionID: sessionID,
			Date:      strings.TrimSpace(req.Date),
			Title:     title,
			Venue:     venue,
			Summary:   strings.Join(summaryParts, " / "),
			SourceURL: strings.TrimSpace(req.SourceURL),
			Domain:    domain,
		})

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":     "success",
			"action":     action,
			"session_id": sessionID,
			"group":      group,
			"domain":     domain,
			"storage":    storageModeLabel(cfg),
			"bucket":     cfg.DataBucket,
			"activity":   act,
			"message":    fmt.Sprintf("Session %s successfully committed to %s (%s)", sessionID, domain, storageModeLabel(cfg)),
		})
	})

	// 7. Intake Patch API (Update perks, photo rules, and special notes on an existing session)
	type patchRequest struct {
		Group             string `json:"group"`
		SessionID         string `json:"session_id"`
		Date              string `json:"date"`
		AdmissionBonus    string `json:"admission_bonus"`
		PhotoSong         string `json:"photo_song"`
		PhotoSongNumber   int    `json:"photo_song_number"`
		PhotoDevice       string `json:"photo_device"`
		PhotoRule         string `json:"photo_rule"`
		SpecialNotes      string `json:"special_notes"`
		SourceURL         string `json:"source_url"`
		UpdateOfficialURL bool   `json:"update_official_url"`
	}

	mux.HandleFunc("/api/intake/patch", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req patchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, fmt.Sprintf("invalid json body: %v", err), http.StatusBadRequest)
			return
		}

		group := normalizeGroupName(req.Group)
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()

		rawBytes, _, err := readGroupSchedule(ctx, cfg, group)
		if err != nil {
			errMsg := fmt.Sprintf("reading schedule: %v", err)
			recordAndNotifyFailure(collector, cfg, "patch_error", group, strings.TrimSpace(req.Date), strings.TrimSpace(req.SessionID), "", strings.TrimSpace(req.SourceURL), errMsg)
			writeJSONError(w, http.StatusInternalServerError, errMsg)
			return
		}

		// Resolve target Session ID (and capture metadata for activity notification)
		var schedDoc struct {
			CurrentNextID string `json:"current_next_id"`
			Sessions      []struct {
				ID    string `json:"id"`
				Date  string `json:"date"`
				Title string `json:"title"`
				Venue struct {
					Name string `json:"name"`
				} `json:"venue"`
			} `json:"sessions"`
		}
		_ = json.Unmarshal(rawBytes, &schedDoc)

		sessionID := strings.TrimSpace(req.SessionID)
		if sessionID == "" {
			if req.Date != "" {
				matches := 0
				for _, s := range schedDoc.Sessions {
					if s.Date == req.Date {
						sessionID = s.ID
						matches++
					}
				}
				if matches > 1 {
					writeJSONError(w, http.StatusConflict, "ambiguous sessions; specify session_id")
					return
				}
				if matches == 0 {
					writeJSONError(w, http.StatusNotFound, "target session not found on date")
					return
				}
			}
			if sessionID == "" && schedDoc.CurrentNextID != "" {
				sessionID = schedDoc.CurrentNextID
			}
			if sessionID == "" && len(schedDoc.Sessions) > 0 {
				sessionID = schedDoc.Sessions[0].ID
			}
		}

		if sessionID == "" {
			errMsg := "target session not found in schedule"
			recordAndNotifyFailure(collector, cfg, "patch_error", group, strings.TrimSpace(req.Date), "", "", strings.TrimSpace(req.SourceURL), errMsg)
			writeJSONError(w, http.StatusNotFound, errMsg)
			return
		}

		var targetDate, targetTitle, targetVenue string
		for _, s := range schedDoc.Sessions {
			if s.ID == sessionID {
				targetDate = s.Date
				targetTitle = s.Title
				targetVenue = s.Venue.Name
				break
			}
		}

		photoRule := strings.TrimSpace(req.PhotoRule)
		var photoAvailable *bool
		if photoRule != "" || strings.TrimSpace(req.PhotoSong) != "" || req.PhotoSongNumber > 0 {
			t := true
			photoAvailable = &t
		}

		updatedBytes, err := schedule.ApplyUpdateSession(rawBytes, schedule.UpdateSessionOptions{
			SessionID:         sessionID,
			PhotoRule:         photoRule,
			PhotoSong:         strings.TrimSpace(req.PhotoSong),
			PhotoSongNumber:   req.PhotoSongNumber,
			PhotoDevice:       strings.TrimSpace(req.PhotoDevice),
			PhotoAvailable:    photoAvailable,
			AdmissionBonus:    strings.TrimSpace(req.AdmissionBonus),
			SpecialNotes:      strings.TrimSpace(req.SpecialNotes),
			SourceTweetURL:    strings.TrimSpace(req.SourceURL),
			UpdateOfficialURL: req.UpdateOfficialURL,
		})
		if err != nil {
			errMsg := fmt.Sprintf("failed to update session: %v", err)
			recordAndNotifyFailure(collector, cfg, "patch_error", group, targetDate, targetTitle, targetVenue, strings.TrimSpace(req.SourceURL), errMsg)
			writeJSONError(w, http.StatusUnprocessableEntity, errMsg)
			return
		}

		if err := writeGroupSchedule(ctx, cfg, group, updatedBytes); err != nil {
			var vErr *validationError
			if errors.As(err, &vErr) {
				recordAndNotifyFailure(collector, cfg, "patch_error", group, targetDate, targetTitle, targetVenue, strings.TrimSpace(req.SourceURL), vErr.Error())
				writeJSONError(w, http.StatusUnprocessableEntity, vErr.Error())
				return
			}
			errMsg := fmt.Sprintf("saving updated schedule: %v", err)
			recordAndNotifyFailure(collector, cfg, "patch_error", group, targetDate, targetTitle, targetVenue, strings.TrimSpace(req.SourceURL), errMsg)
			writeJSONError(w, http.StatusInternalServerError, errMsg)
			return
		}

		domain := fmt.Sprintf("%s.iorin.net", group)
		patchSummary := []string{}
		if strings.TrimSpace(req.AdmissionBonus) != "" {
			patchSummary = append(patchSummary, "入場特典: "+strings.TrimSpace(req.AdmissionBonus))
		}
		if photoRule != "" {
			patchSummary = append(patchSummary, photoRule)
		}
		if strings.TrimSpace(req.SpecialNotes) != "" {
			patchSummary = append(patchSummary, "メモ: "+strings.TrimSpace(req.SpecialNotes))
		}
		act := recordAndNotifyActivity(collector, cfg, ActivityEntry{
			ID:        fmt.Sprintf("act-%d", time.Now().UnixNano()),
			Timestamp: time.Now().In(time.FixedZone("Asia/Tokyo", 9*60*60)).Format(time.RFC3339),
			Action:    "patch",
			Group:     group,
			SessionID: sessionID,
			Date:      targetDate,
			Title:     targetTitle,
			Venue:     targetVenue,
			Summary:   strings.Join(patchSummary, " / "),
			SourceURL: strings.TrimSpace(req.SourceURL),
			Domain:    domain,
		})

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":     "success",
			"session_id": sessionID,
			"group":      group,
			"domain":     domain,
			"storage":    storageModeLabel(cfg),
			"bucket":     cfg.DataBucket,
			"activity":   act,
			"message":    fmt.Sprintf("Perks & photo regulations updated on session %s (%s)", sessionID, domain),
		})
	})

	// 6. AI Parser Proxy (forwards to Cloud Run)
	type aiParseRequest struct {
		SourceID string   `json:"source_id"`
		Text     string   `json:"text"`
		Images   []string `json:"images"`
	}

	mux.HandleFunc("/api/intake/ai-parse", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req aiParseRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json body", http.StatusBadRequest)
			return
		}

		parserURL := os.Getenv("AI_PARSER_URL")
		if parserURL == "" {
			// Dry-run / mock mode when Cloud Run URL is not configured
			mockResp := parser.ParseResponse{
				RequestID: fmt.Sprintf("mock-%d", time.Now().UnixNano()),
				Status:    candidate.StatusCompleted,
				ModelID:   "mock-parser",
				Candidate: candidate.EventCandidate{
					LiveStartRaw:    "18:30",
					LiveEndRaw:      "19:00",
					BenefitStartRaw: "19:15",
					BenefitEndRaw:   "20:15",
				},
				Evidence: []candidate.Evidence{
					{
						Field:      "live_start_raw",
						Source:     "image",
						Quote:      "18:30-19:00 リボンクロック",
						Confidence: 0.95,
					},
				},
			}
			collector.UpdateIntakeAIResult(req.SourceID, mockResp.Candidate, mockResp.Evidence, mockResp.ModelID)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(mockResp)
			return
		}

		// Forward to Cloud Run AI Parser
		var media []parser.MediaItem
		for _, img := range req.Images {
			media = append(media, parser.MediaItem{
				InputURL: img,
			})
		}

		parseReq := parser.ParseRequest{
			RequestID: fmt.Sprintf("k8s-%d", time.Now().UnixNano()),
			Source:    "x",
			SourceID:  req.SourceID,
			Text:      req.Text,
			Media:     media,
		}

		reqBody, _ := json.Marshal(parseReq)
		httpReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, strings.TrimRight(parserURL, "/")+"/parse", bytes.NewReader(reqBody))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")
		if token := os.Getenv("AI_PARSER_TOKEN"); token != "" {
			httpReq.Header.Set("Authorization", "Bearer "+token)
		}

		client := &http.Client{Timeout: 35 * time.Second}
		resp, err := client.Do(httpReq)
		if err != nil {
			http.Error(w, fmt.Sprintf("calling cloud run parser: %v", err), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		respBytes, err := io.ReadAll(resp.Body)
		if err == nil && resp.StatusCode == http.StatusOK {
			var parseResp parser.ParseResponse
			if err := json.Unmarshal(respBytes, &parseResp); err == nil && parseResp.Status == candidate.StatusCompleted {
				collector.UpdateIntakeAIResult(req.SourceID, parseResp.Candidate, parseResp.Evidence, parseResp.ModelID)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(respBytes)
	})

	// 7. Test Discord Notification Endpoint & Process Start Notification Endpoint
	mux.HandleFunc("/api/notify/start", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Action    string `json:"action"`
			Group     string `json:"group"`
			Title     string `json:"title"`
			Summary   string `json:"summary"`
			SourceURL string `json:"source_url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json body", http.StatusBadRequest)
			return
		}
		action := strings.TrimSpace(req.Action)
		if action != "intake_start" && action != "promote_start" {
			action = "intake_start"
		}
		group := normalizeGroupName(req.Group)
		act := recordAndNotifyActivity(collector, cfg, ActivityEntry{
			ID:        fmt.Sprintf("act-%d", time.Now().UnixNano()),
			Timestamp: time.Now().In(time.FixedZone("Asia/Tokyo", 9*60*60)).Format(time.RFC3339),
			Action:    action,
			Group:     group,
			Title:     strings.TrimSpace(req.Title),
			Summary:   strings.TrimSpace(req.Summary),
			SourceURL: strings.TrimSpace(req.SourceURL),
			Domain:    fmt.Sprintf("%s.iorin.net", group),
		})
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":   "ok",
			"activity": act,
		})
	})

	mux.HandleFunc("/api/notify/test", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		webhookURL := strings.TrimSpace(cfg.WebhookURL)
		if webhookURL == "" {
			writeJSONError(w, http.StatusBadRequest, "DISCORD_WEBHOOK_URL が設定されていません")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		msg := "**[Ribbon Clock Admin] テスト通知**\n管理画面からの通知連携テストです。ライブ追加・更新時にこのチャンネルへ通知されます。"
		if err := SendDiscordWebhook(ctx, cfg.NotifyHTTPClient, webhookURL, msg); err != nil {
			writeJSONError(w, http.StatusBadGateway, fmt.Sprintf("Discord 通知エラー: %v", err))
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":  "sent",
			"message": "Discord へテスト通知を送信しました。",
		})
	})

	// 8. On-demand Entity Resolution / Reconcile Cloud Run Job trigger
	mux.HandleFunc("/api/reconcile", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Group  string `json:"group"`
			DryRun bool   `json:"dry_run"`
		}
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&req)
		}
		group := strings.TrimSpace(strings.ToLower(req.Group))
		if group == "" {
			group = "all"
		} else if group != "all" {
			group = normalizeGroupName(group)
		}

		modeLabel := "本番反映"
		if req.DryRun {
			modeLabel = "Dry Run"
		}

		recordAndNotifyActivity(collector, cfg, ActivityEntry{
			ID:        fmt.Sprintf("act-%d", time.Now().UnixNano()),
			Timestamp: time.Now().In(time.FixedZone("Asia/Tokyo", 9*60*60)).Format(time.RFC3339),
			Action:    "reconcile_start",
			Group:     group,
			Title:     "名寄せ・重複整理ジョブ",
			Summary:   fmt.Sprintf("ジョブ起動 (%s)", modeLabel),
			Domain:    fmt.Sprintf("%s.iorin.net", normalizeGroupName(group)),
		})

		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		// Path A: Cloud Run Job trigger (custom hook or real GCP Cloud Run Jobs v2 API)
		if cfg.TriggerReconcileJob != nil || strings.TrimSpace(cfg.CloudRunJob) != "" {
			var execName string
			var err error
			if cfg.TriggerReconcileJob != nil {
				execName, err = cfg.TriggerReconcileJob(ctx, group, req.DryRun)
			} else {
				execName, err = defaultTriggerCloudRunJob(ctx, cfg, group, req.DryRun)
			}
			if err != nil {
				errMsg := err.Error()
				recordAndNotifyFailure(collector, cfg, "reconcile_error", group, "", "名寄せ・重複整理ジョブ", "", "", errMsg)
				writeJSONError(w, http.StatusBadGateway, errMsg)
				return
			}

			act := recordAndNotifyActivity(collector, cfg, ActivityEntry{
				ID:        fmt.Sprintf("act-%d", time.Now().UnixNano()),
				Timestamp: time.Now().In(time.FixedZone("Asia/Tokyo", 9*60*60)).Format(time.RFC3339),
				Action:    "reconcile",
				Group:     group,
				Title:     "名寄せ・重複整理ジョブ",
				Summary:   fmt.Sprintf("Cloud Run Job 実行開始: %s (%s)", execName, modeLabel),
				Domain:    fmt.Sprintf("%s.iorin.net", normalizeGroupName(group)),
			})
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status":    "triggered",
				"mode":      "cloud_run_job",
				"group":     group,
				"dry_run":   req.DryRun,
				"execution": execName,
				"activity":  act,
				"message":   fmt.Sprintf("Cloud Run Job を起動しました (%s)", execName),
			})
			return
		}

		// Path B: In-process GCS fallback when CLOUD_RUN_RECONCILE_JOB is unset
		if cfg.Store != nil && cfg.DataBucket != "" {
			summary, err := reconcile.Run(ctx, reconcile.RunOptions{
				Store:         cfg.Store,
				Bucket:        cfg.DataBucket,
				Group:         group,
				DryRun:        req.DryRun,
				TriggerSource: "monitor-inline",
			})
			if err != nil {
				errMsg := err.Error()
				recordAndNotifyFailure(collector, cfg, "reconcile_error", group, "", "名寄せ・重複整理ジョブ", "", "", errMsg)
				writeJSONError(w, http.StatusInternalServerError, errMsg)
				return
			}
			act := recordAndNotifyActivity(collector, cfg, ActivityEntry{
				ID:        fmt.Sprintf("act-%d", time.Now().UnixNano()),
				Timestamp: time.Now().In(time.FixedZone("Asia/Tokyo", 9*60*60)).Format(time.RFC3339),
				Action:    "reconcile",
				Group:     group,
				Title:     "名寄せ・重複整理 (インライン実行)",
				Summary:   fmt.Sprintf("changed=%t / %d グループ (%s)", summary.AnyChanged, len(summary.Results), modeLabel),
				Domain:    fmt.Sprintf("%s.iorin.net", normalizeGroupName(group)),
			})
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status":   "completed",
				"mode":     "inline",
				"summary":  summary,
				"activity": act,
				"message":  "名寄せ・重複整理を実行しました。",
			})
			return
		}

		errMsg := "CLOUD_RUN_RECONCILE_JOB または GCS ストアが設定されていません"
		recordAndNotifyFailure(collector, cfg, "reconcile_error", group, "", "名寄せ・重複整理ジョブ", "", "", errMsg)
		writeJSONError(w, http.StatusServiceUnavailable, errMsg)
	})

	// 9. Scrape pipeline pending queue (GET list, DELETE dismiss)
	mux.HandleFunc("/api/scrape/pending", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		stStore := scrapeStateStore(cfg)

		switch r.Method {
		case http.MethodGet:
			doc, err := stStore.Load(ctx)
			if err != nil {
				writeJSONError(w, http.StatusInternalServerError, err.Error())
				return
			}
			groupFilter := strings.TrimSpace(r.URL.Query().Get("group"))
			if groupFilter != "" {
				groupFilter = normalizeGroupName(groupFilter)
			}
			items := make([]scrape.PendingItem, 0)
			for _, item := range doc.PendingQueue {
				if item.Status != scrape.StatusPending && item.Status != scrape.StatusAmbiguous {
					continue
				}
				if groupFilter != "" && item.Group != groupFilter {
					continue
				}
				items = append(items, item)
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"items": items,
			})
		case http.MethodDelete:
			id := strings.TrimSpace(r.URL.Query().Get("id"))
			if id == "" {
				writeJSONError(w, http.StatusBadRequest, "id query parameter is required")
				return
			}
			doc, err := stStore.Load(ctx)
			if err != nil {
				writeJSONError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if !doc.UpdatePendingStatus(id, scrape.StatusDismissed) {
				writeJSONError(w, http.StatusNotFound, "pending item not found")
				return
			}
			if err := stStore.Save(ctx, doc); err != nil {
				writeJSONError(w, http.StatusInternalServerError, err.Error())
				return
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			_ = json.NewEncoder(w).Encode(map[string]string{
				"status": "dismissed",
				"id":     id,
			})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// 10. Scrape pipeline execution (mock URLs, fixture file, or official X API v2)
	mux.HandleFunc("/api/scrape/run", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Group            string   `json:"group"`
			URLs             []string `json:"urls"`
			FixturePath      string   `json:"fixture_path"`
			UseXAPI          bool     `json:"use_x_api"`
			UserID           string   `json:"user_id"`
			SinceID          string   `json:"since_id"`
			IgnoreCheckpoint bool     `json:"ignore_checkpoint"`
			MaxAgeHours      int      `json:"max_age_hours"`
			RequireMedia     *bool    `json:"require_media"`
			AutoUpsert       bool     `json:"auto_upsert"`
			DryRun           bool     `json:"dry_run"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid json body: %v", err))
			return
		}

		group := normalizeGroupName(req.Group)
		var src scrape.TimelineSource
		switch {
		case req.UseXAPI:
			token := strings.TrimSpace(os.Getenv("X_BEARER_TOKEN"))
			src = scrape.NewXAPISource(token, 15*time.Second)
		case strings.TrimSpace(req.FixturePath) != "":
			src = &scrape.FixtureSource{Path: strings.TrimSpace(req.FixturePath)}
		case len(req.URLs) > 0:
			src = &scrape.URLListSource{URLs: req.URLs, Fetcher: fetcher, Timeout: 12 * time.Second, Workers: 4}
		default:
			writeJSONError(w, http.StatusBadRequest, "urls, fixture_path, or use_x_api=true is required")
			return
		}

		var aiClient scrape.AIParserClient
		if parserURL := strings.TrimSpace(os.Getenv("AI_PARSER_URL")); parserURL != "" {
			aiClient = &scrape.HTTPAIParserClient{
				BaseURL: parserURL,
				Token:   strings.TrimSpace(os.Getenv("AI_PARSER_TOKEN")),
			}
		}

		runner, err := scrape.NewRunner("")
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
		defer cancel()

		maxAge := req.MaxAgeHours
		if maxAge == 0 {
			maxAge = -1
		}

		report, err := runner.Run(ctx, scrape.RunOptions{
			Group:            group,
			UserID:           strings.TrimSpace(req.UserID),
			SinceID:          strings.TrimSpace(req.SinceID),
			IgnoreCheckpoint: req.IgnoreCheckpoint,
			RequireMedia:     req.RequireMedia,
			MaxAgeHours:      maxAge,
			AutoUpsert:       req.AutoUpsert,
			DryRun:           req.DryRun,
			Source:           src,
			State:            scrapeStateStore(cfg),
			Schedule:         &monitorScheduleAccessor{cfg: cfg},
			AIParser:         aiClient,
		})
		if err != nil {
			writeJSONError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(report)
	})

	return mux
}

type monitorScheduleAccessor struct {
	cfg HandlerConfig
}

func (a *monitorScheduleAccessor) ReadSchedule(ctx context.Context, group string) ([]byte, error) {
	data, _, err := readGroupSchedule(ctx, a.cfg, group)
	return data, err
}

func (a *monitorScheduleAccessor) WriteSchedule(ctx context.Context, group string, data []byte) error {
	return writeGroupSchedule(ctx, a.cfg, group, data)
}

func scrapeStateStore(cfg HandlerConfig) scrape.StateStore {
	if cfg.Store != nil && cfg.StateBucket != "" {
		return &scrape.GCSStateStore{
			Store:  cfg.Store,
			Bucket: cfg.StateBucket,
			Object: scrape.DefaultGCSStateObject,
		}
	}
	path := strings.TrimSpace(cfg.ScrapeStatePath)
	if path == "" {
		if _, err := os.Stat("/var/log/ribonclock"); err == nil {
			path = "/var/log/ribonclock/scrape_state.json"
		} else {
			path = "data/scrape_state.json"
		}
	}
	return &scrape.FileStateStore{Path: path}
}

func defaultTriggerCloudRunJob(ctx context.Context, cfg HandlerConfig, group string, dryRun bool) (string, error) {
	jobName := strings.TrimSpace(cfg.CloudRunJob)
	client, ok := cfg.Store.(*gcs.Client)
	if !ok || client == nil {
		var err error
		client, err = gcs.NewClientFromEnvOrMetadata(ctx)
		if err != nil {
			return "", fmt.Errorf("initializing GCP credentials: %w", err)
		}
	}
	token, err := client.TokenForScope(ctx, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		return "", fmt.Errorf("acquiring GCP token for Cloud Run Job: %w", err)
	}

	apiBase := strings.TrimRight(strings.TrimSpace(cfg.CloudRunAPIBase), "/")
	if apiBase == "" {
		apiBase = "https://run.googleapis.com"
	}
	endpoint := fmt.Sprintf("%s/v2/%s:run", apiBase, strings.TrimPrefix(jobName, "/"))

	payload := map[string]any{
		"overrides": map[string]any{
			"containerOverrides": []map[string]any{
				{
					"env": []map[string]string{
						{"name": "TARGET_GROUP", "value": group},
						{"name": "DRY_RUN", "value": fmt.Sprintf("%t", dryRun)},
						{"name": "TRIGGER_SOURCE", "value": "admin-ui"},
					},
				},
			},
		},
	}
	bodyBytes, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("building Cloud Run Job request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	httpClient := cfg.NotifyHTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 20 * time.Second}
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("invoking Cloud Run Job: %w", err)
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("Cloud Run Job API returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBytes)))
	}

	var op struct {
		Name     string `json:"name"`
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
	}
	_ = json.Unmarshal(respBytes, &op)
	if op.Metadata.Name != "" {
		return op.Metadata.Name, nil
	}
	if op.Name != "" {
		return op.Name, nil
	}
	return jobName, nil
}

func recordAndNotifyFailure(collector *Collector, cfg HandlerConfig, action, group, date, title, venue, sourceURL, errMsg string) ActivityEntry {
	return recordAndNotifyActivity(collector, cfg, ActivityEntry{
		ID:        fmt.Sprintf("act-%d", time.Now().UnixNano()),
		Timestamp: time.Now().In(time.FixedZone("Asia/Tokyo", 9*60*60)).Format(time.RFC3339),
		Action:    action,
		Group:     normalizeGroupName(group),
		Date:      date,
		Title:     title,
		Venue:     venue,
		Error:     errMsg,
		SourceURL: sourceURL,
		Domain:    fmt.Sprintf("%s.iorin.net", normalizeGroupName(group)),
	})
}

func recordAndNotifyActivity(collector *Collector, cfg HandlerConfig, entry ActivityEntry) ActivityEntry {
	webhookURL := strings.TrimSpace(cfg.WebhookURL)
	if webhookURL == "" {
		entry.NotificationStatus = "skipped_unconfigured"
		if collector != nil {
			collector.RecordActivity(entry)
		}
		return entry
	}

	dispatch := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		msg := FormatDiscordNotification(entry)
		err := SendDiscordWebhook(ctx, cfg.NotifyHTTPClient, webhookURL, msg)
		if err != nil {
			entry.NotificationStatus = "failed"
			entry.NotificationDetail = err.Error()
		} else {
			entry.NotificationStatus = "sent"
			entry.NotificationDetail = ""
		}
		if collector != nil && !cfg.NotifySync {
			collector.UpdateActivityNotificationStatus(entry.ID, entry.NotificationStatus, entry.NotificationDetail)
		}
	}

	if cfg.NotifySync {
		dispatch()
		if collector != nil {
			collector.RecordActivity(entry)
		}
		return entry
	}

	entry.NotificationStatus = "pending"
	if collector != nil {
		collector.RecordActivity(entry)
	}
	go dispatch()
	return entry
}

func normalizeGroupName(raw string) string {
	group := strings.TrimSpace(strings.ToLower(raw))
	if group == "" || group == "all" || group == "rincent" {
		return "ribonclock"
	}
	return group
}

func scheduleTargetsForGroup(group string) (gcsPrimary string, gcsMirrors []string, localPrimary string, localSyncTargets []string) {
	if group == "ribonclock" {
		return "data/groups/ribonclock/schedule.json",
			[]string{"schedule.json", "mock_schedule.json"},
			"data/generated/schedule.json",
			[]string{
				"apps/web/public/schedule.json",
				"apps/web/dist/schedule.json",
				"data/groups/ribonclock/schedule.json",
				"apps/web/public/data/groups/ribonclock/schedule.json",
			}
	}
	path := fmt.Sprintf("data/groups/%s/schedule.json", group)
	return path,
		nil,
		path,
		[]string{
			fmt.Sprintf("apps/web/public/data/groups/%s/schedule.json", group),
			fmt.Sprintf("apps/web/dist/data/groups/%s/schedule.json", group),
		}
}

func storageModeLabel(cfg HandlerConfig) string {
	if cfg.Store != nil && cfg.DataBucket != "" {
		return "gcs"
	}
	return "local"
}

func readGroupSchedule(ctx context.Context, cfg HandlerConfig, group string) ([]byte, string, error) {
	gcsPrimary, _, localPrimary, _ := scheduleTargetsForGroup(group)
	if cfg.Store != nil && cfg.DataBucket != "" {
		data, err := cfg.Store.GetObject(ctx, cfg.DataBucket, gcsPrimary)
		if err == nil {
			return data, gcsPrimary, nil
		}
		if !errors.Is(err, gcs.ErrObjectNotFound) {
			return nil, gcsPrimary, err
		}
	}
	data, err := os.ReadFile(localPrimary)
	if err != nil {
		return nil, localPrimary, err
	}
	return data, localPrimary, nil
}

func writeGroupSchedule(ctx context.Context, cfg HandlerConfig, group string, data []byte) error {
	report, err := validate.ValidateBytes(data)
	if err != nil {
		return fmt.Errorf("validating schedule json: %w", err)
	}
	if !report.IsValid {
		return &validationError{Errors: report.Errors}
	}

	gcsPrimary, gcsMirrors, localPrimary, localSyncTargets := scheduleTargetsForGroup(group)
	if cfg.Store != nil && cfg.DataBucket != "" {
		targets := append([]string{gcsPrimary}, gcsMirrors...)
		for _, obj := range targets {
			if err := cfg.Store.PutObject(ctx, cfg.DataBucket, obj, "application/json; charset=utf-8", "no-cache, max-age=0", data); err != nil {
				return fmt.Errorf("writing %s to gs://%s: %w", obj, cfg.DataBucket, err)
			}
		}
		return nil
	}

	if err := os.WriteFile(localPrimary, data, 0644); err != nil {
		return fmt.Errorf("writing local schedule %s: %w", localPrimary, err)
	}
	if len(localSyncTargets) > 0 {
		if _, err := schedule.Sync(localPrimary, localSyncTargets...); err != nil {
			return fmt.Errorf("syncing local targets: %w", err)
		}
	}
	return nil
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": msg,
	})
}
