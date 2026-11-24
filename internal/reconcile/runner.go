package reconcile

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/AobaIwaki123/taite-board/internal/gcs"
	"github.com/AobaIwaki123/taite-board/internal/resolve"
	"github.com/AobaIwaki123/taite-board/internal/schedule"
	"github.com/AobaIwaki123/taite-board/internal/validate"
)

// GroupResult holds the reconciliation result for a single group schedule.
type GroupResult struct {
	Group           string                   `json:"group"`
	PrimaryObject   string                   `json:"primary_object"`
	MirroredObjects []string                 `json:"mirrored_objects,omitempty"`
	Report          schedule.ReconcileReport `json:"report"`
	Written         bool                     `json:"written"`
	Error           string                   `json:"error,omitempty"`
}

// JobSummary summarizes an end-to-end reconciliation job run across one or more groups.
type JobSummary struct {
	StartedAt     string        `json:"started_at"`
	CompletedAt   string        `json:"completed_at"`
	Bucket        string        `json:"bucket"`
	TargetGroup   string        `json:"target_group"`
	DryRun        bool          `json:"dry_run"`
	TriggerSource string        `json:"trigger_source,omitempty"`
	Results       []GroupResult `json:"results"`
	AnyChanged    bool          `json:"any_changed"`
	HasErrors     bool          `json:"has_errors"`
}

// RunOptions configures a GCS schedule reconciliation job execution.
type RunOptions struct {
	Store         gcs.ObjectStore
	Bucket        string
	Group         string // "all" or specific group name (e.g. "ribonclock", "dollykiss")
	DryRun        bool
	TriggerSource string
	EntitiesPath  string
}

// DiscoverGroups lists target schedule groups inside the GCS data bucket.
func DiscoverGroups(ctx context.Context, store gcs.ObjectStore, bucket, requestedGroup string) ([]string, error) {
	req := strings.TrimSpace(strings.ToLower(requestedGroup))
	if req == "rincent" {
		req = "ribonclock"
	}
	if req != "" && req != "all" {
		return []string{req}, nil
	}

	objs, err := store.ListObjects(ctx, bucket, "data/groups/")
	if err != nil {
		return nil, fmt.Errorf("listing groups in gs://%s: %w", bucket, err)
	}

	seen := make(map[string]bool)
	var groups []string
	for _, name := range objs {
		// Expected format: data/groups/<group>/schedule.json
		if !strings.HasPrefix(name, "data/groups/") || !strings.HasSuffix(name, "/schedule.json") {
			continue
		}
		trimmed := strings.TrimPrefix(name, "data/groups/")
		trimmed = strings.TrimSuffix(trimmed, "/schedule.json")
		if trimmed == "" || strings.Contains(trimmed, "/") || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		groups = append(groups, trimmed)
	}

	sort.Slice(groups, func(i, j int) bool {
		if groups[i] == "ribonclock" {
			return true
		}
		if groups[j] == "ribonclock" {
			return false
		}
		return groups[i] < groups[j]
	})
	if len(groups) == 0 {
		groups = []string{"ribonclock"}
	}
	return groups, nil
}

// Run executes deterministic entity resolution and session deduplication across the requested groups in GCS.
func Run(ctx context.Context, opts RunOptions) (JobSummary, error) {
	jst := time.FixedZone("Asia/Tokyo", 9*60*60)
	summary := JobSummary{
		StartedAt:     time.Now().In(jst).Format(time.RFC3339),
		Bucket:        opts.Bucket,
		TargetGroup:   opts.Group,
		DryRun:        opts.DryRun,
		TriggerSource: opts.TriggerSource,
	}
	if summary.TargetGroup == "" {
		summary.TargetGroup = "all"
	}
	if opts.Store == nil {
		return summary, fmt.Errorf("gcs store is required")
	}
	if strings.TrimSpace(opts.Bucket) == "" {
		return summary, fmt.Errorf("gcs bucket is required")
	}

	resolver, err := resolve.NewResolver(opts.EntitiesPath)
	if err != nil {
		return summary, fmt.Errorf("initializing resolver: %w", err)
	}

	groups, err := DiscoverGroups(ctx, opts.Store, opts.Bucket, summary.TargetGroup)
	if err != nil {
		return summary, err
	}

	for _, group := range groups {
		primary := fmt.Sprintf("data/groups/%s/schedule.json", group)
		var mirrors []string
		if group == "ribonclock" {
			mirrors = []string{"schedule.json", "mock_schedule.json"}
		}

		res := GroupResult{
			Group:         group,
			PrimaryObject: primary,
		}

		raw, err := opts.Store.GetObject(ctx, opts.Bucket, primary)
		if err != nil {
			res.Error = fmt.Sprintf("reading %s: %v", primary, err)
			summary.HasErrors = true
			summary.Results = append(summary.Results, res)
			continue
		}

		reconciled, report, err := schedule.ApplyReconcileSchedule(raw, resolver)
		if err != nil {
			res.Error = fmt.Sprintf("reconciling %s: %v", primary, err)
			summary.HasErrors = true
			summary.Results = append(summary.Results, res)
			continue
		}
		res.Report = report

		valReport, err := validate.ValidateBytes(reconciled)
		if err != nil {
			res.Error = fmt.Sprintf("validating reconciled %s: %v", primary, err)
			summary.HasErrors = true
			summary.Results = append(summary.Results, res)
			continue
		}
		if !valReport.IsValid {
			res.Error = fmt.Sprintf("schema validation failed for %s: %s", primary, strings.Join(valReport.Errors, "; "))
			summary.HasErrors = true
			summary.Results = append(summary.Results, res)
			continue
		}

		if report.Changed {
			summary.AnyChanged = true
		}

		if report.Changed && !opts.DryRun {
			targets := append([]string{primary}, mirrors...)
			for _, obj := range targets {
				if err := opts.Store.PutObject(ctx, opts.Bucket, obj, "application/json; charset=utf-8", "no-cache, max-age=0", reconciled); err != nil {
					res.Error = fmt.Sprintf("writing %s: %v", obj, err)
					summary.HasErrors = true
					break
				}
			}
			if res.Error == "" {
				res.Written = true
				res.MirroredObjects = mirrors
			}
		}

		summary.Results = append(summary.Results, res)
	}

	summary.CompletedAt = time.Now().In(jst).Format(time.RFC3339)
	if summary.HasErrors {
		var errs []string
		for _, r := range summary.Results {
			if r.Error != "" {
				errs = append(errs, fmt.Sprintf("%s: %s", r.Group, r.Error))
			}
		}
		return summary, fmt.Errorf("reconciliation failed (%s)", strings.Join(errs, "; "))
	}
	return summary, nil
}
