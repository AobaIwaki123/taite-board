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
  fetch     Fetch raw post snapshots from external URLs (ADR-0002)
  validate  Validate a schedule JSON file against ADR-0001 schema rules
  build     Build web client bundle via Vite`)
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
	target := "data/generated/schedule.json"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		target = args[0]
	}

	fmt.Printf("Validating schedule file: %s\n", target)
	report, err := validate.ValidateFile(target)
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
