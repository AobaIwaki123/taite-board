package syncapi

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AobaIwaki123/taite-board/internal/gcs"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

type memStore struct {
	mu      sync.Mutex
	objects map[string][]byte
}

func newMemStore() *memStore {
	return &memStore{objects: make(map[string][]byte)}
}

func (m *memStore) GetObject(_ context.Context, bucket, object string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.objects[bucket+"/"+object]
	if !ok {
		return nil, gcs.ErrObjectNotFound
	}
	return append([]byte(nil), v...), nil
}

func (m *memStore) PutObject(_ context.Context, bucket, object, _, _ string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objects[bucket+"/"+object] = append([]byte(nil), data...)
	return nil
}

func (m *memStore) ListObjects(_ context.Context, _, _ string) ([]string, error) {
	return nil, nil
}

func generateTestCertAndKey(t *testing.T) (*rsa.PrivateKey, string) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa generate: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "securetoken@system.gserviceaccount.com"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	return priv, string(pemBytes)
}

func signTestJWT(t *testing.T, priv *rsa.PrivateKey, kid, aud, iss, sub, email string, exp int64) string {
	t.Helper()
	h, _ := json.Marshal(map[string]string{"alg": "RS256", "kid": kid, "typ": "JWT"})
	c, _ := json.Marshal(map[string]any{
		"aud":   aud,
		"iss":   iss,
		"sub":   sub,
		"email": email,
		"iat":   exp - 3600,
		"exp":   exp,
	})
	unsigned := base64.RawURLEncoding.EncodeToString(h) + "." + base64.RawURLEncoding.EncodeToString(c)
	sum := sha256.Sum256([]byte(unsigned))
	sig, err := rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatalf("sign jwt: %v", err)
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func TestNormalizeUserID(t *testing.T) {
	id, email, err := NormalizeUserID(" Aoba_01 ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "aoba_01" || email != "aoba_01@users.taite.local" {
		t.Fatalf("unexpected result: id=%s email=%s", id, email)
	}

	if _, _, err := NormalizeUserID("ab"); err == nil {
		t.Errorf("expected error for short user_id")
	}
	if _, _, err := NormalizeUserID("bad@user"); err == nil {
		t.Errorf("expected error for invalid characters")
	}
}

func TestAuthRegisterLoginAndSyncData(t *testing.T) {
	priv, certPEM := generateTestCertAndKey(t)
	now := time.Unix(1700000000, 0)
	validJWT := signTestJWT(t, priv, "kid-1", "taite-board", "https://securetoken.google.com/taite-board", "uid-abc-123", "aoba@users.taite.local", now.Add(time.Hour).Unix())
	adminJWT := signTestJWT(t, priv, "kid-1", "taite-board", "https://securetoken.google.com/taite-board", "uid-admin-999", "boss@users.taite.local", now.Add(time.Hour).Unix())
	expiredJWT := signTestJWT(t, priv, "kid-1", "taite-board", "https://securetoken.google.com/taite-board", "uid-abc-123", "aoba@users.taite.local", now.Add(-time.Minute).Unix())

	store := newMemStore()
	_ = store.PutObject(context.Background(), "taite-board-state", RolesObjectPath, "application/json", "", []byte(`{"roles":{"boss":"admin"}}`))

	adminMux := http.NewServeMux()
	adminMux.HandleFunc("/api/stats", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"admin_ok":true}`))
	})

	h := NewHandler(Config{
		Store:              store,
		StateBucket:        "taite-board-state",
		ProjectID:          "taite-board",
		APIKey:             "test-api-key",
		AdminHandler:       adminMux,
		AdminHTML:          []byte("<html>admin-register</html>"),
		DashboardHTML:      []byte("<html>admin-dashboard</html>"),
		IdentityBaseURL:    "https://identity.test",
		SecureTokenBaseURL: "https://securetoken.test",
		GoogleCertsURL:     "https://certs.test/x509",
		NowFunc:            func() time.Time { return now },
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				switch {
				case strings.HasPrefix(r.URL.String(), "https://identity.test/v1/accounts:signUp"):
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader(`{"idToken":"` + validJWT + `","refreshToken":"ref-1","expiresIn":"3600"}`)),
						Header:     make(http.Header),
					}, nil
				case strings.HasPrefix(r.URL.String(), "https://identity.test/v1/accounts:signInWithPassword"):
					bodyBytes, _ := io.ReadAll(r.Body)
					if strings.Contains(string(bodyBytes), "wrongpass") {
						return &http.Response{
							StatusCode: http.StatusBadRequest,
							Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"INVALID_LOGIN_CREDENTIALS"}}`)),
							Header:     make(http.Header),
						}, nil
					}
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader(`{"idToken":"` + validJWT + `","refreshToken":"ref-2","expiresIn":"3600"}`)),
						Header:     make(http.Header),
					}, nil
				case strings.HasPrefix(r.URL.String(), "https://certs.test/x509"):
					certsJSON, _ := json.Marshal(map[string]string{"kid-1": certPEM})
					hdr := make(http.Header)
					hdr.Set("Cache-Control", "public, max-age=3600")
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(bytes.NewReader(certsJSON)),
						Header:     hdr,
					}, nil
				default:
					t.Fatalf("unexpected HTTP call: %s", r.URL.String())
					return nil, nil
				}
			}),
		},
	})

	mux := h.Mux()

	// 1. Register
	regReq := httptest.NewRequest(http.MethodPost, "/api/sync/register", strings.NewReader(`{"user_id":"Aoba","password":"secret123"}`))
	regRec := httptest.NewRecorder()
	mux.ServeHTTP(regRec, regReq)
	if regRec.Code != http.StatusOK {
		t.Fatalf("register failed (%d): %s", regRec.Code, regRec.Body.String())
	}

	// 2. Wrong password returns 401
	badLoginReq := httptest.NewRequest(http.MethodPost, "/api/sync/login", strings.NewReader(`{"user_id":"aoba","password":"wrongpass"}`))
	badLoginRec := httptest.NewRecorder()
	mux.ServeHTTP(badLoginRec, badLoginReq)
	if badLoginRec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 on wrong password, got %d", badLoginRec.Code)
	}

	// 3. Unauthenticated GET /api/sync/data -> 401
	unauthReq := httptest.NewRequest(http.MethodGet, "/api/sync/data", nil)
	unauthRec := httptest.NewRecorder()
	mux.ServeHTTP(unauthRec, unauthReq)
	if unauthRec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without token, got %d", unauthRec.Code)
	}

	// 4. Expired JWT -> 401
	expReq := httptest.NewRequest(http.MethodGet, "/api/sync/data", nil)
	expReq.Header.Set("Authorization", "Bearer "+expiredJWT)
	expRec := httptest.NewRecorder()
	mux.ServeHTTP(expRec, expReq)
	if expRec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for expired jwt, got %d", expRec.Code)
	}

	// 5. Initial GET /api/sync/data -> empty payload
	getReq := httptest.NewRequest(http.MethodGet, "/api/sync/data", nil)
	getReq.Header.Set("Authorization", "Bearer "+validJWT)
	getRec := httptest.NewRecorder()
	mux.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("initial get failed (%d): %s", getRec.Code, getRec.Body.String())
	}

	// 6. PUT /api/sync/data -> persisted to hashed path in GCS
	putPayload := `{"schema_version":1,"expenses":{"20261101-1":{"ticketFee":2500,"drinkFee":600,"chekiSignedCount":1,"chekiRegularCount":0,"memo":"test"}},"attended":["20261101-1"]}`
	putReq := httptest.NewRequest(http.MethodPut, "/api/sync/data", strings.NewReader(putPayload))
	putReq.Header.Set("Authorization", "Bearer "+validJWT)
	putRec := httptest.NewRecorder()
	mux.ServeHTTP(putRec, putReq)
	if putRec.Code != http.StatusOK {
		t.Fatalf("put failed (%d): %s", putRec.Code, putRec.Body.String())
	}

	// 7. Subsequent GET returns saved data
	get2Req := httptest.NewRequest(http.MethodGet, "/api/sync/data", nil)
	get2Req.Header.Set("Authorization", "Bearer "+validJWT)
	get2Rec := httptest.NewRecorder()
	mux.ServeHTTP(get2Rec, get2Req)
	if get2Rec.Code != http.StatusOK || !strings.Contains(get2Rec.Body.String(), `"ticketFee":2500`) {
		t.Fatalf("subsequent get mismatch (%d): %s", get2Rec.Code, get2Rec.Body.String())
	}

	// 8. Role check via /api/sync/me (regular user vs admin)
	meUserReq := httptest.NewRequest(http.MethodGet, "/api/sync/me", nil)
	meUserReq.Header.Set("Authorization", "Bearer "+validJWT)
	meUserRec := httptest.NewRecorder()
	mux.ServeHTTP(meUserRec, meUserReq)
	if meUserRec.Code != http.StatusOK || !strings.Contains(meUserRec.Body.String(), `"role":"user"`) {
		t.Fatalf("expected role=user for aoba, got (%d): %s", meUserRec.Code, meUserRec.Body.String())
	}

	meAdminReq := httptest.NewRequest(http.MethodGet, "/api/sync/me", nil)
	meAdminReq.Header.Set("Authorization", "Bearer "+adminJWT)
	meAdminRec := httptest.NewRecorder()
	mux.ServeHTTP(meAdminRec, meAdminReq)
	if meAdminRec.Code != http.StatusOK || !strings.Contains(meAdminRec.Body.String(), `"role":"admin"`) {
		t.Fatalf("expected role=admin for boss, got (%d): %s", meAdminRec.Code, meAdminRec.Body.String())
	}

	// 9. /api/admin/stats rejects unauthenticated (401) and regular user (403), allows admin (200)
	admUnauth := httptest.NewRequest(http.MethodGet, "/api/admin/stats", nil)
	admUnauthRec := httptest.NewRecorder()
	mux.ServeHTTP(admUnauthRec, admUnauth)
	if admUnauthRec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for unauth admin api, got %d", admUnauthRec.Code)
	}

	admUser := httptest.NewRequest(http.MethodGet, "/api/admin/stats", nil)
	admUser.Header.Set("Authorization", "Bearer "+validJWT)
	admUserRec := httptest.NewRecorder()
	mux.ServeHTTP(admUserRec, admUser)
	if admUserRec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for regular user admin api, got %d", admUserRec.Code)
	}

	admOk := httptest.NewRequest(http.MethodGet, "/api/admin/stats", nil)
	admOk.Header.Set("Authorization", "Bearer "+adminJWT)
	admOkRec := httptest.NewRecorder()
	mux.ServeHTTP(admOkRec, admOk)
	if admOkRec.Code != http.StatusOK || !strings.Contains(admOkRec.Body.String(), `"admin_ok":true`) {
		t.Errorf("expected 200 for admin user, got (%d): %s", admOkRec.Code, admOkRec.Body.String())
	}

	// 10. /admin & /admin/dashboard enforce cookie/token admin RBAC (401 unauth, 403 user, 200 admin cookie)
	admPage := httptest.NewRequest(http.MethodGet, "/admin", nil)
	admPageRec := httptest.NewRecorder()
	mux.ServeHTTP(admPageRec, admPage)
	if admPageRec.Code != http.StatusUnauthorized || strings.Contains(admPageRec.Body.String(), "<html>admin-register</html>") {
		t.Errorf("expected 401 login gate for unauth /admin, got (%d): %s", admPageRec.Code, admPageRec.Body.String())
	}

	dashPage := httptest.NewRequest(http.MethodGet, "/admin/dashboard", nil)
	dashPageRec := httptest.NewRecorder()
	mux.ServeHTTP(dashPageRec, dashPage)
	if dashPageRec.Code != http.StatusUnauthorized || strings.Contains(dashPageRec.Body.String(), "<html>admin-dashboard</html>") {
		t.Errorf("expected 401 login gate for unauth /admin/dashboard, got (%d): %s", dashPageRec.Code, dashPageRec.Body.String())
	}

	// Regular user token rejected with 403 on /admin
	userPage := httptest.NewRequest(http.MethodGet, "/admin", nil)
	userPage.Header.Set("Authorization", "Bearer "+validJWT)
	userPageRec := httptest.NewRecorder()
	mux.ServeHTTP(userPageRec, userPage)
	if userPageRec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-admin /admin, got %d", userPageRec.Code)
	}

	// Admin Bearer token mints 30-day HttpOnly session cookie via /api/sync/me
	meAdminCookies := meAdminRec.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range meAdminCookies {
		if c.Name == SessionCookieName {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil || !sessionCookie.HttpOnly || sessionCookie.MaxAge < 29*24*3600 {
		t.Fatalf("expected 30-day HttpOnly %s cookie on /api/sync/me, got %+v", SessionCookieName, meAdminCookies)
	}

	// Cookie grants /admin, /admin/dashboard, and /api/admin/stats even past 1h Firebase JWT lifetime (after 24h!)
	now = now.Add(24 * time.Hour)

	admCookieReq := httptest.NewRequest(http.MethodGet, "/admin", nil)
	admCookieReq.AddCookie(sessionCookie)
	admCookieRec := httptest.NewRecorder()
	mux.ServeHTTP(admCookieRec, admCookieReq)
	if admCookieRec.Code != http.StatusOK || !strings.Contains(admCookieRec.Body.String(), "<html>admin-register</html>") {
		t.Errorf("expected /admin HTML via session cookie, got (%d): %s", admCookieRec.Code, admCookieRec.Body.String())
	}

	dashCookieReq := httptest.NewRequest(http.MethodGet, "/admin/dashboard", nil)
	dashCookieReq.AddCookie(sessionCookie)
	dashCookieRec := httptest.NewRecorder()
	mux.ServeHTTP(dashCookieRec, dashCookieReq)
	if dashCookieRec.Code != http.StatusOK || !strings.Contains(dashCookieRec.Body.String(), "<html>admin-dashboard</html>") {
		t.Errorf("expected /admin/dashboard HTML via session cookie, got (%d): %s", dashCookieRec.Code, dashCookieRec.Body.String())
	}

	apiCookieReq := httptest.NewRequest(http.MethodGet, "/api/admin/stats", nil)
	apiCookieReq.AddCookie(sessionCookie)
	apiCookieRec := httptest.NewRecorder()
	mux.ServeHTTP(apiCookieRec, apiCookieReq)
	if apiCookieRec.Code != http.StatusOK || !strings.Contains(apiCookieRec.Body.String(), `"admin_ok":true`) {
		t.Errorf("expected 200 on /api/admin/stats via cookie, got (%d): %s", apiCookieRec.Code, apiCookieRec.Body.String())
	}

	// /api/sync/logout clears session cookie
	logoutReq := httptest.NewRequest(http.MethodPost, "/api/sync/logout", nil)
	logoutRec := httptest.NewRecorder()
	mux.ServeHTTP(logoutRec, logoutReq)
	if logoutRec.Code != http.StatusOK {
		t.Errorf("expected 200 on logout, got %d", logoutRec.Code)
	}
}
