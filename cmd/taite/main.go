package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/AobaIwaki123/taite-board/internal/fetch"
	"github.com/AobaIwaki123/taite-board/internal/schedule"
	"github.com/AobaIwaki123/taite-board/internal/validate"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	subcommand := os.Args[1]

	switch subcommand {
	case "fetch":
		runFetch(os.Args[2:])
	case "validate":
		runValidate(os.Args[2:])
	case "sync":
		runSync(os.Args[2:])
	case "sort":
		runSort(os.Args[2:])
	case "add-session":
		runAddSession(os.Args[2:])
	case "session":
		if len(os.Args) > 2 && os.Args[2] == "add" {
			runAddSession(os.Args[3:])
		} else if len(os.Args) > 2 && os.Args[2] == "sort" {
			runSort(os.Args[3:])
		} else {
			runUpdateSession(os.Args[2:])
		}
	case "update-session":
		runUpdateSession(os.Args[2:])
	case "build":
		runBuild(os.Args[2:])
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", subcommand)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`taite - Idol Timetable Data Pipeline CLI

Usage:
  taite <command> [arguments]

The commands are:
  fetch           Fetch raw post snapshots from external URLs (ADR-0002)
  validate        Validate a schedule JSON file against ADR-0001 schema and source integrity rules
  sync            Synchronize data/generated/schedule.json to apps/web/public/
  sort            Sort all sessions chronologically (date, live.start_time, id) and sync
  add-session     Add a new session with auto ID derivation (YYYYMMDD-N) and auto-sort
  update-session  Update session photo rules, notes, official tweet URL, and auto-sync
  build           Build web client bundle via Vite`)
}

func runFetch(args []string) {
	fs := flag.NewFlagSet("fetch", flag.ExitOnError)
	inputFile := fs.String("input", "", "Path to file containing URLs (one per line)")
	rawDir := fs.String("out", "data/raw", "Base directory to store raw snapshots")
	timeoutSec := fs.Int("timeout", 10, "HTTP timeout in seconds")
	workers := fs.Int("workers", 5, "Concurrent worker count")

	_ = fs.Parse(args)

	var urls []string

	// 1. From input file
	if *inputFile != "" {
		f, err := os.Open(*inputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening input file: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" && !strings.HasPrefix(line, "#") {
				urls = append(urls, line)
			}
		}
	}

	// 2. From positional arguments
	urls = append(urls, fs.Args()...)

	if len(urls) == 0 {
		fmt.Fprintln(os.Stderr, "Error: No URLs provided. Specify URLs as arguments or via -input.")
		os.Exit(1)
	}

	fmt.Printf("Fetching %d post(s) into %s...\n", len(urls), *rawDir)

	runner := fetch.NewRunner(*rawDir, time.Duration(*timeoutSec)*time.Second, *workers)
	manifest, err := runner.Run(context.Background(), urls)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fetch error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Fetch run completed: %s\n", manifest.RunID)
	fmt.Printf("  Saved %d post(s), %d failure(s)\n", manifest.SuccessCount, manifest.FailedCount)
	if manifest.FailedCount > 0 {
		for u, e := range manifest.Failures {
			fmt.Printf("  - %s: %s\n", u, e)
		}
	}
}

func runValidate(args []string) {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	fileOpt := fs.String("file", "data/generated/schedule.json", "Path to schedule JSON file")
	checkURLs := fs.Bool("check-urls", false, "Verify remote image URL reachability over HTTP")
	timeoutSec := fs.Int("timeout", 5, "HTTP timeout in seconds for remote URL check")

	_ = fs.Parse(args)

	target := *fileOpt
	// Also accept positional file argument if specified
	if len(fs.Args()) > 0 && !strings.HasPrefix(fs.Args()[0], "-") {
		target = fs.Args()[0]
	}

	fmt.Printf("Validating schedule file: %s (check-urls=%v)\n", target, *checkURLs)
	opts := validate.Options{
		CheckRemoteURLs: *checkURLs,
		HTTPTimeout:     time.Duration(*timeoutSec) * time.Second,
	}
	report, err := validate.ValidateFileWithOptions(target, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Validation error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Checked %d sessions:\n", report.TotalSessions)
	for _, w := range report.Warnings {
		fmt.Printf("  ⚠ Warning: %s\n", w)
	}
	for _, e := range report.Errors {
		fmt.Printf("  ✕ Error: %s\n", e)
	}

	if !report.IsValid {
		fmt.Println("✕ Validation FAILED")
		os.Exit(1)
	}
	fmt.Println("✓ Validation PASSED (All ADR-0001 schema requirements met)")
}

func runSync(args []string) {
	fs := flag.NewFlagSet("sync", flag.ExitOnError)
	srcOpt := fs.String("source", "data/generated/schedule.json", "Source schedule JSON file")
	rootOpt := fs.String("root", ".", "Project root directory")
	_ = fs.Parse(args)

	targets := schedule.DefaultSyncTargets(*rootOpt)
	hash, err := schedule.Sync(*srcOpt, targets...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Sync error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Synced schedule.json to %d web public targets (sha256: %s)\n", len(targets), hash[:8])
}

func runSort(args []string) {
	fs := flag.NewFlagSet("sort", flag.ExitOnError)
	fileOpt := fs.String("file", "data/generated/schedule.json", "Schedule JSON file path")
	rootOpt := fs.String("root", ".", "Project root directory")
	noSync := fs.Bool("no-sync", false, "Do not auto-sync to web public targets")
	_ = fs.Parse(args)

	var syncTargets []string
	if !*noSync {
		syncTargets = schedule.DefaultSyncTargets(*rootOpt)
	}

	count, err := schedule.SortSchedule(*fileOpt, syncTargets...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Sort error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Successfully sorted %d sessions chronologically in %s\n", count, *fileOpt)
	if len(syncTargets) > 0 {
		fmt.Printf("  Auto-synced to %d web public target(s)\n", len(syncTargets))
	}
}

func runAddSession(args []string) {
	fs := flag.NewFlagSet("add-session", flag.ExitOnError)
	date := fs.String("date", "", "Date (YYYY-MM-DD, required)")
	title := fs.String("title", "", "Live event title (required)")
	sessionName := fs.String("session-name", "", "Session label (e.g. 昼帯, 1部)")
	venue := fs.String("venue", "", "Venue name (required)")
	liveStart := fs.String("live-start", "--:--", "Live start time (HH:MM)")
	liveEnd := fs.String("live-end", "", "Live end time (HH:MM)")
	benType := fs.String("benefit-type", "並行", "Benefit type (並行, 終演後, etc.)")
	benStart := fs.String("benefit-start", "", "Benefit start time (HH:MM)")
	benEnd := fs.String("benefit-end", "", "Benefit end time (HH:MM)")
	booth := fs.String("booth", "", "Benefit booth (e.g. 並行B)")
	ticketPlatform := fs.String("ticket-platform", "", "Ticket platform (e.g. LivePocket, TIGET)")
	ticketURL := fs.String("ticket-url", "", "Ticket purchase URL")
	imageURL := fs.String("image-url", "", "Primary flyer image URL")
	tweetURL := fs.String("tweet-url", "", "Official announcement tweet URL")
	notes := fs.String("notes", "", "Special notes")
	fileOpt := fs.String("file", "data/generated/schedule.json", "Schedule JSON file path")
	rootOpt := fs.String("root", ".", "Project root directory")
	noSync := fs.Bool("no-sync", false, "Do not auto-sync to web public targets")

	_ = fs.Parse(args)

	if *date == "" || *title == "" || *venue == "" {
		fmt.Fprintln(os.Stderr, "Error: -date, -title, and -venue flags are required")
		fs.Usage()
		os.Exit(1)
	}

	var syncTargets []string
	if !*noSync {
		syncTargets = schedule.DefaultSyncTargets(*rootOpt)
	}

	opts := schedule.AddSessionOptions{
		Date:             *date,
		Title:            *title,
		SessionName:      *sessionName,
		VenueName:        *venue,
		LiveStartTime:    *liveStart,
		LiveEndTime:      *liveEnd,
		BenefitType:      *benType,
		BenefitStartTime: *benStart,
		BenefitEndTime:   *benEnd,
		Booth:            *booth,
		TicketPlatform:   *ticketPlatform,
		TicketURL:        *ticketURL,
		ImageURL:         *imageURL,
		SourceTweetURL:   *tweetURL,
		SpecialNotes:     *notes,
		FilePath:         *fileOpt,
		SyncTargets:      syncTargets,
	}

	newID, err := schedule.AddSession(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Add session error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Added session '%s' (%s - %s) to %s\n", newID, *date, *title, *fileOpt)
	if len(syncTargets) > 0 {
		fmt.Printf("  Auto-synced to %d web public target(s)\n", len(syncTargets))
	}
}

func runUpdateSession(args []string) {
	// Support nested subcommands: e.g. "taite session update -session ..." or "taite update-session -session ..."
	if len(args) > 0 && args[0] == "update" {
		args = args[1:]
	}

	fs := flag.NewFlagSet("update-session", flag.ExitOnError)
	sessionID := fs.String("session", "", "Session ID (required, e.g. 20261101-1)")
	rule := fs.String("rule", "", "Clean photo rule (e.g. 5曲目『75秒の沈黙』撮影可能)")
	rawText := fs.String("raw", "", "Raw announcement text snippet")
	tweetURL := fs.String("tweet-url", "", "Source tweet URL")
	official := fs.Bool("official", true, "Update source_tweet_url to the provided tweet-url")
	notes := fs.String("notes", "", "Special notes to prepend or update")
	fileOpt := fs.String("file", "data/generated/schedule.json", "Schedule JSON file path")
	rootOpt := fs.String("root", ".", "Project root directory")
	noSync := fs.Bool("no-sync", false, "Do not auto-sync to web public targets")

	_ = fs.Parse(args)

	if *sessionID == "" {
		fmt.Fprintln(os.Stderr, "Error: -session flag is required")
		fs.Usage()
		os.Exit(1)
	}

	var syncTargets []string
	if !*noSync {
		syncTargets = schedule.DefaultSyncTargets(*rootOpt)
	}

	opts := schedule.UpdateSessionOptions{
		SessionID:         *sessionID,
		PhotoRule:         *rule,
		PhotoRawText:      *rawText,
		SourceTweetURL:    *tweetURL,
		UpdateOfficialURL: *official && *tweetURL != "",
		SpecialNotes:      *notes,
		FilePath:          *fileOpt,
		SyncTargets:       syncTargets,
	}

	if err := schedule.UpdateSession(opts); err != nil {
		fmt.Fprintf(os.Stderr, "Update error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Session %s updated successfully in %s\n", *sessionID, *fileOpt)
	if len(syncTargets) > 0 {
		fmt.Printf("  Auto-synced to %d web public target(s)\n", len(syncTargets))
	}
}

func runBuild(args []string) {
	buildScript := "scripts/build.sh"
	if _, err := os.Stat(buildScript); err != nil {
		// Try finding relative to current dir
		buildScript = filepath.Join("..", "scripts", "build.sh")
	}

	cmd := exec.Command("bash", buildScript)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Build error: %v\n", err)
		os.Exit(1)
	}
}
