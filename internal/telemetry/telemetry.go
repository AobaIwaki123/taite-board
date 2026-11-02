package telemetry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/AobaIwaki123/taite-board/internal/candidate"
	"github.com/AobaIwaki123/taite-board/internal/filter"
	"github.com/AobaIwaki123/taite-board/internal/raw"
)

// UnitCostUSDPerCall is the estimated Gemini 2.5 Flash Vision cost per timetable image prompt ($0.0004).
const UnitCostUSDPerCall = 0.0004

// CrawlRunRecord represents an execution summary of a single crawler run.
type CrawlRunRecord struct {
	RunID        string `json:"run_id"`
	Source       string `json:"source"`
	FetchedAt    string `json:"fetched_at"`
	InputCount   int    `json:"input_count"`
	SuccessCount int    `json:"success_count"`
	FailedCount  int    `json:"failed_count"`
	Filtered     int    `json:"filtered"`
	Passed       int    `json:"passed"`
	Status       string `json:"status"`
}

// FilterBreakdown represents the reasons raw posts were filtered out locally.
type FilterBreakdown struct {
	Media     int64 `json:"media"`
	Age       int64 `json:"age"`
	Keyword   int64 `json:"keyword"`
	Duplicate int64 `json:"duplicate"`
	Total     int64 `json:"total"`
}

// PipelineStats contains aggregated end-to-end telemetry for the ingestion funnel.
type PipelineStats struct {
	LastScannedAt         string           `json:"last_scanned_at"`
	TotalRuns             int64            `json:"total_runs"`
	LastRun               *CrawlRunRecord  `json:"last_run,omitempty"`
	DetectedPosts         int64            `json:"detected_posts"`
	FilteredPosts         int64            `json:"filtered_posts"`
	FilterPassed          int64            `json:"filter_passed"`
	FilterBreakdown       FilterBreakdown  `json:"filter_breakdown"`
	AIDispatched          int64            `json:"ai_dispatched"`
	AICacheHits           int64            `json:"ai_cache_hits"`
	PromotedSessions      int64            `json:"promoted_sessions"`
	SavedAPICalls         int64            `json:"saved_api_calls"`
	EstimatedCostSavedUSD float64          `json:"estimated_cost_saved_usd"`
	RecentRuns            []CrawlRunRecord `json:"recent_runs"`
}

// CollectPipelineStats scans the repository data directory and computes end-to-end pipeline metrics.
func CollectPipelineStats(dataDir string) (PipelineStats, error) {
	now := time.Now()
	stats := PipelineStats{
		LastScannedAt: now.Format(time.RFC3339),
		RecentRuns:    make([]CrawlRunRecord, 0),
	}

	runsDirCandidates := []string{
		filepath.Join(dataDir, "raw", "runs"),
		filepath.Join(dataDir, "runs"),
		"/var/log/ribonclock/runs",
	}
	if customRunsDir := os.Getenv("RUNS_DIR"); customRunsDir != "" {
		runsDirCandidates = append([]string{customRunsDir}, runsDirCandidates...)
	}
	seenFingerprints := make(map[string]bool)

	type runItem struct {
		rec     CrawlRunRecord
		modTime time.Time
	}
	var runItems []runItem

	for _, runsDir := range runsDirCandidates {
		entries, err := os.ReadDir(runsDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			runPath := filepath.Join(runsDir, entry.Name())
			manifestPath := filepath.Join(runPath, "manifest.json")

			var manifest raw.RunManifest
			if mBytes, err := os.ReadFile(manifestPath); err == nil {
				_ = json.Unmarshal(mBytes, &manifest)
			}

			runRec := CrawlRunRecord{
				RunID:        entry.Name(),
				Source:       manifest.Source,
				FetchedAt:    manifest.FetchedAt,
				InputCount:   manifest.InputCount,
				SuccessCount: manifest.SuccessCount,
				FailedCount:  manifest.FailedCount,
			}
			if runRec.Source == "" {
				runRec.Source = "x"
			}

			// Scan posts in this run
			postsDir := filepath.Join(runPath, "posts")
			postEntries, _ := os.ReadDir(postsDir)

			runFiltered := 0
			runPassed := 0

			for _, pe := range postEntries {
				if pe.IsDir() || !strings.HasSuffix(pe.Name(), ".json") {
					continue
				}
				postBytes, err := os.ReadFile(filepath.Join(postsDir, pe.Name()))
				if err != nil {
					continue
				}
				var post raw.Post
				if err := json.Unmarshal(postBytes, &post); err != nil {
					continue
				}

				stats.DetectedPosts++

				// Run local pre-filter evaluation
				res := filter.Evaluate(&post, filter.DefaultRules, seenFingerprints, now)
				if res.Passed {
					stats.FilterPassed++
					runPassed++
					seenFingerprints[res.Fingerprint] = true
				} else {
					stats.FilteredPosts++
					runFiltered++
					stats.FilterBreakdown.Total++
					switch res.Stage {
					case filter.StageMedia:
						stats.FilterBreakdown.Media++
					case filter.StageAge:
						stats.FilterBreakdown.Age++
					case filter.StageExclude, filter.StageTarget:
						stats.FilterBreakdown.Keyword++
					case filter.StageDuplicate:
						stats.FilterBreakdown.Duplicate++
					default:
						stats.FilterBreakdown.Keyword++
					}
				}
			}

			runRec.Filtered = runFiltered
			runRec.Passed = runPassed
			if runRec.InputCount == 0 && (runFiltered+runPassed) > 0 {
				runRec.InputCount = runFiltered + runPassed
			}
			if runPassed > 0 {
				runRec.Status = "Candidates Extracted"
			} else if runFiltered > 0 {
				runRec.Status = "Filtered / Shielded"
			} else {
				runRec.Status = "Completed"
			}

			info, _ := entry.Info()
			modTime := time.Time{}
			if info != nil {
				modTime = info.ModTime()
			}

			runItems = append(runItems, runItem{rec: runRec, modTime: modTime})
		}
	}

	stats.TotalRuns = int64(len(runItems))

	// Sort runs by RunID descending (latest first)
	sort.Slice(runItems, func(i, j int) bool {
		return runItems[i].rec.RunID > runItems[j].rec.RunID
	})

	maxRuns := 10
	if len(runItems) < maxRuns {
		maxRuns = len(runItems)
	}
	for i := 0; i < maxRuns; i++ {
		stats.RecentRuns = append(stats.RecentRuns, runItems[i].rec)
	}
	if len(stats.RecentRuns) > 0 {
		stats.LastRun = &stats.RecentRuns[0]
	}

	// Scan candidate directory for AI dispatch and cache hit metrics
	candidateDir := filepath.Join(dataDir, "candidate")
	if cEntries, err := os.ReadDir(candidateDir); err == nil {
		for _, ce := range cEntries {
			if ce.IsDir() || !strings.HasSuffix(ce.Name(), ".json") {
				continue
			}
			cBytes, err := os.ReadFile(filepath.Join(candidateDir, ce.Name()))
			if err != nil {
				continue
			}
			var cand candidate.IntakeResult
			if err := json.Unmarshal(cBytes, &cand); err != nil {
				continue
			}

			if cand.Status == candidate.StatusSkippedCache {
				stats.AICacheHits++
			} else if cand.NeedsAI || cand.Status == candidate.StatusCompleted || cand.Status == candidate.StatusNeedsReview {
				stats.AIDispatched++
			}
		}
	}

	// If candidate records are not persisted yet, passed filter posts default to AI dispatch candidates
	if stats.AIDispatched == 0 && stats.FilterPassed > 0 {
		stats.AIDispatched = stats.FilterPassed
	}

	// Scan schedule.json for total active/promoted sessions
	schedulePath := filepath.Join(dataDir, "generated", "schedule.json")
	if sBytes, err := os.ReadFile(schedulePath); err == nil {
		var schedDoc struct {
			Sessions []json.RawMessage `json:"sessions"`
		}
		if err := json.Unmarshal(sBytes, &schedDoc); err == nil {
			stats.PromotedSessions = int64(len(schedDoc.Sessions))
		}
	}

	// Calculate cost savings
	stats.SavedAPICalls = stats.FilteredPosts + stats.AICacheHits
	stats.EstimatedCostSavedUSD = float64(stats.SavedAPICalls) * UnitCostUSDPerCall

	return stats, nil
}

// SavePipelineState writes aggregated pipeline metrics to a persistent JSON file.
func SavePipelineState(filePath string, stats PipelineStats) error {
	if filePath == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, data, 0644)
}

// LoadPipelineState loads aggregated pipeline metrics from a persistent JSON file.
func LoadPipelineState(filePath string) (PipelineStats, error) {
	var stats PipelineStats
	if filePath == "" {
		return stats, os.ErrNotExist
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return stats, err
	}
	if err := json.Unmarshal(data, &stats); err != nil {
		return stats, err
	}
	return stats, nil
}

// CollectAndPersist collects pipeline statistics and automatically persists them to a state file.
// If the data directory has no runs (e.g. fresh ephemeral container), it gracefully falls back to the persisted state file.
func CollectAndPersist(dataDir string, stateFile string) (PipelineStats, error) {
	stats, err := CollectPipelineStats(dataDir)
	if err == nil && (stats.TotalRuns > 0 || stats.DetectedPosts > 0) {
		if stateFile != "" {
			_ = SavePipelineState(stateFile, stats)
		}
		return stats, nil
	}

	// Fallback to persisted state file if scan produced empty runs
	if stateFile != "" {
		if persisted, pErr := LoadPipelineState(stateFile); pErr == nil && (persisted.TotalRuns > 0 || persisted.DetectedPosts > 0) {
			// Update promoted sessions from current schedule.json if available
			if stats.PromotedSessions > 0 {
				persisted.PromotedSessions = stats.PromotedSessions
			}
			persisted.LastScannedAt = time.Now().Format(time.RFC3339)
			return persisted, nil
		}
	}

	return stats, err
}

// FormatASCIIFunnel produces a clean, readable ASCII summary of the funnel for CLI output.
func FormatASCIIFunnel(stats PipelineStats) string {
	var sb strings.Builder

	sb.WriteString("============================================================\n")
	sb.WriteString("           Ribbon Clock Pipeline Funnel & Telemetry\n")
	sb.WriteString("============================================================\n")
	if stats.LastRun != nil {
		sb.WriteString(fmt.Sprintf("Latest Crawl Run:        %s (%s)\n", stats.LastRun.RunID, stats.LastRun.Status))
		sb.WriteString(fmt.Sprintf("Latest Run Detected:     %d (Filtered: %d, Passed: %d)\n",
			stats.LastRun.InputCount, stats.LastRun.Filtered, stats.LastRun.Passed))
		sb.WriteString("------------------------------------------------------------\n")
	}
	sb.WriteString(fmt.Sprintf("Total Crawl Runs:        %d\n", stats.TotalRuns))
	sb.WriteString(fmt.Sprintf("Detected Posts:          %d\n", stats.DetectedPosts))
	sb.WriteString("------------------------------------------------------------\n")
	sb.WriteString("[Filter Funnel]\n")

	var filterRate float64
	var passRate float64
	if stats.DetectedPosts > 0 {
		filterRate = float64(stats.FilteredPosts) / float64(stats.DetectedPosts) * 100
		passRate = float64(stats.FilterPassed) / float64(stats.DetectedPosts) * 100
	}

	sb.WriteString(fmt.Sprintf("  [-] Filtered Locally:    %d  (%.1f%%)\n", stats.FilteredPosts, filterRate))
	sb.WriteString(fmt.Sprintf("      * Media Check:       %d\n", stats.FilterBreakdown.Media))
	sb.WriteString(fmt.Sprintf("      * Age / Expired:     %d\n", stats.FilterBreakdown.Age))
	sb.WriteString(fmt.Sprintf("      * Keyword Filter:    %d\n", stats.FilterBreakdown.Keyword))
	sb.WriteString(fmt.Sprintf("      * Duplicate Post:    %d\n", stats.FilterBreakdown.Duplicate))
	sb.WriteString(fmt.Sprintf("  [+] Passed Pre-Filter:   %d  (%.1f%%)\n", stats.FilterPassed, passRate))
	sb.WriteString("------------------------------------------------------------\n")
	sb.WriteString("[AI Parser & Promotion]\n")
	sb.WriteString(fmt.Sprintf("  [>] Cloud Run Dispatched: %d\n", stats.AIDispatched))
	sb.WriteString(fmt.Sprintf("  [=] AI Cache Hits:        %d\n", stats.AICacheHits))
	sb.WriteString(fmt.Sprintf("  [*] Promoted Sessions:    %d (in schedule.json)\n", stats.PromotedSessions))
	sb.WriteString("------------------------------------------------------------\n")
	sb.WriteString("[Cost Protection]\n")
	sb.WriteString(fmt.Sprintf("  Total API Calls Saved:   %d\n", stats.SavedAPICalls))
	sb.WriteString(fmt.Sprintf("  Estimated Cost Saved:    $%.4f USD\n", stats.EstimatedCostSavedUSD))
	sb.WriteString("============================================================\n")

	return sb.String()
}
