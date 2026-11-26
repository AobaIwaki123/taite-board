package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/AobaIwaki123/taite-board/internal/gcs"
	"github.com/AobaIwaki123/taite-board/internal/monitor"
	"github.com/AobaIwaki123/taite-board/internal/reconcile"
	"github.com/AobaIwaki123/taite-board/internal/schedule"
	"github.com/AobaIwaki123/taite-board/internal/version"
)

func main() {
	var (
		bucket        string
		group         string
		dryRun        bool
		webhookURL    string
		triggerSource string
	)

	defaultBucket := getEnv("GCS_DATA_BUCKET", "taite-board-data")
	defaultGroup := getEnv("TARGET_GROUP", "all")
	defaultDryRun, _ := strconv.ParseBool(os.Getenv("DRY_RUN"))
	defaultWebhook := strings.TrimSpace(os.Getenv("DISCORD_WEBHOOK_URL"))
	defaultTrigger := getEnv("TRIGGER_SOURCE", "cloud-run-job")

	flag.StringVar(&bucket, "bucket", defaultBucket, "GCS data bucket name")
	flag.StringVar(&group, "group", defaultGroup, "Target group (all, ribonclock, dollykiss, ...)")
	flag.BoolVar(&dryRun, "dry-run", defaultDryRun, "Preview changes without writing back to GCS")
	flag.StringVar(&webhookURL, "webhook-url", defaultWebhook, "Optional Discord webhook URL")
	flag.StringVar(&triggerSource, "trigger-source", defaultTrigger, "Trigger source label for logs")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	slog.Info("Starting Cloud Run Reconcile Job",
		"version", version.Get(),
		"bucket", bucket,
		"group", group,
		"dry_run", dryRun,
		"trigger_source", triggerSource,
	)

	store, err := gcs.NewClientFromEnvOrMetadata(ctx)
	if err != nil {
		slog.Error("Failed to initialize GCS client", "error", err)
		notifyReconcileError(ctx, webhookURL, group, err)
		os.Exit(1)
	}

	summary, runErr := reconcile.Run(ctx, reconcile.RunOptions{
		Store:              store,
		Bucket:             bucket,
		Group:              group,
		DryRun:             dryRun,
		TriggerSource:      triggerSource,
		ImageFingerprinter: schedule.NewHTTPImageFingerprinter(10 * time.Second),
	})

	outBytes, _ := json.MarshalIndent(summary, "", "  ")
	fmt.Println(string(outBytes))

	if runErr != nil {
		slog.Error("Reconciliation completed with errors", "error", runErr)
		notifyReconcileError(ctx, webhookURL, group, runErr)
		os.Exit(1)
	}

	notifyReconcileSuccess(ctx, webhookURL, summary)
	slog.Info("Reconciliation job finished", "any_changed", summary.AnyChanged, "groups", len(summary.Results))
}

func notifyReconcileError(ctx context.Context, webhookURL, group string, err error) {
	if strings.TrimSpace(webhookURL) == "" {
		return
	}
	msg := fmt.Sprintf("**[Taite Reconcile Job] [失敗] 名寄せ処理エラー**\n- 対象: `%s`\n- エラー: `%v`", group, err)
	if notifyErr := monitor.SendDiscordWebhook(ctx, nil, webhookURL, msg); notifyErr != nil {
		slog.Warn("Failed to send error Discord notification", "error", notifyErr)
	}
}

func notifyReconcileSuccess(ctx context.Context, webhookURL string, summary reconcile.JobSummary) {
	if strings.TrimSpace(webhookURL) == "" {
		return
	}
	mode := "本番反映"
	if summary.DryRun {
		mode = "Dry Run（確認のみ）"
	}
	var lines []string
	lines = append(lines, fmt.Sprintf("**[Taite Reconcile Job] 名寄せ・重複整理完了 (%s)**", mode))
	for _, r := range summary.Results {
		status := "変更なし"
		if r.Written {
			status = "GCS反映済み"
		} else if r.Report.Changed && summary.DryRun {
			status = "変更候補あり"
		}
		lines = append(lines, fmt.Sprintf("- `%s` (%s): %d -> %d件 / 統合:%d / 会場:%d / タイトル:%d / チケット:%d",
			r.Group, status, r.Report.BeforeCount, r.Report.AfterCount, r.Report.MergedCount, r.Report.VenuesNormalized, r.Report.TitlesCleaned, r.Report.TicketsNormalized))
	}
	if notifyErr := monitor.SendDiscordWebhook(ctx, nil, webhookURL, strings.Join(lines, "\n")); notifyErr != nil {
		slog.Warn("Failed to send completion Discord notification", "error", notifyErr)
	}
}

func getEnv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
