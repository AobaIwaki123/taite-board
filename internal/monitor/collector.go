package monitor

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/AobaIwaki123/taite-board/internal/candidate"
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
	RecentRequests []RequestRecord        `json:"recent_requests"`
	RecentIntakes  []candidate.IntakeResult `json:"recent_intakes"`
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
}

// NewCollector creates a new Collector instance.
func NewCollector(version string) *Collector {
	return &Collector{
		version:       version,
		startTime:     time.Now(),
		byRouteStatus: make(map[string]int64),
		byRoute:       make(map[string]int64),
		byStatusGroup: make(map[string]int64),
		hourly:        make(map[string]int64),
		hourlyRoute:   make(map[string]int64),
		recent:        make([]RequestRecord, 0, 50),
		maxRecent:     50,
		recentIntakes: make([]candidate.IntakeResult, 0, 20),
		maxIntakes:    20,
	}
}

// AddIntake records an intake result to the recent intakes list.
func (c *Collector) AddIntake(res candidate.IntakeResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.recentIntakes = append([]candidate.IntakeResult{res}, c.recentIntakes...)
	if len(c.recentIntakes) > c.maxIntakes {
		c.recentIntakes = c.recentIntakes[:c.maxIntakes]
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

	return sb.String()
}
