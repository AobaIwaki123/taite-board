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
	if *months < 1 || *months > 12 {
		fmt.Fprintln(os.Stderr, "months must be between 1 and 12")
		os.Exit(2)
	}
	targets, err := readTargets(*groupsFile, *group)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	store, err := gcs.NewClientFromEnvOrMetadata(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	now := time.Now().In(time.FixedZone("JST", 9*60*60))
	from := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	until := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, *months, 0)
	summary, runErr := timetreesync.Run(ctx, timetreesync.Options{Source: timetree.NewClient(20 * time.Second), Store: store, Bucket: *bucket, Targets: targets, From: from, Until: until, DryRun: *dryRun})
	_ = json.NewEncoder(os.Stdout).Encode(summary)
	if runErr != nil {
		fmt.Fprintln(os.Stderr, runErr)
		os.Exit(1)
	}
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
