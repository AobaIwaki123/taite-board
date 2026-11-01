package parser

import (
	"context"

	"github.com/AobaIwaki123/taite-board/internal/candidate"
)

// MediaItem represents an image or asset reference for AI parsing.
type MediaItem struct {
	MediaID     string `json:"media_id"`
	ContentType string `json:"content_type"`
	SHA256      string `json:"sha256"`
	InputURL    string `json:"input_url"`
}

// ParseRequest conforms to Section 9.1 of the Cloud Run AI Parser specification.
type ParseRequest struct {
	RequestID              string      `json:"request_id"`
	IdempotencyKey         string      `json:"idempotency_key"`
	Source                 string      `json:"source"`
	SourceID               string      `json:"source_id"`
	RawFingerprint         string      `json:"raw_fingerprint"`
	CandidateSchemaVersion int         `json:"candidate_schema_version"`
	ParserVersion          string      `json:"parser_version"`
	ModelID                string      `json:"model_id"`
	PromptVersion          string      `json:"prompt_version"`
	Text                   string      `json:"text"`
	Media                  []MediaItem `json:"media"`
}

// UsageStats tracks token and image consumption for auditability and cost management.
type UsageStats struct {
	InputImages  int `json:"input_images"`
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// ParseResponse conforms to Section 9.2 of the Cloud Run AI Parser specification.
type ParseResponse struct {
	RequestID     string                   `json:"request_id"`
	Status        candidate.CandidateStatus `json:"status"`
	ParserVersion string                   `json:"parser_version"`
	ModelID       string                   `json:"model_id"`
	PromptVersion string                   `json:"prompt_version"`
	Candidate     candidate.EventCandidate `json:"candidate"`
	Evidence      []candidate.Evidence     `json:"evidence"`
	Usage         UsageStats               `json:"usage"`
	Error         string                   `json:"error,omitempty"`
}

// VisionParser defines the interface for AI timetable extraction.
type VisionParser interface {
	Parse(ctx context.Context, req *ParseRequest) (*ParseResponse, error)
}
