package scrape

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AobaIwaki123/taite-board/internal/candidate"
	"github.com/AobaIwaki123/taite-board/internal/gcs"
	"github.com/AobaIwaki123/taite-board/internal/raw"
	"github.com/AobaIwaki123/taite-board/internal/source/x"
)

const (
	// DefaultGCSStateObject stores scrape checkpoints and staged review items in GCS state bucket.
	DefaultGCSStateObject = "scrape/state.json"
	maxTrackedIDs         = 500
	maxPendingItems       = 200
)

// PendingStatus represents the review lifecycle of a staged scrape candidate.
type PendingStatus string

const (
	StatusPending   PendingStatus = "pending"
	StatusAmbiguous PendingStatus = "ambiguous"
	StatusPromoted  PendingStatus = "promoted"
	StatusDismissed PendingStatus = "dismissed"
)

// GroupCheckpoint tracks the crawl cursor and processed deduplication keys for one group.
type GroupCheckpoint struct {
	LatestSinceID         string   `json:"latest_since_id,omitempty"`
	ProcessedIDs          []string `json:"processed_ids,omitempty"`
	ProcessedFingerprints []string `json:"processed_fingerprints,omitempty"`
	UpdatedAt             string   `json:"updated_at,omitempty"`
	LastRunSource         string   `json:"last_run_source,omitempty"`
	LastError             string   `json:"last_error,omitempty"`
}

// PendingItem represents a filtered, resolved post awaiting human confirmation or manual target selection.
type PendingItem struct {
	ID             string                   `json:"id"` // "<group>:<source_id>"
	Group          string                   `json:"group"`
	Status         PendingStatus            `json:"status"`
	CreatedAt      string                   `json:"created_at"`
	UpdatedAt      string                   `json:"updated_at"`
	RawFingerprint string                   `json:"raw_fingerprint"`
	NeedsAI        bool                     `json:"needs_ai"`
	Post           *raw.Post                `json:"post"`
	Candidate      candidate.EventCandidate `json:"candidate"`
	Evidence       []candidate.Evidence     `json:"evidence,omitempty"`
	Error          string                   `json:"error,omitempty"`
}

// StateDoc is the persisted scrape state across groups.
type StateDoc struct {
	SchemaVersion int                         `json:"schema_version"`
	UpdatedAt     string                      `json:"updated_at"`
	Checkpoints   map[string]*GroupCheckpoint `json:"checkpoints"`
	PendingQueue  []PendingItem               `json:"pending_queue"`
}

// StateStore abstracts loading and saving scrape state locally or in GCS.
type StateStore interface {
	Load(ctx context.Context) (*StateDoc, error)
	Save(ctx context.Context, doc *StateDoc) error
}

// NewEmptyStateDoc initializes an empty StateDoc with non-nil maps/slices.
func NewEmptyStateDoc() *StateDoc {
	return &StateDoc{
		SchemaVersion: 1,
		Checkpoints:   make(map[string]*GroupCheckpoint),
		PendingQueue:  []PendingItem{},
	}
}

func normalizeStateDoc(doc *StateDoc) *StateDoc {
	if doc == nil {
		return NewEmptyStateDoc()
	}
	if doc.SchemaVersion < 1 {
		doc.SchemaVersion = 1
	}
	if doc.Checkpoints == nil {
		doc.Checkpoints = make(map[string]*GroupCheckpoint)
	}
	if doc.PendingQueue == nil {
		doc.PendingQueue = []PendingItem{}
	}
	return doc
}

// FileStateStore persists StateDoc as JSON on local disk using atomic rename.
type FileStateStore struct {
	Path string
}

// Load reads StateDoc from disk, returning an empty StateDoc when missing.
func (s *FileStateStore) Load(_ context.Context) (*StateDoc, error) {
	if strings.TrimSpace(s.Path) == "" {
		return NewEmptyStateDoc(), nil
	}
	data, err := os.ReadFile(s.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewEmptyStateDoc(), nil
		}
		return nil, fmt.Errorf("reading scrape state %s: %w", s.Path, err)
	}
	var doc StateDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parsing scrape state %s: %w", s.Path, err)
	}
	return normalizeStateDoc(&doc), nil
}

// Save writes StateDoc atomically to local disk.
func (s *FileStateStore) Save(_ context.Context, doc *StateDoc) error {
	if strings.TrimSpace(s.Path) == "" {
		return nil
	}
	doc = normalizeStateDoc(doc)
	doc.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := os.MkdirAll(filepath.Dir(s.Path), 0755); err != nil {
		return fmt.Errorf("creating scrape state dir: %w", err)
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling scrape state: %w", err)
	}
	tmp := fmt.Sprintf("%s.tmp.%d", s.Path, time.Now().UnixNano())
	if err := os.WriteFile(tmp, append(data, '\n'), 0644); err != nil {
		return fmt.Errorf("writing temp scrape state: %w", err)
	}
	return os.Rename(tmp, s.Path)
}

// GCSStateStore persists StateDoc in GCS.
type GCSStateStore struct {
	Store  gcs.ObjectStore
	Bucket string
	Object string
}

// Load fetches StateDoc from GCS, returning an empty StateDoc on ErrObjectNotFound.
func (s *GCSStateStore) Load(ctx context.Context) (*StateDoc, error) {
	obj := s.Object
	if obj == "" {
		obj = DefaultGCSStateObject
	}
	data, err := s.Store.GetObject(ctx, s.Bucket, obj)
	if err != nil {
		if errors.Is(err, gcs.ErrObjectNotFound) {
			return NewEmptyStateDoc(), nil
		}
		return nil, fmt.Errorf("reading gs://%s/%s: %w", s.Bucket, obj, err)
	}
	var doc StateDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parsing gs://%s/%s: %w", s.Bucket, obj, err)
	}
	return normalizeStateDoc(&doc), nil
}

// Save uploads StateDoc to GCS with no-cache headers.
func (s *GCSStateStore) Save(ctx context.Context, doc *StateDoc) error {
	obj := s.Object
	if obj == "" {
		obj = DefaultGCSStateObject
	}
	doc = normalizeStateDoc(doc)
	doc.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling scrape state: %w", err)
	}
	return s.Store.PutObject(ctx, s.Bucket, obj, "application/json; charset=utf-8", "no-cache, no-store, must-revalidate", append(data, '\n'))
}

// LoadKnownKeysFromScheduleBytes extracts known X post IDs already committed into schedule.json.
func LoadKnownKeysFromScheduleBytes(rawSchedule []byte) map[string]bool {
	keys := make(map[string]bool)
	if len(rawSchedule) == 0 {
		return keys
	}
	var doc struct {
		Sessions []struct {
			SourceTweetURL string `json:"source_tweet_url"`
			SourceRefs     []struct {
				SourceID string `json:"source_id"`
				URL      string `json:"url"`
			} `json:"source_refs"`
		} `json:"sessions"`
	}
	if err := json.Unmarshal(rawSchedule, &doc); err != nil {
		return keys
	}
	for _, sess := range doc.Sessions {
		if sid := x.ExtractStatusID(sess.SourceTweetURL); sid != "" {
			keys[sid] = true
		}
		for _, ref := range sess.SourceRefs {
			if ref.SourceID != "" {
				keys[ref.SourceID] = true
			}
			if sid := x.ExtractStatusID(ref.URL); sid != "" {
				keys[sid] = true
			}
		}
	}
	return keys
}

func appendBoundedUnique(existing []string, addition string, limit int) []string {
	if addition == "" {
		return existing
	}
	for _, v := range existing {
		if v == addition {
			return existing
		}
	}
	out := append(existing, addition)
	if limit > 0 && len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out
}

// UpsertPending inserts or updates a staged candidate while preserving dismissed/promoted terminal states.
func (doc *StateDoc) UpsertPending(item PendingItem) {
	for i := range doc.PendingQueue {
		if doc.PendingQueue[i].ID == item.ID {
			if doc.PendingQueue[i].Status == StatusDismissed || doc.PendingQueue[i].Status == StatusPromoted {
				return
			}
			item.CreatedAt = doc.PendingQueue[i].CreatedAt
			doc.PendingQueue[i] = item
			return
		}
	}
	doc.PendingQueue = append([]PendingItem{item}, doc.PendingQueue...)
	if len(doc.PendingQueue) > maxPendingItems {
		doc.PendingQueue = doc.PendingQueue[:maxPendingItems]
	}
}

// UpdatePendingStatus marks a staged item (e.g. promoted or dismissed) and returns true if found.
func (doc *StateDoc) UpdatePendingStatus(id string, status PendingStatus) bool {
	now := time.Now().UTC().Format(time.RFC3339)
	for i := range doc.PendingQueue {
		if doc.PendingQueue[i].ID == id {
			doc.PendingQueue[i].Status = status
			doc.PendingQueue[i].UpdatedAt = now
			return true
		}
	}
	return false
}
