package monitor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// FormatDiscordNotification formats an ActivityEntry into a concise Japanese Discord message.
func FormatDiscordNotification(entry ActivityEntry) string {
	actionLabel := "新規ライブ登録"
	switch entry.Action {
	case "patch":
		actionLabel = "ライブ情報更新"
	case "intake_start":
		actionLabel = "[開始] リンク取得・解析開始"
	case "promote_start":
		actionLabel = "[開始] カレンダー登録・更新開始"
	case "intake_error":
		actionLabel = "[失敗] リンク取得・解析エラー"
	case "promote_error":
		actionLabel = "[失敗] 新規ライブ登録エラー"
	case "patch_error":
		actionLabel = "[失敗] ライブ情報更新エラー"
	}

	groupLabel := entry.Group
	switch entry.Group {
	case "ribonclock", "":
		groupLabel = "リボンクロック"
	case "dollykiss":
		groupLabel = "Dolly Kiss"
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("**[%s] %s**", groupLabel, actionLabel))
	if entry.Date != "" {
		lines = append(lines, fmt.Sprintf("- 日付: %s", entry.Date))
	}
	if entry.Title != "" {
		lines = append(lines, fmt.Sprintf("- 公演: %s", entry.Title))
	}
	if entry.Venue != "" {
		lines = append(lines, fmt.Sprintf("- 会場: %s", entry.Venue))
	}
	if entry.Summary != "" {
		lines = append(lines, fmt.Sprintf("- 内容: %s", entry.Summary))
	}
	if entry.Error != "" {
		lines = append(lines, fmt.Sprintf("- エラー: `%s`", entry.Error))
	}
	if entry.SessionID != "" {
		lines = append(lines, fmt.Sprintf("- Session ID: `%s`", entry.SessionID))
	}
	if entry.Domain != "" {
		lines = append(lines, fmt.Sprintf("- 反映先: https://%s", entry.Domain))
	}
	if entry.SourceURL != "" {
		lines = append(lines, fmt.Sprintf("- 出典: <%s>", entry.SourceURL))
	}
	return strings.Join(lines, "\n")
}

// SendDiscordWebhook posts a message to a Discord webhook URL without leaking the URL in errors.
func SendDiscordWebhook(ctx context.Context, client *http.Client, webhookURL, content string) error {
	if strings.TrimSpace(webhookURL) == "" {
		return fmt.Errorf("discord webhook URL is not configured")
	}
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}

	payload, err := json.Marshal(map[string]string{
		"content": content,
	})
	if err != nil {
		return fmt.Errorf("marshaling webhook payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("building webhook request")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("discord returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}
