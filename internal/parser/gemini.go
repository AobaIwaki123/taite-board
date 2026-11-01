package parser

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/AobaIwaki123/taite-board/internal/candidate"
)

const (
	DefaultParserVersion = "2026-11-01.1"
	DefaultModelID       = "gemini-2.5-flash"
	DefaultPromptVersion = "event-v1"
)

// GeminiClient implements VisionParser using Google Gemini 2.5 Flash REST API.
type GeminiClient struct {
	APIKey     string
	HTTPClient *http.Client
	Endpoint   string
}

// NewGeminiClient creates a client for the Gemini REST endpoint.
func NewGeminiClient(apiKey string, timeout time.Duration) *GeminiClient {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &GeminiClient{
		APIKey: apiKey,
		HTTPClient: &http.Client{
			Timeout: timeout,
		},
		Endpoint: "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent",
	}
}

type geminiPart struct {
	Text       string           `json:"text,omitempty"`
	InlineData *geminiBlobData `json:"inline_data,omitempty"`
}

type geminiBlobData struct {
	MimeType string `json:"mime_type"`
	Data     string `json:"data"` // base64
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiRequest struct {
	Contents          []geminiContent    `json:"contents"`
	SystemInstruction *geminiContent     `json:"system_instruction,omitempty"`
	GenerationConfig  map[string]any     `json:"generationConfig,omitempty"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
	} `json:"usageMetadata"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type extractedOutput struct {
	Date            string `json:"date"`
	VenueRaw        string `json:"venue_raw"`
	LiveStartRaw    string `json:"live_start_raw"`
	LiveEndRaw      string `json:"live_end_raw"`
	BenefitStartRaw string `json:"benefit_start_raw"`
	BenefitEndRaw   string `json:"benefit_end_raw"`
	Evidence        []struct {
		Field      string  `json:"field"`
		Source     string  `json:"source"`
		Quote      string  `json:"quote"`
		Confidence float64 `json:"confidence"`
	} `json:"evidence"`
}

// Parse sends the post text and timetable images to Gemini 2.5 Flash for structured extraction.
func (c *GeminiClient) Parse(ctx context.Context, req *ParseRequest) (*ParseResponse, error) {
	if c.APIKey == "" {
		return nil, fmt.Errorf("gemini api key is not configured")
	}

	modelID := req.ModelID
	if modelID == "" {
		modelID = DefaultModelID
	}
	parserVer := req.ParserVersion
	if parserVer == "" {
		parserVer = DefaultParserVersion
	}
	promptVer := req.PromptVersion
	if promptVer == "" {
		promptVer = DefaultPromptVersion
	}

	// 1. Prepare User Parts
	var parts []geminiPart
	promptText := fmt.Sprintf("Tweet Text:\n%s\n\nTarget idol group: リボンクロック (リボクロ / Ribbon Clock)\nPlease extract timetable times.", req.Text)
	parts = append(parts, geminiPart{Text: promptText})

	// 2. Fetch and embed images
	imagesCount := 0
	for _, m := range req.Media {
		if m.InputURL == "" {
			continue
		}
		imgBytes, contentType, err := c.fetchImage(ctx, m.InputURL)
		if err != nil {
			continue
		}
		if contentType == "" {
			contentType = "image/jpeg"
		}
		parts = append(parts, geminiPart{
			InlineData: &geminiBlobData{
				MimeType: contentType,
				Data:     base64.StdEncoding.EncodeToString(imgBytes),
			},
		})
		imagesCount++
	}

	// 3. Build Request
	geminiReq := geminiRequest{
		SystemInstruction: &geminiContent{
			Parts: []geminiPart{
				{
					Text: `You are an accurate timetable parsing agent for Japanese idol events.
Target group: "リボンクロック" (Ribbon Clock, リボクロ).
Extract the following JSON strictly:
{
  "date": "YYYY-MM-DD",
  "venue_raw": "Venue name",
  "live_start_raw": "HH:MM",
  "live_end_raw": "HH:MM",
  "benefit_start_raw": "HH:MM",
  "benefit_end_raw": "HH:MM",
  "evidence": [
    {"field": "live_start_raw", "source": "image", "quote": "18:30", "confidence": 0.95}
  ]
}
If a time or venue is not found, leave as empty string. Do not hallucinate.`,
				},
			},
		},
		Contents: []geminiContent{
			{
				Role:  "user",
				Parts: parts,
			},
		},
		GenerationConfig: map[string]any{
			"response_mime_type": "application/json",
			"temperature":        0.1,
		},
	}

	reqBody, err := json.Marshal(geminiReq)
	if err != nil {
		return nil, fmt.Errorf("encoding gemini request: %w", err)
	}

	apiURL := fmt.Sprintf("%s?key=%s", c.Endpoint, c.APIKey)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("creating http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("calling gemini api: %w", err)
	}
	defer httpResp.Body.Close()

	respBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading gemini response body: %w", err)
	}

	var geminiResp geminiResponse
	if err := json.Unmarshal(respBytes, &geminiResp); err != nil {
		return nil, fmt.Errorf("parsing gemini response: %w", err)
	}

	if geminiResp.Error != nil {
		return nil, fmt.Errorf("gemini api error (code %d): %s", geminiResp.Error.Code, geminiResp.Error.Message)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("gemini returned no candidates")
	}

	jsonText := strings.TrimSpace(geminiResp.Candidates[0].Content.Parts[0].Text)
	var out extractedOutput
	if err := json.Unmarshal([]byte(jsonText), &out); err != nil {
		return nil, fmt.Errorf("parsing extracted json from gemini: %w (content: %s)", err, jsonText)
	}

	var evidences []candidate.Evidence
	for _, e := range out.Evidence {
		evidences = append(evidences, candidate.Evidence{
			Field:      e.Field,
			Source:     e.Source,
			Quote:      e.Quote,
			Confidence: e.Confidence,
		})
	}

	status := candidate.StatusCompleted
	if out.LiveStartRaw == "" {
		status = candidate.StatusNeedsReview
	}

	return &ParseResponse{
		RequestID:     req.RequestID,
		Status:        status,
		ParserVersion: parserVer,
		ModelID:       modelID,
		PromptVersion: promptVer,
		Candidate: candidate.EventCandidate{
			Date:            out.Date,
			VenueRaw:        out.VenueRaw,
			LiveStartRaw:    out.LiveStartRaw,
			LiveEndRaw:      out.LiveEndRaw,
			BenefitStartRaw: out.BenefitStartRaw,
			BenefitEndRaw:   out.BenefitEndRaw,
		},
		Evidence: evidences,
		Usage: UsageStats{
			InputImages:  imagesCount,
			InputTokens:  geminiResp.UsageMetadata.PromptTokenCount,
			OutputTokens: geminiResp.UsageMetadata.CandidatesTokenCount,
		},
	}, nil
}

func (c *GeminiClient) fetchImage(ctx context.Context, imageURL string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("downloading image returned status: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	contentType := resp.Header.Get("Content-Type")
	return data, contentType, nil
}
