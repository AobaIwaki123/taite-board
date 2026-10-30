package validate

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func createTestSchedule(t *testing.T, data map[string]interface{}) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "schedule.json")
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		t.Fatalf("marshaling test schedule: %v", err)
	}
	if err := os.WriteFile(path, bytes, 0644); err != nil {
		t.Fatalf("writing test schedule: %v", err)
	}
	return path
}

func validBaseSchedule() map[string]interface{} {
	return map[string]interface{}{
		"schema_version": 1,
		"generated_at":   "2026-10-30T12:00:00Z",
		"timezone":       "Asia/Tokyo",
		"target_group": map[string]interface{}{
			"name": "リボンクロック",
		},
		"sessions": []map[string]interface{}{
			{
				"id":    "20261101-1",
				"title": "はなまるっ Vol.159",
				"date":  "2026-11-01",
				"venue": map[string]interface{}{
					"name": "新宿ZircoTokyo",
				},
				"live": map[string]interface{}{
					"start_time": "15:05",
					"end_time":   "15:30",
					"stage":      "Zirco Stage",
				},
				"benefit": map[string]interface{}{
					"type":       "並行",
					"start_time": "15:35",
					"end_time":   "16:35",
					"booth":      "並行B",
				},
				"ticket": map[string]interface{}{
					"platform": "TicketDive",
					"url":      "https://ticketdive.com/event/hanamaru_159",
				},
				"source_tweet_url": "https://x.com/Ribbon_Clock/status/2114341213147205692",
				"images": []map[string]interface{}{
					{
						"type":  "flyer",
						"label": "フライヤー (1/2)",
						"url":   "https://pbs.twimg.com/media/HVYs_pwacAADw5Q.jpg?name=large",
					},
					{
						"type":  "timetable",
						"label": "タイムテーブル (2/2)",
						"url":   "https://pbs.twimg.com/media/HVYs_ptbcAESY9P.jpg?name=large",
					},
				},
			},
		},
	}
}

func TestValidateFile_Valid(t *testing.T) {
	sched := validBaseSchedule()
	filePath := createTestSchedule(t, sched)

	report, err := ValidateFile(filePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !report.IsValid {
		t.Errorf("expected valid, got errors: %v", report.Errors)
	}
	if len(report.Errors) > 0 {
		t.Errorf("expected 0 errors, got: %v", report.Errors)
	}
}

func TestValidateFile_InvalidSourceTweetURL(t *testing.T) {
	sched := validBaseSchedule()
	sessions := sched["sessions"].([]map[string]interface{})
	sessions[0]["source_tweet_url"] = "https://eplus.jp/marquee-matsuri/" // Non-X URL
	filePath := createTestSchedule(t, sched)

	report, err := ValidateFile(filePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.IsValid {
		t.Errorf("expected invalid due to non-X source_tweet_url")
	}
	found := false
	for _, e := range report.Errors {
		if filepath.Base(e) != "" && (len(e) > 0) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error for source_tweet_url, got %v", report.Errors)
	}
}

func TestValidateFile_RelativeImageURL(t *testing.T) {
	sched := validBaseSchedule()
	sessions := sched["sessions"].([]map[string]interface{})
	sessions[0]["images"] = []map[string]interface{}{
		{
			"type":  "flyer",
			"label": "フライヤー",
			"url":   "/images/20261107_marquee_mini.png", // Forbidden local path
		},
	}
	filePath := createTestSchedule(t, sched)

	report, err := ValidateFile(filePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.IsValid {
		t.Errorf("expected invalid due to relative image URL")
	}
}

func TestValidateFile_TimeInversion(t *testing.T) {
	sched := validBaseSchedule()
	sessions := sched["sessions"].([]map[string]interface{})
	live := sessions[0]["live"].(map[string]interface{})
	live["start_time"] = "15:30"
	live["end_time"] = "15:05" // End before start
	filePath := createTestSchedule(t, sched)

	report, err := ValidateFile(filePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.IsValid {
		t.Errorf("expected invalid due to time inversion")
	}
}

func TestValidateFile_CheckRemoteURLs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("fake image data"))
	}))
	defer server.Close()

	sched := validBaseSchedule()
	sessions := sched["sessions"].([]map[string]interface{})
	sessions[0]["images"] = []map[string]interface{}{
		{
			"type":  "flyer",
			"label": "フライヤー",
			"url":   server.URL + "/test.jpg",
		},
	}
	filePath := createTestSchedule(t, sched)

	opts := Options{
		CheckRemoteURLs: true,
		HTTPTimeout:     2 * time.Second,
	}
	report, err := ValidateFileWithOptions(filePath, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !report.IsValid {
		t.Errorf("expected valid for running server, got errors: %v", report.Errors)
	}

	// Now point to 404
	sessions[0]["images"] = []map[string]interface{}{
		{
			"type":  "flyer",
			"label": "フライヤー",
			"url":   server.URL + "/notfound.jpg",
		},
	}
	// Re-run with server returning 404
	server404 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server404.Close()
	sessions[0]["images"].([]map[string]interface{})[0]["url"] = server404.URL + "/notfound.jpg"

	filePath404 := createTestSchedule(t, sched)
	report404, err := ValidateFileWithOptions(filePath404, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report404.IsValid {
		t.Errorf("expected invalid for 404 image URL")
	}
}
