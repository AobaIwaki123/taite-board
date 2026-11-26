package timetreesync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"time"

	"github.com/AobaIwaki123/taite-board/internal/gcs"
	"github.com/AobaIwaki123/taite-board/internal/schedule"
	"github.com/AobaIwaki123/taite-board/internal/source/timetree"
	"github.com/AobaIwaki123/taite-board/internal/validate"
)

type CalendarSource interface {
	ListEvents(context.Context, string) ([]timetree.Event, error)
}
type VersionedStore interface {
	GetObjectVersion(context.Context, string, string) ([]byte, string, error)
	PutObjectIfGeneration(context.Context, string, string, string, string, []byte, string) error
}
type Target struct{ ID, CalendarID string }
type Options struct {
	Source      CalendarSource
	Store       VersionedStore
	Bucket      string
	Targets     []Target
	From, Until time.Time // inclusive start, exclusive end
	DryRun      bool
}
type GroupResult struct {
	Group     string   `json:"group"`
	Fetched   int      `json:"fetched"`
	Processed int      `json:"processed"`
	Changed   int      `json:"changed"`
	Conflicts []string `json:"conflicts,omitempty"`
	Written   bool     `json:"written"`
	Error     string   `json:"error,omitempty"`
}
type Summary struct {
	StartedAt   string        `json:"started_at"`
	CompletedAt string        `json:"completed_at"`
	Results     []GroupResult `json:"results"`
}

// Run processes groups and events sequentially and uses generation preconditions
// to avoid overwriting changes made while the calendar was being fetched.
func Run(ctx context.Context, opts Options) (Summary, error) {
	summary := Summary{StartedAt: time.Now().UTC().Format(time.RFC3339)}
	if opts.Source == nil || opts.Store == nil || opts.Bucket == "" || len(opts.Targets) == 0 || !opts.From.Before(opts.Until) {
		return summary, errors.New("source, store, bucket, targets and a valid date range are required")
	}
	failed := false
	for _, target := range opts.Targets {
		r := GroupResult{Group: target.ID}
		events, err := opts.Source.ListEvents(ctx, target.CalendarID)
		if err == nil {
			r.Fetched = len(events)
			err = syncGroup(ctx, opts, target, events, &r)
		}
		if err != nil {
			r.Error = err.Error()
			failed = true
		}
		summary.Results = append(summary.Results, r)
	}
	summary.CompletedAt = time.Now().UTC().Format(time.RFC3339)
	if failed {
		return summary, errors.New("one or more calendars failed")
	}
	return summary, nil
}

func syncGroup(ctx context.Context, opts Options, target Target, events []timetree.Event, result *GroupResult) error {
	object := fmt.Sprintf("data/groups/%s/schedule.json", target.ID)
	for attempt := 0; attempt < 3; attempt++ {
		before, generation, err := opts.Store.GetObjectVersion(ctx, opts.Bucket, object)
		if err != nil {
			return fmt.Errorf("read %s: %w", object, err)
		}
		updated := before
		result.Processed, result.Changed, result.Conflicts = 0, 0, nil
		sort.SliceStable(events, func(i, j int) bool { return events[i].StartAt < events[j].StartAt })
		seen := make(map[string]bool)
		for _, event := range events {
			if event.ID == "" {
				result.Conflicts = append(result.Conflicts, "missing event ID")
				continue
			}
			if seen[event.ID] {
				continue
			}
			seen[event.ID] = true
			add, err := Extract(target.CalendarID, event)
			if err != nil {
				result.Conflicts = append(result.Conflicts, fmt.Sprintf("%s: %v", event.ID, err))
				continue
			}
			date, err := time.Parse("2006-01-02", add.Date)
			if err != nil || date.Before(opts.From) || !date.Before(opts.Until) {
				continue
			}
			id, err := existingSourceTarget(updated, event.ID, add.Date)
			if err != nil {
				result.Conflicts = append(result.Conflicts, fmt.Sprintf("%s: %v", event.ID, err))
				continue
			}
			add.TargetSessionID = id
			if len(event.ImageUrls) > 0 {
				add.ImageURL = event.ImageUrls[0]
			}
			candidate, id, err := schedule.ApplyAddSession(updated, add)
			if err == nil && len(event.ImageUrls) > 1 {
				for _, imageURL := range event.ImageUrls[1:] {
					candidate, _, err = schedule.ApplyAddSession(candidate, schedule.AddSessionOptions{Date: add.Date, TargetSessionID: id, PreserveExisting: true, ImageURL: imageURL})
					if err != nil {
						break
					}
				}
			}
			if err == nil {
				candidate, err = preserveRootFields(updated, candidate)
			}
			if err == nil {
				report, validateErr := validate.ValidateBytes(candidate)
				if validateErr != nil {
					err = validateErr
				} else if !report.IsValid {
					err = fmt.Errorf("invalid schedule: %v", report.Errors)
				}
			}
			if err != nil {
				result.Conflicts = append(result.Conflicts, fmt.Sprintf("%s: %v", event.ID, err))
				continue
			}
			changed, err := sessionsChanged(updated, candidate)
			if err != nil {
				return err
			}
			if changed {
				result.Changed++
			}
			updated = candidate
			result.Processed++
		}
		changed, err := sessionsChanged(before, updated)
		if err != nil {
			return err
		}
		if opts.DryRun {
			return nil
		}
		if !changed {
			if target.ID == "ribonclock" {
				_, err := readMirrors(ctx, opts, before)
				return err
			}
			return nil
		}
		var mirrorVersions map[string]string
		if target.ID == "ribonclock" {
			mirrorVersions, err = readMirrors(ctx, opts, before)
			if err != nil {
				return err
			}
		}
		err = opts.Store.PutObjectIfGeneration(ctx, opts.Bucket, object, "application/json; charset=utf-8", "no-cache, max-age=0", updated, generation)
		if errors.Is(err, gcs.ErrGenerationMismatch) {
			continue
		}
		if err != nil {
			return fmt.Errorf("write %s: %w", object, err)
		}
		result.Written = true
		if target.ID == "ribonclock" {
			for _, mirror := range []string{"schedule.json", "mock_schedule.json"} {
				if err := opts.Store.PutObjectIfGeneration(ctx, opts.Bucket, mirror, "application/json; charset=utf-8", "no-cache, max-age=0", updated, mirrorVersions[mirror]); err != nil {
					return fmt.Errorf("write mirror %s: %w", mirror, err)
				}
			}
		}
		return nil
	}
	return errors.New("schedule changed during all retry attempts")
}

func readMirrors(ctx context.Context, opts Options, primary []byte) (map[string]string, error) {
	versions := make(map[string]string)
	for _, mirror := range []string{"schedule.json", "mock_schedule.json"} {
		data, gen, err := opts.Store.GetObjectVersion(ctx, opts.Bucket, mirror)
		if err != nil {
			return nil, fmt.Errorf("read mirror %s: %w", mirror, err)
		}
		mismatch, err := sessionsChanged(primary, data)
		if err != nil {
			return nil, err
		}
		if mismatch {
			return nil, fmt.Errorf("mirror %s differs from primary; not overwritten", mirror)
		}
		versions[mirror] = gen
	}
	return versions, nil
}

// ApplyAddSession sorts sessions through its schedule model. Preserve root
// fields outside that model so synchronization cannot erase unrelated data.
func preserveRootFields(before, after []byte) ([]byte, error) {
	var original, changed map[string]json.RawMessage
	if err := json.Unmarshal(before, &original); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(after, &changed); err != nil {
		return nil, err
	}
	original["sessions"] = changed["sessions"]
	original["generated_at"] = changed["generated_at"]
	return json.MarshalIndent(original, "", "  ")
}

func existingSourceTarget(data []byte, sourceID, date string) (string, error) {
	var doc struct {
		Sessions []struct {
			ID     string `json:"id"`
			Date   string `json:"date"`
			Legacy string `json:"source_timetree_id"`
			Refs   []struct {
				Type string `json:"type"`
				ID   string `json:"source_id"`
			} `json:"source_refs"`
		} `json:"sessions"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return "", err
	}
	var found string
	for _, s := range doc.Sessions {
		match := s.Legacy == sourceID
		for _, ref := range s.Refs {
			if ref.Type == "timetree_event" && ref.ID == sourceID {
				match = true
			}
		}
		if !match {
			continue
		}
		if found != "" && found != s.ID {
			return "", errors.New("source ID matches multiple sessions")
		}
		if s.Date != date {
			return "", fmt.Errorf("event date changed from %s to %s", s.Date, date)
		}
		found = s.ID
	}
	return found, nil
}

func sessionsChanged(a, b []byte) (bool, error) {
	var left, right map[string]interface{}
	if err := json.Unmarshal(a, &left); err != nil {
		return false, err
	}
	if err := json.Unmarshal(b, &right); err != nil {
		return false, err
	}
	delete(left, "generated_at")
	delete(right, "generated_at")
	return !reflect.DeepEqual(left, right), nil
}
