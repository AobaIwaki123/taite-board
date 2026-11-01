package monitor

import (
	"strings"
	"testing"
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
