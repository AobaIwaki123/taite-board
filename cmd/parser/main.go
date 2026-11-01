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
	"syscall"
	"time"

	"github.com/AobaIwaki123/taite-board/internal/parser"
	"github.com/AobaIwaki123/taite-board/internal/version"
)

func main() {
	var (
		port      int
		apiKey    string
		authToken string
	)

	defaultPort := 8080
	if envPort := os.Getenv("PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil {
			defaultPort = p
		}
	}

	flag.IntVar(&port, "port", defaultPort, "Port to listen on (default: $PORT or 8080)")
	flag.StringVar(&apiKey, "gemini-api-key", os.Getenv("GEMINI_API_KEY"), "Gemini API Key")
	flag.StringVar(&authToken, "auth-token", os.Getenv("PARSER_AUTH_TOKEN"), "Optional bearer auth token for Cloud Run")
	flag.Parse()

	ver := version.Get()
	slog.Info("Starting Cloud Run Timetable AI Parser",
		"version", ver,
		"port", port,
		"has_gemini_key", apiKey != "",
		"has_auth_token", authToken != "",
	)

	geminiClient := parser.NewGeminiClient(apiKey, 30*time.Second)
	srv := parser.NewServer(geminiClient, authToken)

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      srv.Handler(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 45 * time.Second,
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go func() {
		slog.Info("Cloud Run AI Parser listening", "addr", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("Shutting down Cloud Run AI Parser gracefully...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		slog.Error("Forced shutdown error", "error", err)
	}
	slog.Info("Cloud Run AI Parser stopped cleanly")
}
