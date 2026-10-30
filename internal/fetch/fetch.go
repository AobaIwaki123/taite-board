package fetch

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/AobaIwaki123/taite-board/internal/raw"
	"github.com/AobaIwaki123/taite-board/internal/source/x"
)

// Runner manages raw post fetching and snapshot persistence (ADR-0002).
type Runner struct {
	BaseDir string
	Timeout time.Duration
	Workers int
}

// NewRunner creates a new Fetch Runner.
func NewRunner(baseDir string, timeout time.Duration, workers int) *Runner {
	if workers <= 0 {
		workers = 5
	}
	return &Runner{
		BaseDir: baseDir,
		Timeout: timeout,
		Workers: workers,
	}
}

// Run executes fetching for a list of URLs and saves immutable snapshots to data/raw/runs/<run-id>/.
func (r *Runner) Run(ctx context.Context, urls []string) (*raw.RunManifest, error) {
	runID := time.Now().Format("20060102T150405-0700")
	runDir := filepath.Join(r.BaseDir, "runs", runID)
	postsDir := filepath.Join(runDir, "posts")

	if err := os.MkdirAll(postsDir, 0755); err != nil {
		return nil, fmt.Errorf("creating run directories: %w", err)
	}

	client := x.NewClient(r.Timeout)
	sem := make(chan struct{}, r.Workers)
	var wg sync.WaitGroup

	var mu sync.Mutex
	failures := make(map[string]string)
	var savedPosts []string

	for _, u := range urls {
		u := u
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			reqCtx, cancel := context.WithTimeout(ctx, r.Timeout)
			defer cancel()

			post, err := client.FetchPost(reqCtx, u)
			if err != nil {
				mu.Lock()
				failures[u] = err.Error()
				mu.Unlock()
				return
			}

			// Save post snapshot
			postFile := filepath.Join(postsDir, fmt.Sprintf("%s.json", post.SourceID))
			postData, err := json.MarshalIndent(post, "", "  ")
			if err != nil {
				mu.Lock()
				failures[u] = fmt.Sprintf("serializing post: %v", err)
				mu.Unlock()
				return
			}

			if err := os.WriteFile(postFile, postData, 0644); err != nil {
				mu.Lock()
				failures[u] = fmt.Sprintf("writing post file: %v", err)
				mu.Unlock()
				return
			}

			mu.Lock()
			savedPosts = append(savedPosts, post.SourceID)
			mu.Unlock()
		}()
	}

	wg.Wait()

	manifest := &raw.RunManifest{
		SchemaVersion: 1,
		RunID:         runID,
		Source:        "x",
		FetchedAt:     time.Now().Format(time.RFC3339),
		InputCount:    len(urls),
		SuccessCount:  len(savedPosts),
		FailedCount:   len(failures),
		Failures:      failures,
		SavedPosts:    savedPosts,
	}

	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err == nil {
		_ = os.WriteFile(filepath.Join(runDir, "manifest.json"), manifestData, 0644)
	}

	return manifest, nil
}
