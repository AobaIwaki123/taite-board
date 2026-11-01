package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/AobaIwaki123/taite-board/internal/monitor"
	"github.com/AobaIwaki123/taite-board/internal/version"
)

func main() {
	var (
		logPath       string
		intakesPath   string
		dashboardPort int
		metricsPort   int
	)

	flag.StringVar(&logPath, "log-path", getEnv("LOG_PATH", "/var/log/ribonclock/access.log"), "Path to Nginx access log")
	flag.StringVar(&intakesPath, "intakes-path", getEnv("INTAKES_PATH", "/var/log/ribonclock/intakes.json"), "Path to persisted intakes file")
	flag.IntVar(&dashboardPort, "port", 8082, "Port for access dashboard and JSON API")
	flag.IntVar(&metricsPort, "metrics-port", 9090, "Port for Prometheus metrics")
	flag.Parse()

	ver := version.Get()
	slog.Info("Starting Ribbon Clock Access Monitor", "version", ver, "logPath", logPath, "intakesPath", intakesPath, "dashboardPort", dashboardPort, "metricsPort", metricsPort)

	collector := monitor.NewCollector(ver)
	if intakesPath != "" {
		collector.SetIntakesPath(intakesPath)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Start log tailer in background
	go monitor.StartTailer(ctx, logPath, collector)

	handler := monitor.NewHandler(collector)

	// Dashboard Server (port 8082)
	dashboardServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", dashboardPort),
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
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

	go func() {
		slog.Info("Dashboard server listening", "addr", dashboardServer.Addr)
		if err := dashboardServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Dashboard server error", "err", err)
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
	_ = metricsServer.Shutdown(shutdownCtx)
	slog.Info("Monitor cleanly stopped")
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
