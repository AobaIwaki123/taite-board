package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/AobaIwaki123/taite-board/internal/gcs"
	"github.com/AobaIwaki123/taite-board/internal/monitor"
	"github.com/AobaIwaki123/taite-board/internal/source/timetree"
	"github.com/AobaIwaki123/taite-board/internal/timetreesync"
)

// Command-line and group configuration are kept separate from the calendar
// source and synchronization engine to support other callers and calendars.
func main() {
	groupsFile := flag.String("groups-file", "data/groups.json", "group configuration JSON")
	group := flag.String("group", "all", "group ID or all")
	bucket := flag.String("bucket", envOr("GCS_DATA_BUCKET", "taite-board-data"), "GCS data bucket")
	months := flag.Int("months", 3, "number of calendar months, including current month")
	dryRun := flag.Bool("dry-run", false, "report changes without writing")
	flag.Parse()
	webhook := strings.TrimSpace(os.Getenv("DISCORD_WEBHOOK_URL"))
	if !*dryRun {
		notify(webhook, "TimeTree 同期を開始します。")
	}
	fail := func(err error) {
		fmt.Fprintln(os.Stderr, err)
		if !*dryRun {
			notify(webhook, "TimeTree 同期に失敗しました。詳細は実行ログを確認してください。")
		}
	}
	if *months < 1 || *months > 12 {
		fail(fmt.Errorf("months must be between 1 and 12"))
		os.Exit(2)
	}
	targets, err := readTargets(*groupsFile, *group)
	if err != nil {
		fail(err)
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	store, err := gcs.NewClientFromEnvOrMetadata(ctx)
	if err != nil {
		fail(err)
		os.Exit(1)
	}
	now := time.Now().In(time.FixedZone("JST", 9*60*60))
	from := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	until := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, *months, 0)
	summary, runErr := timetreesync.Run(ctx, timetreesync.Options{Source: timetree.NewClient(20 * time.Second), Store: store, Bucket: *bucket, Targets: targets, From: from, Until: until, DryRun: *dryRun})
	_ = json.NewEncoder(os.Stdout).Encode(summary)
	if !*dryRun {
		notify(webhook, formatSummary(summary, runErr))
	}
	if runErr != nil {
		fmt.Fprintln(os.Stderr, runErr)
		os.Exit(1)
	}
}

func notify(webhook, text string) {
	if webhook == "" {
		fmt.Fprintln(os.Stderr, "Discord notification is not configured")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := monitor.SendDiscordWebhook(ctx, nil, webhook, text); err != nil {
		// Do not print request errors: HTTP clients can include the webhook URL.
		fmt.Fprintln(os.Stderr, "Discord notification failed")
	}
}

func formatSummary(summary timetreesync.Summary, runErr error) string {
	status := "完了"
	if runErr != nil {
		status = "一部失敗"
	}
	lines := []string{"TimeTree 同期 " + status}
	for _, result := range summary.Results {
		name := result.Group
		switch name {
		case "ribonclock":
			name = "リボンクロック"
		case "dollykiss":
			name = "Dolly Kiss"
		}
		updated, pending := 0, result.Changed
		if result.Written {
			updated, pending = result.Changed, 0
		}
		line := fmt.Sprintf("%s: 取得%d件・更新%d件・保留%d件", name, result.Fetched, updated, len(result.Conflicts))
		if pending > 0 {
			line += fmt.Sprintf("・未反映%d件", pending)
		}
		if result.Error != "" {
			line += "・失敗（実行ログを参照）"
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func readTargets(path, requested string) ([]timetreesync.Target, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Groups map[string]struct {
			ID       string `json:"id"`
			Calendar string `json:"timetree_id"`
		} `json:"groups"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	var targets []timetreesync.Target
	for id, group := range doc.Groups {
		if requested != "all" && requested != id {
			continue
		}
		if group.ID != id {
			return nil, fmt.Errorf("group ID mismatch for %s", id)
		}
		if group.Calendar != "" {
			targets = append(targets, timetreesync.Target{ID: id, CalendarID: group.Calendar})
		}
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i].ID < targets[j].ID })
	if len(targets) == 0 {
		return nil, fmt.Errorf("no configured TimeTree calendars for %s", requested)
	}
	return targets, nil
}
