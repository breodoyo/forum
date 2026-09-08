package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofrs/uuid"

	"forum/internal/db"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	conn, err := db.Init(":memory:")
	if err != nil {
		t.Fatalf("db.Init: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return New(conn)
}

func doJSON(t *testing.T, handler http.HandlerFunc, method, target string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, target, &buf)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder, dst any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), dst); err != nil {
		t.Fatalf("decode response body %q: %v", rec.Body.String(), err)
	}
}

//Test Register

func TestRegister_Success(t *testing.T) {
	s := newTestService(t)

	rec := doJSON(t, s.Register, http.MethodPost, "/api/register", RegisterRequest{
		Email:    "Alice@Example.com",
		Username: "alice",
		Password: "supersecret",
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var resp map[string]string
	decodeBody(t, rec, &resp)
	if resp["email"] != "alice@example.com" {
		t.Errorf("email = %q, want normalized lowercase %q", resp["email"], "alice@example.com")
	}
	if resp["username"] != "alice" {
		t.Errorf("username = %q, want %q", resp["username"], "alice")
	}
	if resp["id"] == "" {
		t.Error("expected a non-empty id")
	}
}

func TestRegister_InvalidEmail(t *testing.T) {
	s := newTestService(t)

	rec := doJSON(t, s.Register, http.MethodPost, "/api/register", RegisterRequest{
		Email:    "not-an-email",
		Username: "bob",
		Password: "supersecret",
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestRegister_MissingUsername(t *testing.T) {
	s := newTestService(t)

	rec := doJSON(t, s.Register, http.MethodPost, "/api/register", RegisterRequest{
		Email:    "bob@example.com",
		Username: "   ",
		Password: "supersecret",
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestRegister_PasswordTooShort(t *testing.T) {
	s := newTestService(t)

	rec := doJSON(t, s.Register, http.MethodPost, "/api/register", RegisterRequest{
		Email:    "bob@example.com",
		Username: "bob",
		Password: "short",
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	s := newTestService(t)

	first := RegisterRequest{Email: "dupe@example.com", Username: "first", Password: "supersecret"}
	if rec := doJSON(t, s.Register, http.MethodPost, "/api/register", first); rec.Code != http.StatusCreated {
		t.Fatalf("initial registration failed: status = %d, body = %s", rec.Code, rec.Body.String())
	}

	second := RegisterRequest{Email: "dupe@example.com", Username: "second", Password: "supersecret"}
	rec := doJSON(t, s.Register, http.MethodPost, "/api/register", second)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusConflict, rec.Body.String())
	}
}

func TestRegister_DuplicateUsername(t *testing.T) {
	s := newTestService(t)

	first := RegisterRequest{Email: "one@example.com", Username: "shared", Password: "supersecret"}
	if rec := doJSON(t, s.Register, http.MethodPost, "/api/register", first); rec.Code != http.StatusCreated {
		t.Fatalf("initial registration failed: status = %d, body = %s", rec.Code, rec.Body.String())
	}

	second := RegisterRequest{Email: "two@example.com", Username: "shared", Password: "supersecret"}
	rec := doJSON(t, s.Register, http.MethodPost, "/api/register", second)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusConflict, rec.Body.String())
	}
}

// test Login

func registerTestUser(t *testing.T, s *Service, email, username, password string) {
	t.Helper()
	rec := doJSON(t, s.Register, http.MethodPost, "/api/register", RegisterRequest{
		Email: email, Username: username, Password: password,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("setup registration failed: status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestLogin_Success(t *testing.T) {
	s := newTestService(t)
	registerTestUser(t, s, "carol@example.com", "carol", "supersecret")

	rec := doJSON(t, s.Login, http.MethodPost, "/api/login", LoginRequest{
		Email: "carol@example.com", Password: "supersecret",
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	cookies := rec.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == sessionCookieName {
			sessionCookie = c
		}
	}
	if sessionCookie == nil {
		t.Fatal("expected a session cookie to be set")
	}
	if sessionCookie.Value == "" {
		t.Error("session cookie value should not be empty")
	}
	if !sessionCookie.HttpOnly {
		t.Error("session cookie should be HttpOnly")
	}
	if sessionCookie.Expires.Before(time.Now()) {
		t.Error("session cookie should expire in the future")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	s := newTestService(t)
	registerTestUser(t, s, "dave@example.com", "dave", "supersecret")

	rec := doJSON(t, s.Login, http.MethodPost, "/api/login", LoginRequest{
		Email: "dave@example.com", Password: "wrongpassword",
	})

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}
}

func TestLogin_UnknownEmail(t *testing.T) {
	s := newTestService(t)

	rec := doJSON(t, s.Login, http.MethodPost, "/api/login", LoginRequest{
		Email: "nobody@example.com", Password: "whatever1",
	})

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}
}

func TestLogin_ReplacesPriorSession(t *testing.T) {
	s := newTestService(t)
	registerTestUser(t, s, "erin@example.com", "erin", "supersecret")

	first := doJSON(t, s.Login, http.MethodPost, "/api/login", LoginRequest{
		Email: "erin@example.com", Password: "supersecret",
	})
	second := doJSON(t, s.Login, http.MethodPost, "/api/login", LoginRequest{
		Email: "erin@example.com", Password: "supersecret",
	})

	firstSessionID := cookieValue(t, first, sessionCookieName)
	secondSessionID := cookieValue(t, second, sessionCookieName)

	if firstSessionID == secondSessionID {
		t.Fatal("expected a new session id on each login")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: firstSessionID})
	if _, err := s.userFromRequest(req); err == nil {
		t.Error("expected the first session to have been invalidated by the second login")
	}
}

func cookieValue(t *testing.T, rec *httptest.ResponseRecorder, name string) string {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c.Value
		}
	}
	t.Fatalf("cookie %q not found in response", name)
	return ""
}

//Test Logout

func TestLogout_ClearsSession(t *testing.T) {
	s := newTestService(t)
	registerTestUser(t, s, "frank@example.com", "frank", "supersecret")
	loginRec := doJSON(t, s.Login, http.MethodPost, "/api/login", LoginRequest{
		Email: "frank@example.com", Password: "supersecret",
	})
	sessionID := cookieValue(t, loginRec, sessionCookieName)

	req := httptest.NewRequest(http.MethodPost, "/api/logout", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	rec := httptest.NewRecorder()
	s.Logout(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	checkReq := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
	checkReq.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	if _, err := s.userFromRequest(checkReq); err == nil {
		t.Error("expected session to be invalid after logout")
	}
}

//Test Middleware

func TestRequireAuth_NoCookie(t *testing.T) {
	s := newTestService(t)

	called := false
	handler := s.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	req := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}
	if called {
		t.Error("wrapped handler should not run without a valid session")
	}
}

func TestRequireAuth_ValidSession(t *testing.T) {
	s := newTestService(t)
	registerTestUser(t, s, "grace@example.com", "grace", "supersecret")
	loginRec := doJSON(t, s.Login, http.MethodPost, "/api/login", LoginRequest{
		Email: "grace@example.com", Password: "supersecret",
	})
	sessionID := cookieValue(t, loginRec, sessionCookieName)

	var gotUser *User
	handler := s.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		u, ok := UserFromContext(r.Context())
		if !ok {
			t.Error("expected a user in context")
		}
		gotUser = u
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if gotUser == nil || gotUser.Username != "grace" {
		t.Errorf("gotUser = %+v, want username %q", gotUser, "grace")
	}
}

func TestRequireAuth_ExpiredSession(t *testing.T) {
	s := newTestService(t)
	registerTestUser(t, s, "heidi@example.com", "heidi", "supersecret")

	var userID string
	if err := s.DB.QueryRow(`SELECT id FROM users WHERE email = ?`, "heidi@example.com").Scan(&userID); err != nil {
		t.Fatalf("lookup user id: %v", err)
	}

	sessionID, err := uuid.NewV4()
	if err != nil {
		t.Fatalf("uuid.NewV4: %v", err)
	}
	// Insert a session that already expired.
	if _, err := s.DB.Exec(
		`INSERT INTO sessions (id, user_id, expires_at) VALUES (?, ?, ?)`,
		sessionID.String(), userID, time.Now().Add(-time.Hour),
	); err != nil {
		t.Fatalf("insert expired session: %v", err)
	}

	handler := s.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		t.Error("wrapped handler should not run with an expired session")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID.String()})
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}
}

func TestOptionalAuth_NoCookiePassesThrough(t *testing.T) {
	s := newTestService(t)

	called := false
	handler := s.OptionalAuth(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if _, ok := UserFromContext(r.Context()); ok {
			t.Error("did not expect a user in context without a session cookie")
		}
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if !called {
		t.Fatal("expected wrapped handler to run for a guest request")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestOptionalAuth_ValidSessionAttachesUser(t *testing.T) {
	s := newTestService(t)
	registerTestUser(t, s, "ivan@example.com", "ivan", "supersecret")
	loginRec := doJSON(t, s.Login, http.MethodPost, "/api/login", LoginRequest{
		Email: "ivan@example.com", Password: "supersecret",
	})
	sessionID := cookieValue(t, loginRec, sessionCookieName)

	handler := s.OptionalAuth(func(w http.ResponseWriter, r *http.Request) {
		u, ok := UserFromContext(r.Context())
		if !ok || u.Username != "ivan" {
			t.Errorf("expected user %q in context, got %+v (ok=%v)", "ivan", u, ok)
		}
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}