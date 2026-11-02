package monitor

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/AobaIwaki123/taite-board/internal/candidate"
	"github.com/AobaIwaki123/taite-board/internal/telemetry"
)

// LogEntry represents an anonymous Nginx access log entry.
type LogEntry struct {
	Timestamp string `json:"ts"`
	Route     string `json:"route"`
	Status    int    `json:"status"`
}

// RequestRecord stores a single recent request record.
type RequestRecord struct {
	Time   string `json:"time"`
	Route  string `json:"route"`
	Status int    `json:"status"`
}

// HourlyCount stores count of requests for a specific hour.
type HourlyCount struct {
	Hour          string `json:"hour"`  // "15:00"
	TotalRequests int64  `json:"total_requests"`
	PageViews     int64  `json:"page_views"`
	Count         int64  `json:"count"` // backward compatible alias
}

// StatsSnapshot contains full aggregated metrics for API and Dashboard.
type StatsSnapshot struct {
	Version        string          `json:"version"`
	UptimeSeconds  int64           `json:"uptime_seconds"`
	TotalRequests  int64           `json:"total_requests"`
	TodayRequests  int64           `json:"today_requests"`
	TotalPageViews int64           `json:"total_page_views"`
	TodayPageViews int64           `json:"today_page_views"`
	LastHourCount  int64           `json:"last_hour_count"`
	ErrorRate      float64         `json:"error_rate"`
	ByRoute        map[string]int64 `json:"by_route"`
	ByStatus       map[string]int64 `json:"by_status"`
	Hourly         []HourlyCount   `json:"hourly"`
	RecentRequests []RequestRecord          `json:"recent_requests"`
	RecentIntakes  []candidate.IntakeResult `json:"recent_intakes"`
	Pipeline       telemetry.PipelineStats  `json:"pipeline"`
}

// Collector collects, aggregates, and stores access statistics in memory.
type Collector struct {
	mu            sync.RWMutex
	version       string
	startTime     time.Time
	totalRequests int64

	byRouteStatus map[string]int64 // "document:200" -> count
	byRoute       map[string]int64 // "document" -> count
	byStatusGroup map[string]int64 // "2xx", "3xx", "4xx", "5xx" -> count
	hourly        map[string]int64 // "2006-01-02 15:00" -> total count
	hourlyRoute   map[string]int64 // "route:2006-01-02 15:00" -> route count
	recent        []RequestRecord  // ring buffer (max 50)
	maxRecent     int
	recentIntakes []candidate.IntakeResult // ring buffer (max 20)
	maxIntakes    int
	intakesPath   string

	dataDir            string
	stateFilePath      string
	pipelineStats      telemetry.PipelineStats
	lastPipelineScan   time.Time
	pipelineScanPeriod time.Duration
}

// NewCollector creates a new Collector instance.
func NewCollector(version string) *Collector {
	return &Collector{
		version:            version,
		startTime:          time.Now(),
		byRouteStatus:      make(map[string]int64),
		byRoute:            make(map[string]int64),
		byStatusGroup:      make(map[string]int64),
		hourly:             make(map[string]int64),
		hourlyRoute:        make(map[string]int64),
		recent:             make([]RequestRecord, 0, 50),
		maxRecent:          50,
		recentIntakes:      make([]candidate.IntakeResult, 0, 20),
		maxIntakes:         20,
		pipelineScanPeriod: 10 * time.Second,
	}
}

// SetDataDir sets the repository data directory for reading pipeline telemetry.
func (c *Collector) SetDataDir(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.dataDir = path
	c.doRefreshPipelineStatsLocked()
}

// SetStateFilePath sets the persistence path for pipeline state retention across restarts.
func (c *Collector) SetStateFilePath(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stateFilePath = path
	c.doRefreshPipelineStatsLocked()
}

// SetPipelineStats explicitly injects pipeline telemetry (useful for testing or direct event push).
func (c *Collector) SetPipelineStats(stats telemetry.PipelineStats) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pipelineStats = stats
	c.lastPipelineScan = time.Now()
}

func (c *Collector) refreshPipelineStatsLocked() {
	if c.dataDir == "" && c.stateFilePath == "" {
		return
	}
	if time.Since(c.lastPipelineScan) < c.pipelineScanPeriod {
		return
	}
	c.doRefreshPipelineStatsLocked()
}

func (c *Collector) doRefreshPipelineStatsLocked() {
	stats, err := telemetry.CollectAndPersist(c.dataDir, c.stateFilePath)
	if err == nil {
		c.pipelineStats = stats
	}
	c.lastPipelineScan = time.Now()
}

// SetIntakesPath sets the persistence path for intake items and loads existing records if present.
func (c *Collector) SetIntakesPath(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.intakesPath = path
	if path == "" {
		return
	}
	data, err := os.ReadFile(path)
	if err == nil {
		var loaded []candidate.IntakeResult
		if err := json.Unmarshal(data, &loaded); err == nil && len(loaded) > 0 {
			c.recentIntakes = loaded
			if len(c.recentIntakes) > c.maxIntakes {
				c.recentIntakes = c.recentIntakes[:c.maxIntakes]
			}
		}
	}
}

// AddIntake records an intake result to the recent intakes list and persists it to disk.
func (c *Collector) AddIntake(res candidate.IntakeResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.recentIntakes = append([]candidate.IntakeResult{res}, c.recentIntakes...)
	if len(c.recentIntakes) > c.maxIntakes {
		c.recentIntakes = c.recentIntakes[:c.maxIntakes]
	}
	if c.intakesPath != "" {
		if data, err := json.MarshalIndent(c.recentIntakes, "", "  "); err == nil {
			_ = os.WriteFile(c.intakesPath, data, 0644)
		}
	}
}

// UpdateIntakeAIResult updates an existing intake record with Cloud Run AI Vision results.
func (c *Collector) UpdateIntakeAIResult(sourceID string, cand candidate.EventCandidate, evidence []candidate.Evidence, modelID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	found := false
	for i := range c.recentIntakes {
		if c.recentIntakes[i].SourceID == sourceID {
			if cand.LiveStartRaw != "" {
				c.recentIntakes[i].Candidate.LiveStartRaw = cand.LiveStartRaw
			}
			if cand.LiveEndRaw != "" {
				c.recentIntakes[i].Candidate.LiveEndRaw = cand.LiveEndRaw
			}
			if cand.BenefitStartRaw != "" {
				c.recentIntakes[i].Candidate.BenefitStartRaw = cand.BenefitStartRaw
			}
			if cand.BenefitEndRaw != "" {
				c.recentIntakes[i].Candidate.BenefitEndRaw = cand.BenefitEndRaw
			}
			if cand.Date != "" {
				c.recentIntakes[i].Candidate.Date = cand.Date
			}
			if cand.VenueRaw != "" {
				c.recentIntakes[i].Candidate.VenueRaw = cand.VenueRaw
			}
			c.recentIntakes[i].Evidence = append(c.recentIntakes[i].Evidence, evidence...)
			c.recentIntakes[i].Status = candidate.StatusCompleted
			c.recentIntakes[i].NeedsAI = false
			found = true
			break
		}
	}

	// Increment pipeline AI dispatched counter
	c.pipelineStats.AIDispatched++
	if c.stateFilePath != "" {
		_ = telemetry.SavePipelineState(c.stateFilePath, c.pipelineStats)
	}

	if found && c.intakesPath != "" {
		if data, err := json.MarshalIndent(c.recentIntakes, "", "  "); err == nil {
			_ = os.WriteFile(c.intakesPath, data, 0644)
		}
	}
}

// RecentIntakes returns a copy of recent intake records.
func (c *Collector) RecentIntakes() []candidate.IntakeResult {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]candidate.IntakeResult, len(c.recentIntakes))
	copy(out, c.recentIntakes)
	return out
}

// ProcessLine parses a single JSON line and updates metrics.
func (c *Collector) ProcessLine(line string) error {
	line = strings.TrimSpace(line)
	if line == "" || !strings.HasPrefix(line, "{") {
		return nil
	}

	var entry LogEntry
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.totalRequests++

	route := entry.Route
	if route == "" {
		route = "other"
	}
	c.byRoute[route]++

	statusGroup := "other"
	switch {
	case entry.Status >= 200 && entry.Status < 300:
		statusGroup = "2xx"
	case entry.Status >= 300 && entry.Status < 400:
		statusGroup = "3xx"
	case entry.Status >= 400 && entry.Status < 500:
		statusGroup = "4xx"
	case entry.Status >= 500 && entry.Status < 600:
		statusGroup = "5xx"
	}
	c.byStatusGroup[statusGroup]++

	key := fmt.Sprintf("%s:%d", route, entry.Status)
	c.byRouteStatus[key]++

	// Parse timestamp for hourly bucket (Asia/Tokyo)
	jst := time.FixedZone("Asia/Tokyo", 9*60*60)
	t, err := time.Parse(time.RFC3339, entry.Timestamp)
	if err != nil {
		t = time.Now().In(jst)
	} else {
		t = t.In(jst)
	}

	hourKey := t.Format("2006-01-02 15:00")
	c.hourly[hourKey]++
	c.hourlyRoute[fmt.Sprintf("%s:%s", route, hourKey)]++

	// Append to recent ring buffer
	rec := RequestRecord{
		Time:   t.Format("15:04:05"),
		Route:  route,
		Status: entry.Status,
	}
	if len(c.recent) >= c.maxRecent {
		c.recent = append(c.recent[1:], rec)
	} else {
		c.recent = append(c.recent, rec)
	}

	return nil
}

// Snapshot returns a copy of current aggregated statistics.
func (c *Collector) Snapshot() StatsSnapshot {
	c.mu.Lock()
	c.refreshPipelineStatsLocked()
	c.mu.Unlock()

	c.mu.RLock()
	defer c.mu.RUnlock()

	jst := time.FixedZone("Asia/Tokyo", 9*60*60)
	now := time.Now().In(jst)
	todayPrefix := now.Format("2006-01-02")
	lastHourKey := now.Format("2006-01-02 15:00")

	var todayCount int64
	var todayPV int64
	for k, count := range c.hourly {
		if strings.HasPrefix(k, todayPrefix) {
			todayCount += count
		}
	}
	for k, count := range c.hourlyRoute {
		if strings.HasPrefix(k, "document:"+todayPrefix) {
			todayPV += count
		}
	}

	// Calculate error rate (4xx + 5xx)
	var errCount int64
	errCount += c.byStatusGroup["4xx"]
	errCount += c.byStatusGroup["5xx"]
	var errorRate float64
	if c.totalRequests > 0 {
		errorRate = float64(errCount) / float64(c.totalRequests) * 100.0
	}

	// Build last 24 hours timeline
	hourlyList := make([]HourlyCount, 0, 24)
	for i := 23; i >= 0; i-- {
		targetHour := now.Add(-time.Duration(i) * time.Hour)
		hKey := targetHour.Format("2006-01-02 15:00")
		dispKey := targetHour.Format("15:00")
		tCount := c.hourly[hKey]
		pvCount := c.hourlyRoute["document:"+hKey]
		hourlyList = append(hourlyList, HourlyCount{
			Hour:          dispKey,
			TotalRequests: tCount,
			PageViews:     pvCount,
			Count:         tCount,
		})
	}

	// Clone byRoute map
	routes := make(map[string]int64, len(c.byRoute))
	for k, v := range c.byRoute {
		routes[k] = v
	}

	// Clone byStatusGroup map
	statuses := make(map[string]int64, len(c.byStatusGroup))
	for k, v := range c.byStatusGroup {
		statuses[k] = v
	}

	// Clone recent (reverse order for latest first)
	recentReversed := make([]RequestRecord, len(c.recent))
	for i, r := range c.recent {
		recentReversed[len(c.recent)-1-i] = r
	}

	// Clone recent intakes
	intakesCopy := make([]candidate.IntakeResult, len(c.recentIntakes))
	copy(intakesCopy, c.recentIntakes)

	return StatsSnapshot{
		Version:        c.version,
		UptimeSeconds:  int64(time.Since(c.startTime).Seconds()),
		TotalRequests:  c.totalRequests,
		TodayRequests:  todayCount,
		TotalPageViews: c.byRoute["document"],
		TodayPageViews: todayPV,
		LastHourCount:  c.hourly[lastHourKey],
		ErrorRate:      errorRate,
		ByRoute:        routes,
		ByStatus:       statuses,
		Hourly:         hourlyList,
		RecentRequests: recentReversed,
		RecentIntakes:  intakesCopy,
		Pipeline:       c.pipelineStats,
	}
}

// PrometheusMetrics formats the counters in Prometheus text exposition format.
func (c *Collector) PrometheusMetrics() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var sb strings.Builder

	sb.WriteString("# HELP ribonclock_http_requests_total Total HTTP requests handled by Ribbon Clock Nginx.\n")
	sb.WriteString("# TYPE ribonclock_http_requests_total counter\n")

	keys := make([]string, 0, len(c.byRouteStatus))
	for k := range c.byRouteStatus {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		parts := strings.Split(k, ":")
		if len(parts) == 2 {
			sb.WriteString(fmt.Sprintf("ribonclock_http_requests_total{route=\"%s\",status=\"%s\"} %d\n",
				parts[0], parts[1], c.byRouteStatus[k]))
		}
	}

	sb.WriteString("\n# HELP ribonclock_monitor_uptime_seconds Uptime of the access monitor service.\n")
	sb.WriteString("# TYPE ribonclock_monitor_uptime_seconds gauge\n")
	sb.WriteString(fmt.Sprintf("ribonclock_monitor_uptime_seconds %d\n", int64(time.Since(c.startTime).Seconds())))

	// Pipeline Ingestion & Cost Shield Funnel Metrics
	sb.WriteString("\n# HELP ribonclock_pipeline_detected_posts_total Total raw posts detected across crawl runs.\n")
	sb.WriteString("# TYPE ribonclock_pipeline_detected_posts_total counter\n")
	sb.WriteString(fmt.Sprintf("ribonclock_pipeline_detected_posts_total %d\n", c.pipelineStats.DetectedPosts))

	sb.WriteString("\n# HELP ribonclock_pipeline_filtered_posts_total Total posts filtered out by local pre-filter stage.\n")
	sb.WriteString("# TYPE ribonclock_pipeline_filtered_posts_total counter\n")
	sb.WriteString(fmt.Sprintf("ribonclock_pipeline_filtered_posts_total{stage=\"media\"} %d\n", c.pipelineStats.FilterBreakdown.Media))
	sb.WriteString(fmt.Sprintf("ribonclock_pipeline_filtered_posts_total{stage=\"age\"} %d\n", c.pipelineStats.FilterBreakdown.Age))
	sb.WriteString(fmt.Sprintf("ribonclock_pipeline_filtered_posts_total{stage=\"keyword\"} %d\n", c.pipelineStats.FilterBreakdown.Keyword))
	sb.WriteString(fmt.Sprintf("ribonclock_pipeline_filtered_posts_total{stage=\"duplicate\"} %d\n", c.pipelineStats.FilterBreakdown.Duplicate))

	sb.WriteString("\n# HELP ribonclock_pipeline_ai_dispatched_total Total candidate posts dispatched to Cloud Run AI parser.\n")
	sb.WriteString("# TYPE ribonclock_pipeline_ai_dispatched_total counter\n")
	sb.WriteString(fmt.Sprintf("ribonclock_pipeline_ai_dispatched_total %d\n", c.pipelineStats.AIDispatched))

	sb.WriteString("\n# HELP ribonclock_pipeline_ai_cache_hits_total Total AI parser invocations saved via fingerprint cache.\n")
	sb.WriteString("# TYPE ribonclock_pipeline_ai_cache_hits_total counter\n")
	sb.WriteString(fmt.Sprintf("ribonclock_pipeline_ai_cache_hits_total %d\n", c.pipelineStats.AICacheHits))

	sb.WriteString("\n# HELP ribonclock_pipeline_promoted_sessions_total Total timetable sessions promoted to schedule.json.\n")
	sb.WriteString("# TYPE ribonclock_pipeline_promoted_sessions_total gauge\n")
	sb.WriteString(fmt.Sprintf("ribonclock_pipeline_promoted_sessions_total %d\n", c.pipelineStats.PromotedSessions))

	sb.WriteString("\n# HELP ribonclock_pipeline_estimated_cost_saved_usd Cumulative estimated API cost saved in USD via local filtering.\n")
	sb.WriteString("# TYPE ribonclock_pipeline_estimated_cost_saved_usd gauge\n")
	sb.WriteString(fmt.Sprintf("ribonclock_pipeline_estimated_cost_saved_usd %.4f\n", c.pipelineStats.EstimatedCostSavedUSD))

	return sb.String()
}
