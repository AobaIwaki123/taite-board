package reconcile

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/AobaIwaki123/taite-board/internal/gcs"
)

type memStore struct {
	mu      sync.Mutex
	objects map[string][]byte
}

func (m *memStore) GetObject(_ context.Context, _, object string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.objects[object]
	if !ok {
		return nil, gcs.ErrObjectNotFound
	}
	return b, nil
}

func (m *memStore) PutObject(_ context.Context, _, object, _, _ string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objects[object] = append([]byte(nil), data...)
	return nil
}

func (m *memStore) ListObjects(_ context.Context, _, _ string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var keys []string
	for k := range m.objects {
		keys = append(keys, k)
	}
	return keys, nil
}

func TestRun_MultiGroupReconcileAndMirrors(t *testing.T) {
	ribonJSON := `{
  "schema_version": 1,
  "generated_at": "2026-11-23T00:00:00+09:00",
  "timezone": "Asia/Tokyo",
  "target_group": { "name": "リボンクロック", "canonical_id": "group:ribbon-clock" },
  "sessions": [
    {
      "id": "20261123-1",
      "title": "🎀ギガ盛りMAX🎀",
      "date": "2026-11-23",
      "day_of_week": "月祝",
      "venue": { "canonical_id": "", "name": "ZEAL THEATER TOKYO" },
      "live": { "start_time": "--:--", "end_time": "", "stage": null },
      "benefit": { "type": "並行", "start_time": "--:--", "end_time": "", "booth": "ロビ-B" },
      "ticket": { "platform": "", "url": "https://livepocket.jp/e/x8-ap", "prices": [] },
      "source_tweet_url": "https://x.com/Ribbon_Clock/status/111"
    },
    {
      "id": "20261123-2",
      "title": "ギガ盛りMAX",
      "date": "2026-11-23",
      "day_of_week": "月祝",
      "venue": { "canonical_id": "venue:zeal-theater-shimbashi", "name": "ZEAL THEATER 新橋" },
      "live": { "start_time": "13:15", "end_time": "13:35", "stage": null },
      "benefit": { "type": "並行", "start_time": "13:40", "end_time": "14:40", "booth": "並行A" },
      "ticket": { "platform": "LivePocket", "url": "https://livepocket.jp/e/x8-ap", "prices": [] },
      "source_tweet_url": "https://x.com/Ribbon_Clock/status/222"
    }
  ]
}`
	dollyJSON := `{
  "schema_version": 1,
  "generated_at": "2026-11-23T00:00:00+09:00",
  "timezone": "Asia/Tokyo",
  "target_group": { "name": "Dolly Kiss", "canonical_id": "group:dolly-kiss" },
  "sessions": [
    {
      "id": "dollykiss-20261123-1",
      "title": "ギガ盛りMAX",
      "date": "2026-11-23",
      "day_of_week": "月祝",
      "venue": { "canonical_id": "", "name": "ZEAL THEATER TOKYO" },
      "live": { "start_time": "15:00", "end_time": "15:20", "stage": null },
      "benefit": { "type": "並行", "start_time": "15:30", "end_time": "16:30", "booth": "B" },
      "ticket": { "platform": "", "url": "https://tiget.net/events/555", "prices": [] },
      "source_tweet_url": "https://x.com/DollyKiss_staff/status/999"
    }
  ]
}`

	store := &memStore{
		objects: map[string][]byte{
			"data/groups/ribonclock/schedule.json": []byte(ribonJSON),
			"data/groups/dollykiss/schedule.json":  []byte(dollyJSON),
		},
	}

	// DryRun first should not write anything
	drySum, err := Run(context.Background(), RunOptions{
		Store:  store,
		Bucket: "taite-board-data",
		Group:  "all",
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("dry run failed: %v", err)
	}
	if !drySum.AnyChanged || len(drySum.Results) != 2 {
		t.Fatalf("unexpected dry run summary: %+v", drySum)
	}
	if _, err := store.GetObject(context.Background(), "taite-board-data", "schedule.json"); !errors.Is(err, gcs.ErrObjectNotFound) {
		t.Fatalf("expected dry run not to write schedule.json")
	}

	// Real run should reconcile both groups and mirror ribonclock root objects
	sum, err := Run(context.Background(), RunOptions{
		Store:         store,
		Bucket:        "taite-board-data",
		Group:         "all",
		DryRun:        false,
		TriggerSource: "unit-test",
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if !sum.AnyChanged || !sum.Results[0].Written || !sum.Results[1].Written {
		t.Fatalf("expected both groups written, got %+v", sum.Results)
	}

	mirrorData, err := store.GetObject(context.Background(), "taite-board-data", "schedule.json")
	if err != nil {
		t.Fatalf("expected schedule.json mirrored: %v", err)
	}
	var doc struct {
		Sessions []map[string]any `json:"sessions"`
	}
	if err := json.Unmarshal(mirrorData, &doc); err != nil {
		t.Fatalf("unmarshal mirrored schedule: %v", err)
	}
	if len(doc.Sessions) != 1 {
		t.Errorf("expected 1 merged session in mirrored schedule.json, got %d", len(doc.Sessions))
	}
}
