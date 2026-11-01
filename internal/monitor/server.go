package monitor

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
)

//go:embed dashboard.html
var DashboardHTML []byte

// Handler creates an http.Handler that serves the dashboard, API, and metrics.
func NewHandler(collector *Collector) http.Handler {
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

	return mux
}
