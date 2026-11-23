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
	"github.com/AobaIwaki123/taite-board/internal/resolve"
	"github.com/AobaIwaki123/taite-board/internal/schedule"
	"github.com/AobaIwaki123/taite-board/internal/source/x"
	"github.com/AobaIwaki123/taite-board/internal/validate"
)

//go:embed dashboard.html
var DashboardHTML []byte

// HandlerConfig configures optional cloud storage backing for the monitor server.
type HandlerConfig struct {
	Store       gcs.ObjectStore
	DataBucket  string
	StateBucket string
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
	var store gcs.ObjectStore
	if dataBucket != "" || stateBucket != "" {
		if client, err := gcs.NewClientFromEnv(); err == nil && client != nil {
			store = client
			if collector != nil {
				collector.SetCloudStorage(store, dataBucket, stateBucket)
			}
		}
	}
	return NewHandlerWithConfig(collector, nil, nil, HandlerConfig{
		Store:       store,
		DataBucket:  dataBucket,
		StateBucket: stateBucket,
	})
}

// NewHandlerWithDependencies provides dependency injection for testing in local filesystem mode.
func NewHandlerWithDependencies(collector *Collector, fetcher PostFetcher, resolver IntakeResolver) http.Handler {
	return NewHandlerWithConfig(collector, fetcher, resolver, HandlerConfig{})
}

// NewHandlerWithConfig provides full dependency and storage injection for testing and production.
func NewHandlerWithConfig(collector *Collector, fetcher PostFetcher, resolver IntakeResolver, cfg HandlerConfig) http.Handler {
	if fetcher == nil {
		fetcher = x.NewClient(15 * time.Second)
	}
	if resolver == nil {
		resolver, _ = resolve.NewResolver("")
	}
	mux := http.NewServeMux()

	// 1. Dashboard UI
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(DashboardHTML)
	})

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
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnprocessableEntity)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": fmt.Sprintf("failed to fetch post: %v", err),
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
		Group          string `json:"group"`
		SourceID       string `json:"source_id"`
		SourceURL      string `json:"source_url"`
		Author         string `json:"author"`
		Title          string `json:"title"`
		Date           string `json:"date"`
		Venue          string `json:"venue"`
		LiveStart      string `json:"live_start"`
		LiveEnd        string `json:"live_end"`
		BenefitStart   string `json:"benefit_start"`
		BenefitEnd     string `json:"benefit_end"`
		Booth          string `json:"booth"`
		ImageURL       string `json:"image_url"`
		AdmissionBonus string `json:"admission_bonus"`
		SpecialNotes   string `json:"special_notes"`
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
		if title == "" {
			title = fmt.Sprintf("ライブ出演 (%s)", req.Date)
		}
		venue := strings.TrimSpace(req.Venue)
		if venue == "" {
			venue = "未定"
		}

		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()

		currentBytes, _, err := readGroupSchedule(ctx, cfg, group)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("reading schedule: %v", err))
			return
		}

		updatedBytes, sessionID, err := schedule.ApplyAddSession(currentBytes, schedule.AddSessionOptions{
			Date:             strings.TrimSpace(req.Date),
			Title:            title,
			VenueName:        venue,
			LiveStartTime:    strings.TrimSpace(req.LiveStart),
			LiveEndTime:      strings.TrimSpace(req.LiveEnd),
			BenefitStartTime: strings.TrimSpace(req.BenefitStart),
			BenefitEndTime:   strings.TrimSpace(req.BenefitEnd),
			Booth:            strings.TrimSpace(req.Booth),
			ImageURL:         strings.TrimSpace(req.ImageURL),
			SourceTweetURL:   strings.TrimSpace(req.SourceURL),
			SpecialNotes:     strings.TrimSpace(req.SpecialNotes),
			AdmissionBonus:   strings.TrimSpace(req.AdmissionBonus),
		})
		if err != nil {
			writeJSONError(w, http.StatusUnprocessableEntity, fmt.Sprintf("failed to add session: %v", err))
			return
		}

		if err := writeGroupSchedule(ctx, cfg, group, updatedBytes); err != nil {
			var vErr *validationError
			if errors.As(err, &vErr) {
				writeJSONError(w, http.StatusUnprocessableEntity, vErr.Error())
				return
			}
			writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("saving schedule: %v", err))
			return
		}

		if collector != nil {
			collector.IncrementPromoted()
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":     "success",
			"session_id": sessionID,
			"group":      group,
			"domain":     fmt.Sprintf("%s.iorin.net", group),
			"storage":    storageModeLabel(cfg),
			"bucket":     cfg.DataBucket,
			"message":    fmt.Sprintf("Session %s successfully committed to %s.iorin.net (%s)", sessionID, group, storageModeLabel(cfg)),
		})
	})

	// 7. Intake Patch API (Update perks, photo rules, and special notes on an existing session)
	type patchRequest struct {
		Group             string `json:"group"`
		SessionID         string `json:"session_id"`
		Date              string `json:"date"`
		AdmissionBonus    string `json:"admission_bonus"`
		PhotoSong         string `json:"photo_song"`
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
			writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("reading schedule: %v", err))
			return
		}

		// Resolve target Session ID if empty
		sessionID := strings.TrimSpace(req.SessionID)
		if sessionID == "" {
			var schedDoc struct {
				CurrentNextID string `json:"current_next_id"`
				Sessions      []struct {
					ID   string `json:"id"`
					Date string `json:"date"`
				} `json:"sessions"`
			}
			_ = json.Unmarshal(rawBytes, &schedDoc)

			if req.Date != "" {
				for _, s := range schedDoc.Sessions {
					if s.Date == req.Date {
						sessionID = s.ID
						break
					}
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
			writeJSONError(w, http.StatusNotFound, "target session not found in schedule")
			return
		}

		photoRule := strings.TrimSpace(req.PhotoRule)
		if photoRule == "" && strings.TrimSpace(req.PhotoSong) != "" {
			photoRule = fmt.Sprintf("撮可: %s", strings.TrimSpace(req.PhotoSong))
		}
		var photoAvailable *bool
		if photoRule != "" || strings.TrimSpace(req.PhotoSong) != "" {
			t := true
			photoAvailable = &t
		}

		updatedBytes, err := schedule.ApplyUpdateSession(rawBytes, schedule.UpdateSessionOptions{
			SessionID:         sessionID,
			PhotoRule:         photoRule,
			PhotoSong:         strings.TrimSpace(req.PhotoSong),
			PhotoDevice:       strings.TrimSpace(req.PhotoDevice),
			PhotoAvailable:    photoAvailable,
			AdmissionBonus:    strings.TrimSpace(req.AdmissionBonus),
			SpecialNotes:      strings.TrimSpace(req.SpecialNotes),
			SourceTweetURL:    strings.TrimSpace(req.SourceURL),
			UpdateOfficialURL: req.UpdateOfficialURL,
		})
		if err != nil {
			writeJSONError(w, http.StatusUnprocessableEntity, fmt.Sprintf("failed to update session: %v", err))
			return
		}

		if err := writeGroupSchedule(ctx, cfg, group, updatedBytes); err != nil {
			var vErr *validationError
			if errors.As(err, &vErr) {
				writeJSONError(w, http.StatusUnprocessableEntity, vErr.Error())
				return
			}
			writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("saving updated schedule: %v", err))
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":     "success",
			"session_id": sessionID,
			"group":      group,
			"domain":     fmt.Sprintf("%s.iorin.net", group),
			"storage":    storageModeLabel(cfg),
			"bucket":     cfg.DataBucket,
			"message":    fmt.Sprintf("Perks & photo regulations updated on session %s (%s.iorin.net)", sessionID, group),
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

	return mux
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
