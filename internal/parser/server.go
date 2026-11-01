package parser

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/AobaIwaki123/taite-board/internal/candidate"
)

// Server handles Cloud Run HTTP endpoints.
type Server struct {
	parser    VisionParser
	authToken string
}

// NewServer initializes a new Cloud Run server instance.
func NewServer(parser VisionParser, authToken string) *Server {
	return &Server{
		parser:    parser,
		authToken: strings.TrimSpace(authToken),
	}
}

// Handler returns the HTTP mux for the server.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})

	mux.HandleFunc("/parse", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Optional Auth Token Verification (ADR / Cloud Run auth spec)
		if s.authToken != "" {
			authHeader := r.Header.Get("Authorization")
			apiKeyHeader := r.Header.Get("X-API-KEY")

			bearerValid := strings.HasPrefix(authHeader, "Bearer ") && strings.TrimPrefix(authHeader, "Bearer ") == s.authToken
			apiKeyValid := apiKeyHeader == s.authToken

			if !bearerValid && !apiKeyValid {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}

		var req ParseRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, fmt.Sprintf("invalid json body: %v", err), http.StatusBadRequest)
			return
		}

		if req.RequestID == "" {
			req.RequestID = fmt.Sprintf("parse-%d", time.Now().UnixNano())
		}

		ctx := r.Context()
		resp, err := s.parser.Parse(ctx, &req)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnprocessableEntity)
			_ = json.NewEncoder(w).Encode(ParseResponse{
				RequestID: req.RequestID,
				Status:    candidate.StatusFailedRetryable,
				Error:     err.Error(),
			})
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	})

	return mux
}
