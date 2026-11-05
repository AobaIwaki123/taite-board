package monitor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	githubapi "github.com/AobaIwaki123/taite-board/internal/github"
	"github.com/AobaIwaki123/taite-board/internal/schedule"
	"github.com/AobaIwaki123/taite-board/internal/validate"
)

// ScheduleWriteback is the GitHub-backed persistence boundary for admin
// intake. The mutation callback is rerun after a 409 conflict with fresh file
// content, so concurrent updates are not silently overwritten.
type ScheduleWriteback interface {
	UpdateFile(context.Context, string, string, string, githubapi.FileMutation) (githubapi.CommitResult, error)
}

type promoteRequest struct {
	Group          string `json:"group"`
	SourceID       string `json:"source_id"`
	SourceURL      string `json:"source_url"`
	Author         string `json:"author"`
	Title          string `json:"title"`
	Date           string `json:"date"`
	Venue          string `json:"venue"`
	LiveStart      string `json:"live_start"`
	LiveEnd        string `json:"live_end"`
	BenefitStart   string `json:"benefit_start"`
	BenefitEnd     string `json:"benefit_end"`
	Booth          string `json:"booth"`
	ImageURL       string `json:"image_url"`
	AdmissionBonus string `json:"admission_bonus"`
}

type patchRequest struct {
	Group          string `json:"group"`
	SessionID      string `json:"session_id"`
	Date           string `json:"date"`
	AdmissionBonus string `json:"admission_bonus"`
	PhotoSong      string `json:"photo_song"`
	PhotoDevice    string `json:"photo_device"`
	PhotoRule      string `json:"photo_rule"`
	SpecialNotes   string `json:"special_notes"`
	SourceURL      string `json:"source_url"`
}

// commitRequest is the shared request contract for /api/intake/commit. It
// deliberately mirrors the existing promote/patch payloads so older clients
// can keep using their dedicated endpoints while the dashboard migrates.
type commitRequest struct {
	Action         string `json:"action"`
	Group          string `json:"group"`
	SourceID       string `json:"source_id"`
	SourceURL      string `json:"source_url"`
	Author         string `json:"author"`
	Title          string `json:"title"`
	Date           string `json:"date"`
	Venue          string `json:"venue"`
	LiveStart      string `json:"live_start"`
	LiveEnd        string `json:"live_end"`
	BenefitStart   string `json:"benefit_start"`
	BenefitEnd     string `json:"benefit_end"`
	Booth          string `json:"booth"`
	ImageURL       string `json:"image_url"`
	AdmissionBonus string `json:"admission_bonus"`
	SessionID      string `json:"session_id"`
	PhotoSong      string `json:"photo_song"`
	PhotoDevice    string `json:"photo_device"`
	PhotoRule      string `json:"photo_rule"`
	SpecialNotes   string `json:"special_notes"`
}

func (r commitRequest) promote() promoteRequest {
	return promoteRequest{
		Group:          r.Group,
		SourceID:       r.SourceID,
		SourceURL:      r.SourceURL,
		Author:         r.Author,
		Title:          r.Title,
		Date:           r.Date,
		Venue:          r.Venue,
		LiveStart:      r.LiveStart,
		LiveEnd:        r.LiveEnd,
		BenefitStart:   r.BenefitStart,
		BenefitEnd:     r.BenefitEnd,
		Booth:          r.Booth,
		ImageURL:       r.ImageURL,
		AdmissionBonus: r.AdmissionBonus,
	}
}

func (r commitRequest) patch() patchRequest {
	return patchRequest{
		Group:          r.Group,
		SessionID:      r.SessionID,
		Date:           r.Date,
		AdmissionBonus: r.AdmissionBonus,
		PhotoSong:      r.PhotoSong,
		PhotoDevice:    r.PhotoDevice,
		PhotoRule:      r.PhotoRule,
		SpecialNotes:   r.SpecialNotes,
		SourceURL:      r.SourceURL,
	}
}

type commitResponse struct {
	Status      string   `json:"status"`
	SessionID   string   `json:"session_id"`
	Group       string   `json:"group"`
	Domain      string   `json:"domain"`
	Message     string   `json:"message"`
	CommitSHA   string   `json:"commit_sha,omitempty"`
	CommitURL   string   `json:"commit_url,omitempty"`
	CommitPaths []string `json:"commit_paths,omitempty"`
}

func serveCommitResponse(w http.ResponseWriter, response commitResponse, err error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err != nil {
		status := http.StatusBadGateway
		if githubapi.IsConflict(err) {
			status = http.StatusConflict
		} else if strings.Contains(err.Error(), "unsupported group") || strings.Contains(err.Error(), "required") {
			status = http.StatusBadRequest
		}
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}

func serveCommitUnavailable(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusServiceUnavailable)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": "GitHub writeback is not configured; set GITHUB_TOKEN on the monitor",
	})
}

func commitPromote(ctx context.Context, writeback ScheduleWriteback, req promoteRequest) (commitResponse, error) {
	group, err := normalizeWritebackGroup(req.Group)
	if err != nil {
		return commitResponse{}, err
	}
	paths := scheduleWritebackPaths(group)
	branch := writebackBranch()
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = fmt.Sprintf("ライブ出演 (%s)", req.Date)
	}
	venue := strings.TrimSpace(req.Venue)
	if venue == "" {
		venue = "未定"
	}
	benefitType := "並行"
	benefitStart := req.BenefitStart
	benefitEnd := req.BenefitEnd
	if benefitStart == "" {
		benefitType = "未定"
		benefitStart = "--:--"
		benefitEnd = "--:--"
	}

	var sessionID string
	var lastCommit githubapi.CommitResult
	for _, path := range paths {
		commit, err := writeback.UpdateFile(ctx, branch, path,
			fmt.Sprintf("feat(data): add session %s via admin intake", req.Date),
			func(current []byte) ([]byte, error) {
				updated, id, err := mutateSchedule(current, func(tempPath string) (string, error) {
					return schedule.AddSession(schedule.AddSessionOptions{
						Date:             req.Date,
						Title:            title,
						VenueName:        venue,
						LiveStartTime:    req.LiveStart,
						LiveEndTime:      req.LiveEnd,
						BenefitType:      benefitType,
						BenefitStartTime: benefitStart,
						BenefitEndTime:   benefitEnd,
						Booth:            req.Booth,
						TicketPlatform:   "公式アナウンス待ち",
						ImageURL:         req.ImageURL,
						SourceTweetURL:   req.SourceURL,
						AdmissionBonus:   req.AdmissionBonus,
						FilePath:         tempPath,
					})
				})
				if err != nil {
					return nil, err
				}
				if sessionID != "" && sessionID != id {
					return nil, fmt.Errorf("session id changed across schedule copies: %s != %s", sessionID, id)
				}
				sessionID = id
				return updated, nil
			})
		if err != nil {
			return commitResponse{}, fmt.Errorf("committing %s: %w", path, err)
		}
		lastCommit = commit
	}

	return commitResponse{
		Status:      "success",
		SessionID:   sessionID,
		Group:       group,
		Domain:      fmt.Sprintf("%s.iorin.net", group),
		Message:     fmt.Sprintf("Session %s committed to GitHub for %s.iorin.net; Argo CD will synchronize it", sessionID, group),
		CommitSHA:   lastCommit.SHA,
		CommitURL:   lastCommit.URL,
		CommitPaths: paths,
	}, nil
}

func commitPatch(ctx context.Context, writeback ScheduleWriteback, req patchRequest) (commitResponse, error) {
	group, err := normalizeWritebackGroup(req.Group)
	if err != nil {
		return commitResponse{}, err
	}
	paths := scheduleWritebackPaths(group)
	branch := writebackBranch()
	var sessionID string
	var lastCommit githubapi.CommitResult
	commitTarget := strings.TrimSpace(req.SessionID)
	if commitTarget == "" {
		commitTarget = strings.TrimSpace(req.Date)
	}
	if commitTarget == "" {
		commitTarget = "current session"
	}

	photoRule := req.PhotoRule
	if photoRule == "" && req.PhotoSong != "" {
		photoRule = fmt.Sprintf("撮可: %s", req.PhotoSong)
	}
	var photoAvailable *bool
	if photoRule != "" || req.PhotoSong != "" {
		available := true
		photoAvailable = &available
	}

	for _, path := range paths {
		commit, err := writeback.UpdateFile(ctx, branch, path,
			fmt.Sprintf("fix(data): update session %s via admin intake", commitTarget),
			func(current []byte) ([]byte, error) {
				updated, id, err := mutateSchedule(current, func(tempPath string) (string, error) {
					targetID := strings.TrimSpace(req.SessionID)
					if targetID == "" {
						resolvedID, resolveErr := resolveSessionID(current, req.Date)
						if resolveErr != nil {
							return "", resolveErr
						}
						targetID = resolvedID
					}
					if err := schedule.UpdateSession(schedule.UpdateSessionOptions{
						SessionID:         targetID,
						PhotoRule:         photoRule,
						PhotoSong:         req.PhotoSong,
						PhotoDevice:       req.PhotoDevice,
						PhotoAvailable:    photoAvailable,
						AdmissionBonus:    req.AdmissionBonus,
						SpecialNotes:      req.SpecialNotes,
						SourceTweetURL:    req.SourceURL,
						UpdateOfficialURL: false,
						FilePath:          tempPath,
					}); err != nil {
						return "", err
					}
					return targetID, nil
				})
				if err != nil {
					return nil, err
				}
				if sessionID != "" && sessionID != id {
					return nil, fmt.Errorf("session id changed across schedule copies: %s != %s", sessionID, id)
				}
				sessionID = id
				return updated, nil
			})
		if err != nil {
			return commitResponse{}, fmt.Errorf("committing %s: %w", path, err)
		}
		lastCommit = commit
	}

	return commitResponse{
		Status:      "success",
		SessionID:   sessionID,
		Group:       group,
		Domain:      fmt.Sprintf("%s.iorin.net", group),
		Message:     fmt.Sprintf("Session %s updated in GitHub for %s.iorin.net; Argo CD will synchronize it", sessionID, group),
		CommitSHA:   lastCommit.SHA,
		CommitURL:   lastCommit.URL,
		CommitPaths: paths,
	}, nil
}

func mutateSchedule(current []byte, mutate func(string) (string, error)) ([]byte, string, error) {
	if len(current) == 0 {
		return nil, "", errors.New("GitHub schedule file is empty")
	}
	tempDir, err := os.MkdirTemp("", "taite-writeback-")
	if err != nil {
		return nil, "", fmt.Errorf("creating temporary writeback directory: %w", err)
	}
	defer os.RemoveAll(tempDir)
	tempPath := filepath.Join(tempDir, "schedule.json")
	if err := os.WriteFile(tempPath, current, 0600); err != nil {
		return nil, "", fmt.Errorf("staging schedule: %w", err)
	}
	id, err := mutate(tempPath)
	if err != nil {
		return nil, "", err
	}
	report, err := validate.ValidateFile(tempPath)
	if err != nil {
		return nil, "", fmt.Errorf("validating staged schedule: %w", err)
	}
	if !report.IsValid {
		return nil, "", fmt.Errorf("staged schedule validation failed: %s", strings.Join(report.Errors, "; "))
	}
	updated, err := os.ReadFile(tempPath)
	if err != nil {
		return nil, "", fmt.Errorf("reading staged schedule: %w", err)
	}
	return updated, id, nil
}

func resolveSessionID(raw []byte, date string) (string, error) {
	var doc struct {
		CurrentNextID string `json:"current_next_id"`
		Sessions      []struct {
			ID   string `json:"id"`
			Date string `json:"date"`
		} `json:"sessions"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return "", fmt.Errorf("parsing schedule for target session: %w", err)
	}
	if date != "" {
		for _, session := range doc.Sessions {
			if session.Date == date {
				return session.ID, nil
			}
		}
	}
	if doc.CurrentNextID != "" {
		return doc.CurrentNextID, nil
	}
	if len(doc.Sessions) > 0 {
		return doc.Sessions[0].ID, nil
	}
	return "", errors.New("target session not found in schedule")
}

func normalizeWritebackGroup(raw string) (string, error) {
	group := strings.ToLower(strings.TrimSpace(raw))
	if group == "" || group == "all" || group == "rincent" {
		group = "ribonclock"
	}
	if group != "ribonclock" && group != "dollykiss" {
		return "", fmt.Errorf("unsupported group %q", raw)
	}
	return group, nil
}

func scheduleWritebackPaths(group string) []string {
	paths := []string{fmt.Sprintf("data/groups/%s/schedule.json", group)}
	if group == "ribonclock" {
		return append(paths,
			"data/generated/schedule.json",
			"apps/web/public/data/groups/ribonclock/schedule.json",
			"apps/web/public/schedule.json",
			"apps/web/public/mock_schedule.json",
		)
	}
	return append(paths, fmt.Sprintf("apps/web/public/data/groups/%s/schedule.json", group))
}

func writebackBranch() string {
	branch := strings.TrimSpace(os.Getenv("GITHUB_BRANCH"))
	if branch == "" {
		return "main"
	}
	return branch
}
