package monitor

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/AobaIwaki123/taite-board/internal/candidate"
	"github.com/AobaIwaki123/taite-board/internal/raw"
	"github.com/AobaIwaki123/taite-board/internal/resolve"
	"github.com/AobaIwaki123/taite-board/internal/source/x"
)

//go:embed dashboard.html
var DashboardHTML []byte

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
	xClient := x.NewClient(15 * time.Second)
	res, _ := resolve.NewResolver("")
	return NewHandlerWithDependencies(collector, xClient, res)
}

// NewHandlerWithDependencies provides full dependency injection for testing.
func NewHandlerWithDependencies(collector *Collector, fetcher PostFetcher, resolver IntakeResolver) http.Handler {
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
		if needsAI {
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

	return mux
}
