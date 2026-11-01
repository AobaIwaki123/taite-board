package monitor

import (
	"strings"
	"testing"

	"github.com/AobaIwaki123/taite-board/internal/candidate"
)

func TestCollector_ProcessLine(t *testing.T) {
	c := NewCollector("1.0.0")

	// Sample JSON log lines
	lines := []string{
		`{"ts":"2026-11-01T15:00:00+09:00","route":"document","status":200}`,
		`{"ts":"2026-11-01T15:05:00+09:00","route":"schedule","status":200}`,
		`{"ts":"2026-11-01T15:10:00+09:00","route":"asset","status":304}`,
		`{"ts":"2026-11-01T15:15:00+09:00","route":"other","status":404}`,
		``, // empty line
		`invalid json`,
	}

	for _, l := range lines {
		_ = c.ProcessLine(l)
	}

	snap := c.Snapshot()
	if snap.TotalRequests != 4 {
		t.Fatalf("expected 4 total requests, got %d", snap.TotalRequests)
	}
	if snap.TotalPageViews != 1 {
		t.Errorf("expected 1 total page views, got %d", snap.TotalPageViews)
	}
	if snap.TodayPageViews != 1 {
		t.Errorf("expected 1 today page views, got %d", snap.TodayPageViews)
	}
	if snap.ByRoute["document"] != 1 {
		t.Errorf("expected 1 document request, got %d", snap.ByRoute["document"])
	}
	if snap.ByRoute["schedule"] != 1 {
		t.Errorf("expected 1 schedule request, got %d", snap.ByRoute["schedule"])
	}
	if snap.ByStatus["2xx"] != 2 {
		t.Errorf("expected 2 2xx requests, got %d", snap.ByStatus["2xx"])
	}
	if snap.ByStatus["4xx"] != 1 {
		t.Errorf("expected 1 4xx request, got %d", snap.ByStatus["4xx"])
	}
	if snap.ErrorRate != 25.0 {
		t.Errorf("expected 25.0%% error rate, got %f", snap.ErrorRate)
	}

	prom := c.PrometheusMetrics()
	if !strings.Contains(prom, `ribonclock_http_requests_total{route="document",status="200"} 1`) {
		t.Errorf("prometheus metrics missing document:200 counter:\n%s", prom)
	}
	if !strings.Contains(prom, `ribonclock_http_requests_total{route="other",status="404"} 1`) {
		t.Errorf("prometheus metrics missing other:404 counter:\n%s", prom)
	}
}

func TestCollector_IntakesPersistence(t *testing.T) {
	tempFile := t.TempDir() + "/intakes.json"
	c1 := NewCollector("1.0.0")
	c1.SetIntakesPath(tempFile)

	c1.AddIntake(candidate.IntakeResult{
		Source:   "x",
		SourceID: "112233",
		Text:     "First post",
	})
	c1.AddIntake(candidate.IntakeResult{
		Source:   "x",
		SourceID: "445566",
		Text:     "Second post",
	})

	if len(c1.RecentIntakes()) != 2 {
		t.Fatalf("expected 2 intakes in c1, got %d", len(c1.RecentIntakes()))
	}

	// Create new collector simulating pod restart
	c2 := NewCollector("1.0.0")
	c2.SetIntakesPath(tempFile)

	intakes := c2.RecentIntakes()
	if len(intakes) != 2 {
		t.Fatalf("expected 2 intakes reloaded in c2 after restart, got %d", len(intakes))
	}
	if intakes[0].SourceID != "445566" {
		t.Errorf("expected newest intake first (445566), got %s", intakes[0].SourceID)
	}
}
