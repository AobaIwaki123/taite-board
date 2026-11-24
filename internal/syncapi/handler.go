package syncapi

import (
	"bytes"
	"context"
	"crypto"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/AobaIwaki123/taite-board/internal/gcs"
)

const (
	defaultIdentityBaseURL    = "https://identitytoolkit.googleapis.com"
	defaultSecureTokenBaseURL = "https://securetoken.googleapis.com"
	defaultGoogleCertsURL     = "https://www.googleapis.com/robot/v1/metadata/x509/securetoken@system.gserviceaccount.com"
	internalEmailDomain       = "users.taite.local"
	maxSyncPayloadBytes       = 1024 * 1024 // 1 MiB safety bound per user sync file
	RoleUser                  = "user"
	RoleAdmin                 = "admin"
	RolesObjectPath           = "auth/roles.json"
	CookieSecretObjectPath    = "auth/cookie_secret.json"
	SessionCookieName         = "taite_session"
	sessionCookieTTL          = 30 * 24 * time.Hour // 30 days
)

var userIDRegex = regexp.MustCompile(`^[a-z0-9._-]{3,32}$`)

// Config holds dependencies and configuration for the user sync API server.
type Config struct {
	Store              gcs.ObjectStore
	StateBucket        string
	ProjectID          string
	APIKey             string
	AdminHandler       http.Handler
	AdminHTML          []byte
	DashboardHTML      []byte
	SessionSecret      []byte
	HTTPClient         *http.Client
	IdentityBaseURL    string
	SecureTokenBaseURL string
	GoogleCertsURL     string
	NowFunc            func() time.Time
}

// SyncPayload represents the per-user expense and attendance data persisted in GCS.
type SyncPayload struct {
	SchemaVersion int                        `json:"schema_version"`
	UpdatedAt     string                     `json:"updated_at"`
	Expenses      map[string]json.RawMessage `json:"expenses"`
	Attended      []string                   `json:"attended"`
}

// Handler serves /api/sync/* endpoints on a dedicated public port.
type Handler struct {
	cfg        Config
	certMu     sync.RWMutex
	certs      map[string]*rsa.PublicKey
	certsUntil time.Time

	secretMu  sync.Mutex
	secretKey []byte
}

// NewHandler creates a Handler for user authentication and GCS sync.
func NewHandler(cfg Config) *Handler {
	if cfg.ProjectID == "" {
		cfg.ProjectID = "taite-board"
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 15 * time.Second}
	}
	if cfg.IdentityBaseURL == "" {
		cfg.IdentityBaseURL = defaultIdentityBaseURL
	}
	if cfg.SecureTokenBaseURL == "" {
		cfg.SecureTokenBaseURL = defaultSecureTokenBaseURL
	}
	if cfg.GoogleCertsURL == "" {
		cfg.GoogleCertsURL = defaultGoogleCertsURL
	}
	if cfg.NowFunc == nil {
		cfg.NowFunc = time.Now
	}
	return &Handler{
		cfg:   cfg,
		certs: make(map[string]*rsa.PublicKey),
	}
}

// Mux returns an http.Handler routing /api/sync/* and RBAC-protected /admin endpoints.
func (h *Handler) Mux() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintln(w, "ok")
	})
	mux.HandleFunc("/api/sync/register", h.handleAuthAction("accounts:signUp"))
	mux.HandleFunc("/api/sync/login", h.handleAuthAction("accounts:signInWithPassword"))
	mux.HandleFunc("/api/sync/refresh", h.handleRefresh)
	mux.HandleFunc("/api/sync/logout", h.handleLogout)
	mux.HandleFunc("/api/sync/me", h.handleMe)
	mux.HandleFunc("/api/sync/data", h.handleSyncData)

	adminPage := h.cfg.AdminHTML
	if len(adminPage) == 0 {
		adminPage = h.cfg.DashboardHTML
	}
	if len(adminPage) > 0 {
		protectedAdmin := h.requireAdminPage(adminPage)
		mux.Handle("/admin", protectedAdmin)
		mux.Handle("/admin/", protectedAdmin)
	}
	if len(h.cfg.DashboardHTML) > 0 {
		protectedDash := h.requireAdminPage(h.cfg.DashboardHTML)
		mux.Handle("/admin/dashboard", protectedDash)
		mux.Handle("/admin/dashboard/", protectedDash)
	}

	if h.cfg.AdminHandler != nil {
		mux.Handle("/api/admin/", h.requireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/api/" + strings.TrimPrefix(r.URL.Path, "/api/admin/")
			h.cfg.AdminHandler.ServeHTTP(w, r2)
		})))
	}

	return mux
}

// NormalizeUserID validates a user-chosen handle and derives the synthetic Identity Platform email.
func NormalizeUserID(raw string) (string, string, error) {
	id := strings.ToLower(strings.TrimSpace(raw))
	if !userIDRegex.MatchString(id) {
		return "", "", fmt.Errorf("ユーザーIDは3〜32文字の半角英数字・記号(._-)で入力してください")
	}
	return id, id + "@" + internalEmailDomain, nil
}

type authRequest struct {
	UserID   string `json:"user_id"`
	Password string `json:"password"`
}

type authResponse struct {
	UserID       string `json:"user_id"`
	Role         string `json:"role"`
	IDToken      string `json:"id_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    string `json:"expires_in"`
}

func (h *Handler) handleAuthAction(action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if h.cfg.APIKey == "" {
			writeError(w, http.StatusServiceUnavailable, "認証サービスが構成されていません")
			return
		}

		var req authRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, 16*1024)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "リクエスト形式が不正です")
			return
		}

		userID, email, err := NormalizeUserID(req.UserID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if len(req.Password) < 6 || len(req.Password) > 128 {
			writeError(w, http.StatusBadRequest, "パスワードは6〜128文字で入力してください")
			return
		}

		payload, _ := json.Marshal(map[string]any{
			"email":             email,
			"password":          req.Password,
			"returnSecureToken": true,
		})
		endpoint := fmt.Sprintf("%s/v1/%s?key=%s",
			strings.TrimRight(h.cfg.IdentityBaseURL, "/"),
			action,
			url.QueryEscape(h.cfg.APIKey),
		)

		httpReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, endpoint, bytes.NewReader(payload))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "認証リクエスト作成に失敗しました")
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := h.cfg.HTTPClient.Do(httpReq)
		if err != nil {
			writeError(w, http.StatusBadGateway, "認証サーバーへの接続に失敗しました")
			return
		}
		defer resp.Body.Close()

		respBytes, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		if err != nil {
			writeError(w, http.StatusBadGateway, "認証レスポンスの読み取りに失敗しました")
			return
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			status, msg := mapIdentityError(respBytes, resp.StatusCode)
			writeError(w, status, msg)
			return
		}

		var idResp struct {
			LocalID      string `json:"localId"`
			IDToken      string `json:"idToken"`
			RefreshToken string `json:"refreshToken"`
			ExpiresIn    string `json:"expiresIn"`
		}
		if err := json.Unmarshal(respBytes, &idResp); err != nil || idResp.IDToken == "" {
			writeError(w, http.StatusBadGateway, "認証レスポンスの解析に失敗しました")
			return
		}

		role := h.ResolveRole(r.Context(), idResp.LocalID, userID)
		h.setSessionCookie(r.Context(), w, r, idResp.LocalID, userID)

		writeJSON(w, http.StatusOK, authResponse{
			UserID:       userID,
			Role:         role,
			IDToken:      idResp.IDToken,
			RefreshToken: idResp.RefreshToken,
			ExpiresIn:    idResp.ExpiresIn,
		})
	}
}

func (h *Handler) handleRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if h.cfg.APIKey == "" {
		writeError(w, http.StatusServiceUnavailable, "認証サービスが構成されていません")
		return
	}

	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 16*1024)).Decode(&req); err != nil || strings.TrimSpace(req.RefreshToken) == "" {
		writeError(w, http.StatusBadRequest, "refresh_token が必要です")
		return
	}

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", strings.TrimSpace(req.RefreshToken))

	endpoint := fmt.Sprintf("%s/v1/token?key=%s",
		strings.TrimRight(h.cfg.SecureTokenBaseURL, "/"),
		url.QueryEscape(h.cfg.APIKey),
	)
	httpReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "トークン更新リクエスト作成に失敗しました")
		return
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := h.cfg.HTTPClient.Do(httpReq)
	if err != nil {
		writeError(w, http.StatusBadGateway, "認証サーバーへの接続に失敗しました")
		return
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		writeError(w, http.StatusBadGateway, "トークン更新レスポンスの読み取りに失敗しました")
		return
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		writeError(w, http.StatusUnauthorized, "セッションの有効期限が切れました。再ログインしてください")
		return
	}

	var tokenResp struct {
		IDToken      string `json:"id_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    string `json:"expires_in"`
	}
	if err := json.Unmarshal(respBytes, &tokenResp); err != nil || tokenResp.IDToken == "" {
		writeError(w, http.StatusBadGateway, "トークン更新レスポンスの解析に失敗しました")
		return
	}

	if tokenResp.IDToken != "" {
		if uid, userID, err := h.VerifyIDToken(r.Context(), tokenResp.IDToken); err == nil {
			h.setSessionCookie(r.Context(), w, r, uid, userID)
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"id_token":      tokenResp.IDToken,
		"refresh_token": tokenResp.RefreshToken,
		"expires_in":    tokenResp.ExpiresIn,
	})
}

func (h *Handler) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	clearSessionCookie(w, r)
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
}

func (h *Handler) authenticateRequest(r *http.Request) (uid, userID string, ok bool) {
	if uid, userID, _, ok := h.verifySessionCookie(r); ok {
		return uid, userID, true
	}
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return "", "", false
	}
	rawToken := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	uid, userID, err := h.VerifyIDToken(r.Context(), rawToken)
	if err != nil {
		return "", "", false
	}
	return uid, userID, true
}

func (h *Handler) handleMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	uid, userID, ok := h.authenticateRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "トークンが無効または期限切れです")
		return
	}
	if _, _, remaining, hasCookie := h.verifySessionCookie(r); !hasCookie || remaining < 23*24*time.Hour {
		h.setSessionCookie(r.Context(), w, r, uid, userID)
	}
	role := h.ResolveRole(r.Context(), uid, userID)
	writeJSON(w, http.StatusOK, map[string]string{
		"user_id": userID,
		"uid":     uid,
		"role":    role,
	})
}

func (h *Handler) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, userID, ok := h.authenticateRequest(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "認証トークンまたはセッション Cookie が無効・期限切れです")
			return
		}
		if h.ResolveRole(r.Context(), uid, userID) != RoleAdmin {
			writeError(w, http.StatusForbidden, "管理者権限 (admin ロール) が必要です")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) requireAdminPage(html []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		uid, userID, ok := h.authenticateRequest(r)
		if !ok {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(adminLoginGateHTML("", "")))
			return
		}
		if h.ResolveRole(r.Context(), uid, userID) != RoleAdmin {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(adminLoginGateHTML(userID, "現在ログイン中のアカウントには admin 権限がありません（roles.json で昇格が必要です）。")))
			return
		}
		if _, _, remaining, hasCookie := h.verifySessionCookie(r); !hasCookie || remaining < 23*24*time.Hour {
			h.setSessionCookie(r.Context(), w, r, uid, userID)
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(html)
	}
}

// ResolveRole resolves the role ("user" or "admin") from gs://<state_bucket>/auth/roles.json.
func (h *Handler) ResolveRole(ctx context.Context, uid, userID string) string {
	if h.cfg.Store == nil || h.cfg.StateBucket == "" {
		return RoleUser
	}
	data, err := h.cfg.Store.GetObject(ctx, h.cfg.StateBucket, RolesObjectPath)
	if err != nil {
		return RoleUser
	}
	var doc struct {
		Roles map[string]string `json:"roles"`
	}
	if err := json.Unmarshal(data, &doc); err != nil || doc.Roles == nil {
		return RoleUser
	}
	if userID != "" && strings.EqualFold(strings.TrimSpace(doc.Roles[strings.ToLower(userID)]), RoleAdmin) {
		return RoleAdmin
	}
	if uid != "" && strings.EqualFold(strings.TrimSpace(doc.Roles[uid]), RoleAdmin) {
		return RoleAdmin
	}
	return RoleUser
}

func (h *Handler) handleSyncData(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPut {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if h.cfg.Store == nil || h.cfg.StateBucket == "" {
		writeError(w, http.StatusServiceUnavailable, "クラウド同期ストレージが構成されていません")
		return
	}

	uid, _, ok := h.authenticateRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "トークンが無効または期限切れです")
		return
	}

	objPath := userSyncObjectPath(uid)

	if r.Method == http.MethodGet {
		data, err := h.cfg.Store.GetObject(r.Context(), h.cfg.StateBucket, objPath)
		if err != nil {
			if errors.Is(err, gcs.ErrObjectNotFound) {
				writeJSON(w, http.StatusOK, SyncPayload{
					SchemaVersion: 1,
					Expenses:      map[string]json.RawMessage{},
					Attended:      []string{},
				})
				return
			}
			writeError(w, http.StatusInternalServerError, "同期データの読み込みに失敗しました")
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
		return
	}

	// PUT: validate payload and persist to GCS
	body, err := io.ReadAll(io.LimitReader(r.Body, maxSyncPayloadBytes+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "リクエストボディの読み取りに失敗しました")
		return
	}
	if len(body) > maxSyncPayloadBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "同期データサイズが上限を超えています")
		return
	}

	var payload SyncPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "同期データ JSON が不正です")
		return
	}
	if payload.SchemaVersion < 1 {
		payload.SchemaVersion = 1
	}
	if payload.Expenses == nil {
		payload.Expenses = map[string]json.RawMessage{}
	}
	if payload.Attended == nil {
		payload.Attended = []string{}
	}
	payload.UpdatedAt = h.cfg.NowFunc().UTC().Format(time.RFC3339)

	encoded, err := json.Marshal(payload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "同期データのシリアライズに失敗しました")
		return
	}

	if err := h.cfg.Store.PutObject(r.Context(), h.cfg.StateBucket, objPath, "application/json; charset=utf-8", "no-cache, max-age=0", encoded); err != nil {
		writeError(w, http.StatusInternalServerError, "クラウドへの保存に失敗しました")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(encoded)
}

func userSyncObjectPath(uid string) string {
	sum := sha256.Sum256([]byte(uid))
	return "users/" + hex.EncodeToString(sum[:]) + "/sync.json"
}

// VerifyIDToken validates a Google Identity Platform JWT (RS256) and returns the subject UID and derived userID.
func (h *Handler) VerifyIDToken(ctx context.Context, idToken string) (string, string, error) {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return "", "", fmt.Errorf("malformed jwt")
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", "", fmt.Errorf("decoding jwt header: %w", err)
	}
	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return "", "", fmt.Errorf("parsing jwt header: %w", err)
	}
	if header.Alg != "RS256" || header.Kid == "" {
		return "", "", fmt.Errorf("unsupported jwt header alg=%s kid=%s", header.Alg, header.Kid)
	}

	claimsBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", "", fmt.Errorf("decoding jwt claims: %w", err)
	}
	var claims struct {
		Aud   string `json:"aud"`
		Iss   string `json:"iss"`
		Sub   string `json:"sub"`
		Email string `json:"email"`
		Exp   int64  `json:"exp"`
		Iat   int64  `json:"iat"`
	}
	if err := json.Unmarshal(claimsBytes, &claims); err != nil {
		return "", "", fmt.Errorf("parsing jwt claims: %w", err)
	}

	nowUnix := h.cfg.NowFunc().Unix()
	expectedIss := "https://securetoken.google.com/" + h.cfg.ProjectID
	if claims.Aud != h.cfg.ProjectID {
		return "", "", fmt.Errorf("unexpected aud: %s", claims.Aud)
	}
	if claims.Iss != expectedIss {
		return "", "", fmt.Errorf("unexpected iss: %s", claims.Iss)
	}
	if claims.Sub == "" {
		return "", "", fmt.Errorf("empty sub")
	}
	if claims.Exp <= nowUnix {
		return "", "", fmt.Errorf("token expired")
	}
	if claims.Iat > nowUnix+300 {
		return "", "", fmt.Errorf("token issued in future")
	}

	pubKey, err := h.getPublicKey(ctx, header.Kid)
	if err != nil {
		return "", "", err
	}

	sigBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return "", "", fmt.Errorf("decoding jwt signature: %w", err)
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(pubKey, crypto.SHA256, digest[:], sigBytes); err != nil {
		return "", "", fmt.Errorf("invalid jwt signature: %w", err)
	}

	userID := ""
	if suffix := "@" + internalEmailDomain; strings.HasSuffix(strings.ToLower(claims.Email), suffix) {
		userID = strings.TrimSuffix(strings.ToLower(claims.Email), suffix)
	}

	return claims.Sub, userID, nil
}

func (h *Handler) getPublicKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	now := h.cfg.NowFunc()
	h.certMu.RLock()
	if now.Before(h.certsUntil) {
		if k, ok := h.certs[kid]; ok {
			h.certMu.RUnlock()
			return k, nil
		}
	}
	h.certMu.RUnlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.cfg.GoogleCertsURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := h.cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching google public certs: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("fetching google public certs returned %d", resp.StatusCode)
	}

	var rawMap map[string]string
	if err := json.Unmarshal(body, &rawMap); err != nil {
		return nil, fmt.Errorf("parsing certs json: %w", err)
	}

	parsed := make(map[string]*rsa.PublicKey, len(rawMap))
	for k, pemStr := range rawMap {
		block, _ := pem.Decode([]byte(pemStr))
		if block == nil {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			continue
		}
		if rsaPub, ok := cert.PublicKey.(*rsa.PublicKey); ok {
			parsed[k] = rsaPub
		}
	}

	ttl := parseMaxAge(resp.Header.Get("Cache-Control"))
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}

	h.certMu.Lock()
	h.certs = parsed
	h.certsUntil = now.Add(ttl)
	h.certMu.Unlock()

	pubKey, ok := parsed[kid]
	if !ok {
		return nil, fmt.Errorf("public key kid=%s not found", kid)
	}
	return pubKey, nil
}

func parseMaxAge(cacheControl string) time.Duration {
	for _, part := range strings.Split(cacheControl, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "max-age=") {
			secs, err := strconv.Atoi(strings.TrimPrefix(part, "max-age="))
			if err == nil && secs > 0 {
				return time.Duration(secs) * time.Second
			}
		}
	}
	return 0
}

func mapIdentityError(body []byte, statusCode int) (int, string) {
	var errPayload struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(body, &errPayload)
	code := strings.Split(errPayload.Error.Message, " ")[0]

	switch code {
	case "EMAIL_EXISTS":
		return http.StatusConflict, "このユーザーIDはすでに登録されています"
	case "INVALID_LOGIN_CREDENTIALS", "EMAIL_NOT_FOUND", "INVALID_PASSWORD":
		return http.StatusUnauthorized, "ユーザーIDまたはパスワードが正しくありません"
	case "WEAK_PASSWORD":
		return http.StatusBadRequest, "パスワードは6文字以上で設定してください"
	case "TOO_MANY_ATTEMPTS_TRY_LATER":
		return http.StatusTooManyRequests, "試行回数が多すぎます。しばらく待ってから再度お試しください"
	case "USER_DISABLED":
		return http.StatusForbidden, "このアカウントは無効化されています"
	default:
		if statusCode >= 400 && statusCode < 500 {
			return http.StatusBadRequest, "認証に失敗しました (" + code + ")"
		}
		return http.StatusBadGateway, "認証サーバー側でエラーが発生しました"
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

type sessionCookieClaims struct {
	UID    string `json:"uid"`
	UserID string `json:"user_id"`
	Exp    int64  `json:"exp"`
	Iat    int64  `json:"iat"`
}

func isSecureRequest(r *http.Request) bool {
	if r == nil {
		return false
	}
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https")
}

func (h *Handler) getOrInitCookieSecret(ctx context.Context) ([]byte, error) {
	h.secretMu.Lock()
	defer h.secretMu.Unlock()

	if len(h.secretKey) >= 32 {
		return h.secretKey, nil
	}
	if len(h.cfg.SessionSecret) >= 32 {
		h.secretKey = append([]byte(nil), h.cfg.SessionSecret...)
		return h.secretKey, nil
	}
	if envSecret := strings.TrimSpace(os.Getenv("SESSION_COOKIE_SECRET")); envSecret != "" {
		sum := sha256.Sum256([]byte(envSecret))
		h.secretKey = sum[:]
		return h.secretKey, nil
	}

	if h.cfg.Store != nil && h.cfg.StateBucket != "" {
		if data, err := h.cfg.Store.GetObject(ctx, h.cfg.StateBucket, CookieSecretObjectPath); err == nil {
			var doc struct {
				SecretHex string `json:"secret_hex"`
			}
			if json.Unmarshal(data, &doc) == nil && doc.SecretHex != "" {
				if decoded, hexErr := hex.DecodeString(doc.SecretHex); hexErr == nil && len(decoded) >= 32 {
					h.secretKey = decoded
					return h.secretKey, nil
				}
			}
		}
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, fmt.Errorf("generating cookie secret: %w", err)
	}
	if h.cfg.Store != nil && h.cfg.StateBucket != "" {
		payload, _ := json.Marshal(map[string]string{"secret_hex": hex.EncodeToString(raw)})
		_ = h.cfg.Store.PutObject(ctx, h.cfg.StateBucket, CookieSecretObjectPath, "application/json", "no-store", payload)
	}
	h.secretKey = raw
	return h.secretKey, nil
}

func (h *Handler) setSessionCookie(ctx context.Context, w http.ResponseWriter, r *http.Request, uid, userID string) {
	if uid == "" && userID == "" {
		return
	}
	secret, err := h.getOrInitCookieSecret(ctx)
	if err != nil {
		return
	}
	now := h.cfg.NowFunc()
	claims := sessionCookieClaims{
		UID:    uid,
		UserID: userID,
		Iat:    now.Unix(),
		Exp:    now.Add(sessionCookieTTL).Unix(),
	}
	claimsBytes, err := json.Marshal(claims)
	if err != nil {
		return
	}
	payload := base64.RawURLEncoding.EncodeToString(claimsBytes)
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    payload + "." + sig,
		Path:     "/",
		MaxAge:   int(sessionCookieTTL.Seconds()),
		HttpOnly: true,
		Secure:   isSecureRequest(r),
		SameSite: http.SameSiteLaxMode,
	})
}

func clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   isSecureRequest(r),
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *Handler) verifySessionCookie(r *http.Request) (uid, userID string, remaining time.Duration, ok bool) {
	c, err := r.Cookie(SessionCookieName)
	if err != nil || c == nil || c.Value == "" {
		return "", "", 0, false
	}
	parts := strings.SplitN(c.Value, ".", 2)
	if len(parts) != 2 {
		return "", "", 0, false
	}
	secret, err := h.getOrInitCookieSecret(r.Context())
	if err != nil {
		return "", "", 0, false
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(parts[0]))
	expectedSig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(parts[1]), []byte(expectedSig)) {
		return "", "", 0, false
	}
	claimsBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", "", 0, false
	}
	var claims sessionCookieClaims
	if err := json.Unmarshal(claimsBytes, &claims); err != nil {
		return "", "", 0, false
	}
	nowUnix := h.cfg.NowFunc().Unix()
	if claims.Exp <= nowUnix {
		return "", "", 0, false
	}
	return claims.UID, claims.UserID, time.Duration(claims.Exp-nowUnix) * time.Second, true
}

func adminLoginGateHTML(loggedInUser, errMsg string) string {
	safeUser := html.EscapeString(loggedInUser)
	safeErr := html.EscapeString(errMsg)
	noticeHTML := ""
	if safeErr != "" {
		noticeHTML = fmt.Sprintf(`<div class="err">%s</div>`, safeErr)
	}
	logoutBlock := ""
	if safeUser != "" {
		logoutBlock = fmt.Sprintf(`
      <p class="sub">現在のログイン: <strong>%s</strong></p>
      <button type="button" class="btn-secondary" onclick="doLogout()">ログアウトして別アカウントでログイン</button>
    `, safeUser)
	}
	return `<!DOCTYPE html>
<html lang="ja">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>Ribbon Clock Admin Login</title>
  <style>
    body { margin:0; min-height:100vh; display:flex; align-items:center; justify-content:center; padding:16px; background:#09090b; color:#fafaf9; font-family:-apple-system,BlinkMacSystemFont,"Hiragino Sans",sans-serif; }
    .card { width:100%; max-width:380px; background:#111114; border:1px solid rgba(255,255,255,0.1); border-radius:16px; padding:22px; box-shadow:0 20px 40px rgba(0,0,0,0.5); }
    h1 { margin:0 0 6px; font-size:18px; }
    .sub { margin:0 0 14px; font-size:12px; color:#a8a29e; line-height:1.5; }
    label { display:block; font-size:12px; color:#d6d3d1; margin:10px 0 4px; font-weight:600; }
    input { width:100%; box-sizing:border-box; padding:10px 12px; border-radius:10px; border:1px solid rgba(255,255,255,0.16); background:#09090b; color:#fff; font-size:14px; }
    button { width:100%; margin-top:16px; padding:10px 14px; border-radius:10px; border:0; background:#e11d48; color:#fff; font-size:14px; font-weight:700; cursor:pointer; }
    button:disabled { opacity:0.6; }
    .btn-secondary { background:#27272a; margin-top:8px; }
    .err { margin-top:10px; padding:10px; border-radius:10px; background:rgba(244,63,94,0.14); border:1px solid rgba(244,63,94,0.35); color:#fecdd3; font-size:12px; }
    .foot { margin-top:14px; text-align:center; font-size:12px; }
    a { color:#38bdf8; text-decoration:none; }
  </style>
</head>
<body>
  <div class="card">
    <h1>Ribbon Clock 管理画面</h1>
    <p class="sub">admin ロールを持つアカウントでログインすると、30日間有効な HttpOnly Cookie セッションで保護された管理画面へアクセスできます。</p>
    ` + noticeHTML + `
    ` + logoutBlock + `
    <form id="login-form" onsubmit="doLogin(event)">
      <label for="uid">ユーザーID</label>
      <input id="uid" type="text" required autocomplete="username" placeholder="例: ribbon_01" />
      <label for="pw">パスワード</label>
      <input id="pw" type="password" required autocomplete="current-password" />
      <button type="submit" id="submit-btn">ログインして開く</button>
    </form>
    <div id="js-err" class="err" style="display:none;"></div>
    <div class="foot"><a href="/">公開スケジュールへ戻る</a></div>
  </div>
  <script>
    (async function tryAutoUpgradeFromLocalStorage() {
      try {
        const raw = localStorage.getItem('taite_sync_session');
        if (!raw) return;
        const s = JSON.parse(raw);
        if (!s || !s.refreshToken) return;
        const res = await fetch('/api/sync/refresh', {
          method: 'POST',
          credentials: 'same-origin',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ refresh_token: s.refreshToken })
        });
        if (res.ok) {
          const data = await res.json();
          const expSec = parseInt(String(data.expires_in || '3600'), 10) || 3600;
          s.idToken = data.id_token;
          s.refreshToken = data.refresh_token || s.refreshToken;
          s.expiresAt = Date.now() + expSec * 1000;
          localStorage.setItem('taite_sync_session', JSON.stringify(s));
          const me = await fetch('/api/sync/me', { credentials: 'same-origin' });
          if (me.ok) {
            const meData = await me.json();
            if (meData.role === 'admin') {
              window.location.reload();
            }
          }
        }
      } catch {}
    })();
    async function doLogin(e) {
      e.preventDefault();
      const btn = document.getElementById('submit-btn');
      const errEl = document.getElementById('js-err');
      errEl.style.display = 'none';
      btn.disabled = true;
      try {
        const res = await fetch('/api/sync/login', {
          method: 'POST',
          credentials: 'same-origin',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            user_id: document.getElementById('uid').value.trim(),
            password: document.getElementById('pw').value
          })
        });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error || 'ログインに失敗しました');
        if (data.role !== 'admin') throw new Error('このアカウントには admin 権限がありません');
        const expSec = parseInt(String(data.expires_in || '3600'), 10) || 3600;
        localStorage.setItem('taite_sync_session', JSON.stringify({
          userId: data.user_id,
          role: data.role,
          idToken: data.id_token,
          refreshToken: data.refresh_token,
          expiresAt: Date.now() + expSec * 1000
        }));
        window.location.reload();
      } catch (err) {
        errEl.innerText = err.message;
        errEl.style.display = 'block';
        btn.disabled = false;
      }
    }
    async function doLogout() {
      await fetch('/api/sync/logout', { method: 'POST', credentials: 'same-origin' }).catch(() => {});
      localStorage.removeItem('taite_sync_session');
      window.location.reload();
    }
  </script>
</body>
</html>`
}
