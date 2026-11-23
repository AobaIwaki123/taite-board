package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/AobaIwaki123/taite-board/internal/gcs"
	"github.com/AobaIwaki123/taite-board/internal/monitor"
	"github.com/AobaIwaki123/taite-board/internal/syncapi"
	"github.com/AobaIwaki123/taite-board/internal/version"
)

func main() {
	var (
		logPath       string
		intakesPath   string
		dataDir       string
		statePath     string
		dashboardPort int
		syncPort      int
		metricsPort   int
	)

	flag.StringVar(&logPath, "log-path", getEnv("LOG_PATH", "/var/log/ribonclock/access.log"), "Path to Nginx access log")
	flag.StringVar(&intakesPath, "intakes-path", getEnv("INTAKES_PATH", "/var/log/ribonclock/intakes.json"), "Path to persisted intakes file")
	flag.StringVar(&dataDir, "data-dir", getEnv("DATA_DIR", "data"), "Path to data directory for pipeline telemetry")
	flag.StringVar(&statePath, "state-path", getEnv("PIPELINE_STATE_PATH", "/var/log/ribonclock/pipeline_state.json"), "Path to persisted pipeline state file")
	flag.IntVar(&dashboardPort, "port", 8082, "Port for access dashboard and JSON API")
	flag.IntVar(&syncPort, "sync-port", getEnvInt("SYNC_PORT", 8083), "Port for public user auth and sync API")
	flag.IntVar(&metricsPort, "metrics-port", 9090, "Port for Prometheus metrics")
	flag.Parse()

	ver := version.Get()
	slog.Info("Starting Ribbon Clock Access Monitor", "version", ver, "logPath", logPath, "intakesPath", intakesPath, "dataDir", dataDir, "statePath", statePath, "dashboardPort", dashboardPort, "metricsPort", metricsPort)

	dataBucket := strings.TrimSpace(os.Getenv("GCS_DATA_BUCKET"))
	stateBucket := strings.TrimSpace(os.Getenv("GCS_STATE_BUCKET"))
	var gcsStore gcs.ObjectStore
	if dataBucket != "" || stateBucket != "" {
		client, err := gcs.NewClientFromEnv()
		if err != nil {
			slog.Warn("Failed to initialize GCS client; falling back to local filesystem", "err", err)
		} else if client != nil {
			gcsStore = client
			slog.Info("GCS Cloud Storage initialized", "dataBucket", dataBucket, "stateBucket", stateBucket)
		} else {
			slog.Warn("GCS buckets configured without credentials; falling back to local filesystem")
		}
	}

	collector := monitor.NewCollector(ver)
	if intakesPath != "" {
		collector.SetIntakesPath(intakesPath)
	}
	if statePath != "" {
		collector.SetStateFilePath(statePath)
	}
	if dataDir != "" {
		collector.SetDataDir(dataDir)
	}
	if gcsStore != nil {
		collector.SetCloudStorage(gcsStore, dataBucket, stateBucket)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Start log tailer in background
	go monitor.StartTailer(ctx, logPath, collector)

	handler := monitor.NewHandlerWithConfig(collector, nil, nil, monitor.HandlerConfig{
		Store:       gcsStore,
		DataBucket:  dataBucket,
		StateBucket: stateBucket,
	})

	// Dashboard Server (port 8082)
	// WriteTimeout must exceed Cloud Run AI parser timeout (35s).
	dashboardServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", dashboardPort),
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	// Metrics Server (port 9090, separate port for internal Prometheus scraping)
	metricsMux := http.NewServeMux()
	metricsMux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, collector.PrometheusMetrics())
	})
	metricsMux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})

	metricsServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", metricsPort),
		Handler:      metricsMux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	syncHandler := syncapi.NewHandler(syncapi.Config{
		Store:         gcsStore,
		StateBucket:   stateBucket,
		ProjectID:     getEnv("GCP_PROJECT_ID", "taite-board"),
		APIKey:        strings.TrimSpace(os.Getenv("IDENTITY_PLATFORM_API_KEY")),
		AdminHandler:  handler,
		DashboardHTML: monitor.DashboardHTML,
	})
	syncServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", syncPort),
		Handler:      syncHandler.Mux(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	go func() {
		slog.Info("Dashboard server listening", "addr", dashboardServer.Addr)
		if err := dashboardServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Dashboard server error", "err", err)
		}
	}()

	go func() {
		slog.Info("User sync server listening", "addr", syncServer.Addr)
		if err := syncServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("User sync server error", "err", err)
		}
	}()

	go func() {
		slog.Info("Metrics server listening", "addr", metricsServer.Addr)
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Metrics server error", "err", err)
		}
	}()

	<-ctx.Done()
	slog.Info("Shutting down monitor servers...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	_ = dashboardServer.Shutdown(shutdownCtx)
	_ = syncServer.Shutdown(shutdownCtx)
	_ = metricsServer.Shutdown(shutdownCtx)
	slog.Info("Monitor cleanly stopped")
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if val := os.Getenv(key); val != "" {
		if n, err := strconv.Atoi(val); err == nil {
			return n
		}
	}
	return fallback
}
