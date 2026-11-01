package candidate

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// CandidateStatus defines the processing lifecycle state (ADR-0004 & Cloud Run AI Parser Spec).
type CandidateStatus string

const (
	StatusNotRequested         CandidateStatus = "not_requested"
	StatusSkippedDeterministic CandidateStatus = "skipped_deterministic"
	StatusSkippedCache         CandidateStatus = "skipped_cache"
	StatusQueued               CandidateStatus = "queued"
	StatusRunning              CandidateStatus = "running"
	StatusCompleted            CandidateStatus = "completed"
	StatusNeedsReview          CandidateStatus = "needs_review"
	StatusFailedRetryable      CandidateStatus = "failed_retryable"
	StatusFailedPermanent      CandidateStatus = "failed_permanent"
)

// Evidence holds traceability information for an extracted field.
type Evidence struct {
	Field      string  `json:"field"`
	Source     string  `json:"source"` // "text" or "image"
	MediaID    string  `json:"media_id,omitempty"`
	Quote      string  `json:"quote,omitempty"`
	Confidence float64 `json:"confidence"`
}

// EventCandidate represents extracted candidate event parameters before curation.
type EventCandidate struct {
	Date             string `json:"date"`
	VenueRaw         string `json:"venue_raw"`
	VenueCanonicalID string `json:"venue_canonical_id,omitempty"`
	GroupRaw         string `json:"group_raw"`
	GroupCanonicalID string `json:"group_canonical_id,omitempty"`
	LiveStartRaw     string `json:"live_start_raw"`
	LiveEndRaw       string `json:"live_end_raw"`
	BenefitStartRaw  string `json:"benefit_start_raw,omitempty"`
	BenefitEndRaw    string `json:"benefit_end_raw,omitempty"`
}

// IntakeResult encapsulates the result of fetching and resolving a post.
type IntakeResult struct {
	Source         string          `json:"source"`
	SourceID       string          `json:"source_id"`
	URL            string          `json:"url"`
	Author         string          `json:"author"`
	Text           string          `json:"text"`
	Images         []string        `json:"images"`
	RawFingerprint string          `json:"raw_fingerprint"`
	NeedsAI        bool            `json:"needs_ai"`
	Status         CandidateStatus `json:"status"`
	Candidate      EventCandidate  `json:"candidate"`
	Evidence       []Evidence      `json:"evidence"`
}

// ComputeRawFingerprint generates SHA-256(source + source_id + normalized_text + ordered_images).
func ComputeRawFingerprint(source, sourceID, normalizedText string, images []string) string {
	h := sha256.New()
	h.Write([]byte(source))
	h.Write([]byte(":"))
	h.Write([]byte(sourceID))
	h.Write([]byte(":"))
	h.Write([]byte(strings.TrimSpace(normalizedText)))
	h.Write([]byte(":"))
	for _, img := range images {
		h.Write([]byte(img))
		h.Write([]byte(","))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// ComputeParseCacheKey generates SHA-256(raw_fingerprint + candidate_schema_version + parser_version + model_id + prompt_version).
func ComputeParseCacheKey(rawFingerprint string, schemaVer int, parserVer, modelID, promptVer string) string {
	h := sha256.New()
	h.Write([]byte(rawFingerprint))
	h.Write([]byte(":"))
	h.Write([]byte(string(rune(schemaVer))))
	h.Write([]byte(":"))
	h.Write([]byte(parserVer))
	h.Write([]byte(":"))
	h.Write([]byte(modelID))
	h.Write([]byte(":"))
	h.Write([]byte(promptVer))
	return hex.EncodeToString(h.Sum(nil))
}
