package auth

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"golang.org/x/crypto/bcrypt"

	"forum/internal/httputil"
)

const (
	sessionCookieName = "forum_session"
	sessionDuration   = 24 * time.Hour
)

var emailRE = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

// User is the authenticated identity attached to a request's context.
type User struct {
	ID       string
	Email    string
	Username string
}

type Service struct {
	DB *sql.DB
}

func New(db *sql.DB) *Service {
	return &Service{DB: db}
}

type ctxKey string

const userCtxKey ctxKey = "auth.user"

// Registration

type RegisterRequest struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Service) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Username = strings.TrimSpace(req.Username)

	if req.Email == "" || !emailRE.MatchString(req.Email) {
		httputil.WriteError(w, http.StatusBadRequest, "a valid email is required")
		return
	}
	if req.Username == "" {
		httputil.WriteError(w, http.StatusBadRequest, "username is required")
		return
	}
	if len(req.Password) < 8 {
		httputil.WriteError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	var exists int
	if err := s.DB.QueryRow(`SELECT COUNT(1) FROM users WHERE email = ?`, req.Email).Scan(&exists); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if exists > 0 {
		httputil.WriteError(w, http.StatusConflict, "email is already registered")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	id, err := uuid.NewV4()
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if _, err := s.DB.Exec(
		`INSERT INTO users (id, email, username, password_hash) VALUES (?, ?, ?, ?)`,
		id.String(), req.Email, req.Username, string(hash),
	); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			httputil.WriteError(w, http.StatusConflict, "email or username already taken")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, map[string]string{
		"id":       id.String(),
		"email":    req.Email,
		"username": req.Username,
	})
}

//Login / Logout

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Service) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	var id, username, hash string
	err := s.DB.QueryRow(
		`SELECT id, username, password_hash FROM users WHERE email = ?`, req.Email,
	).Scan(&id, &username, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		httputil.WriteError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)); err != nil {
		httputil.WriteError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	sessionID, err := uuid.NewV4()
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	expires := time.Now().Add(sessionDuration)

	// inserting keeps a single logged-in session per account.
	if _, err := s.DB.Exec(`DELETE FROM sessions WHERE user_id = ?`, id); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if _, err := s.DB.Exec(
		`INSERT INTO sessions (id, user_id, expires_at) VALUES (?, ?, ?)`,
		sessionID.String(), id, expires,
	); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    sessionID.String(),
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	httputil.WriteJSON(w, http.StatusOK, map[string]string{
		"id":       id,
		"email":    req.Email,
		"username": username,
	})
}

func (s *Service) Logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		s.DB.Exec(`DELETE FROM sessions WHERE id = ?`, cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
	})
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
}

//Session lookup & middleware

func (s *Service) userFromRequest(r *http.Request) (*User, error) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return nil, errors.New("no session cookie")
	}

	var userID string
	var expiresAt time.Time
	err = s.DB.QueryRow(
		`SELECT user_id, expires_at FROM sessions WHERE id = ?`, cookie.Value,
	).Scan(&userID, &expiresAt)
	if err != nil {
		return nil, errors.New("invalid session")
	}
	if time.Now().After(expiresAt) {
		s.DB.Exec(`DELETE FROM sessions WHERE id = ?`, cookie.Value)
		return nil, errors.New("session expired")
	}

	var u User
	if err := s.DB.QueryRow(
		`SELECT id, email, username FROM users WHERE id = ?`, userID,
	).Scan(&u.ID, &u.Email, &u.Username); err != nil {
		return nil, errors.New("user not found")
	}
	return &u, nil
}

// RequireAuth rejects the request with 401 if there is no valid session,
func (s *Service) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := s.userFromRequest(r)
		if err != nil {
			httputil.WriteError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		ctx := context.WithValue(r.Context(), userCtxKey, user)
		next(w, r.WithContext(ctx))
	}
}

func (s *Service) OptionalAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if user, err := s.userFromRequest(r); err == nil {
			r = r.WithContext(context.WithValue(r.Context(), userCtxKey, user))
		}
		next(w, r)
	}
}

// retrieves the authenticated user attached by RequireAuth or OptionalAuth, if any.
func UserFromContext(ctx context.Context) (*User, bool) {
	u, ok := ctx.Value(userCtxKey).(*User)
	return u, ok
}