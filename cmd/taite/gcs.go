package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AobaIwaki123/taite-board/internal/gcs"
	"github.com/AobaIwaki123/taite-board/internal/schedule"
)

func runGCS(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: taite gcs <push|pull> [--bucket taite-board-data]")
		os.Exit(1)
	}

	sub := args[0]
	fsFlags := flag.NewFlagSet("gcs "+sub, flag.ExitOnError)
	defaultBucket := os.Getenv("GCS_DATA_BUCKET")
	if defaultBucket == "" {
		defaultBucket = "taite-board-data"
	}
	bucket := fsFlags.String("bucket", defaultBucket, "GCS data bucket name")
	_ = fsFlags.Parse(args[1:])

	client, err := gcs.NewClientFromEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "GCS credentials error: %v\n", err)
		os.Exit(1)
	}
	if client == nil {
		fmt.Fprintln(os.Stderr, "Error: set GOOGLE_APPLICATION_CREDENTIALS, GCS_CREDENTIALS_FILE, or GCS_CREDENTIALS_JSON")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	switch sub {
	case "push":
		if err := gcsPush(ctx, client, *bucket); err != nil {
			fmt.Fprintf(os.Stderr, "GCS push failed: %v\n", err)
			os.Exit(1)
		}
	case "pull":
		if err := gcsPull(ctx, client, *bucket); err != nil {
			fmt.Fprintf(os.Stderr, "GCS pull failed: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown gcs subcommand: %s (expected push or pull)\n", sub)
		os.Exit(1)
	}
}

func gcsPush(ctx context.Context, store gcs.ObjectStore, bucket string) error {
	type uploadItem struct {
		localPath  string
		objectName string
	}
	var items []uploadItem

	if _, err := os.Stat("data/generated/schedule.json"); err == nil {
		items = append(items,
			uploadItem{"data/generated/schedule.json", "schedule.json"},
			uploadItem{"data/generated/schedule.json", "mock_schedule.json"},
		)
	}
	if _, err := os.Stat("data/groups.json"); err == nil {
		items = append(items, uploadItem{"data/groups.json", "data/groups.json"})
	}

	_ = filepath.WalkDir("data/groups", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if filepath.Base(path) == "schedule.json" {
			items = append(items, uploadItem{path, filepath.ToSlash(path)})
		}
		return nil
	})
	_ = filepath.WalkDir("data/events", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".json") {
			items = append(items, uploadItem{path, filepath.ToSlash(path)})
		}
		return nil
	})

	for _, item := range items {
		data, err := os.ReadFile(item.localPath)
		if err != nil {
			return fmt.Errorf("reading %s: %w", item.localPath, err)
		}
		if err := store.PutObject(ctx, bucket, item.objectName, "application/json; charset=utf-8", "no-cache, max-age=0", data); err != nil {
			return fmt.Errorf("uploading %s -> gs://%s/%s: %w", item.localPath, bucket, item.objectName, err)
		}
		fmt.Printf("  ↑ %s -> gs://%s/%s (%d bytes)\n", item.localPath, bucket, item.objectName, len(data))
	}
	fmt.Printf("✓ Pushed %d object(s) to gs://%s\n", len(items), bucket)
	return nil
}

func gcsPull(ctx context.Context, store gcs.ObjectStore, bucket string) error {
	objects, err := store.ListObjects(ctx, bucket, "")
	if err != nil {
		return err
	}
	count := 0
	for _, obj := range objects {
		if !strings.HasSuffix(obj, ".json") {
			continue
		}
		data, err := store.GetObject(ctx, bucket, obj)
		if err != nil {
			return fmt.Errorf("downloading gs://%s/%s: %w", bucket, obj, err)
		}

		var targets []string
		switch {
		case obj == "schedule.json":
			targets = append(targets, "data/generated/schedule.json", "apps/web/public/schedule.json")
		case obj == "mock_schedule.json":
			targets = append(targets, "apps/web/public/mock_schedule.json")
		case obj == "data/groups.json":
			targets = append(targets, "data/groups.json")
		case strings.HasPrefix(obj, "data/groups/") || strings.HasPrefix(obj, "data/events/"):
			targets = append(targets, obj, filepath.Join("apps/web/public", obj))
		}

		for _, t := range targets {
			if err := os.MkdirAll(filepath.Dir(t), 0755); err != nil {
				return err
			}
			if err := os.WriteFile(t, data, 0644); err != nil {
				return fmt.Errorf("writing %s: %w", t, err)
			}
			fmt.Printf("  ↓ gs://%s/%s -> %s\n", bucket, obj, t)
		}
		count++
	}

	if _, err := os.Stat("data/generated/schedule.json"); err == nil {
		_, _ = schedule.Sync("data/generated/schedule.json", schedule.DefaultSyncTargets("")...)
	}
	fmt.Printf("✓ Pulled %d object(s) from gs://%s\n", count, bucket)
	return nil
}
