package comments

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gofrs/uuid"

	"forum/internal/auth"
	"forum/internal/httputil"
)

type Comment struct {
	ID           string    `json:"id"`
	PostID       string    `json:"post_id"`
	UserID       string    `json:"user_id"`
	Username     string    `json:"user"`
	Content      string    `json:"body"`
	CreatedAt    time.Time `json:"created_at"`
	CreatedAtHuman string  `json:"created_at_human,omitempty"`
	Likes        int       `json:"likes"`
	Dislikes     int       `json:"dislikes"`
	UserReaction string    `json:"user_reaction"` // "like", "dislike", or ""
}

type Service struct {
	DB *sql.DB
}

func New(db *sql.DB) *Service {
	return &Service{DB: db}
}

type CreateRequest struct {
	Content string `json:"body"`
}

func (s *Service) Create(w http.ResponseWriter, r *http.Request, postID string) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var exists int
	if err := s.DB.QueryRow(`SELECT COUNT(1) FROM posts WHERE id = ?`, postID).Scan(&exists); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if exists == 0 {
		httputil.WriteError(w, http.StatusNotFound, "post not found")
		return
	}

	var req CreateRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Content = strings.TrimSpace(req.Content)
	if req.Content == "" {
		httputil.WriteError(w, http.StatusBadRequest, "content is required")
		return
	}

	id, err := uuid.NewV4()
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if _, err := s.DB.Exec(
		`INSERT INTO comments (id, post_id, user_id, content) VALUES (?, ?, ?, ?)`,
		id.String(), postID, user.ID, req.Content,
	); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	var created Comment
	if err := s.DB.QueryRow(`
		SELECT c.id, c.post_id, c.user_id, u.username, c.content, c.created_at
		FROM comments c JOIN users u ON u.id = c.user_id
		WHERE c.id = ?`, id.String(),
	).Scan(&created.ID, &created.PostID, &created.UserID, &created.Username, &created.Content, &created.CreatedAt); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	created.CreatedAtHuman = humanTime(created.CreatedAt)
	s.hydrate(&created, user.ID)

	httputil.WriteJSON(w, http.StatusCreated, created)
}

// ListByPostData returns every comment on a post, hydrated with reaction
// counts and the current user's own reaction.
func (s *Service) ListByPostData(postID, currentUserID string) ([]Comment, error) {
	rows, err := s.DB.Query(`
		SELECT c.id, c.post_id, c.user_id, u.username, c.content, c.created_at
		FROM comments c JOIN users u ON u.id = c.user_id
		WHERE c.post_id = ?
		ORDER BY c.created_at ASC`, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []Comment{}
	for rows.Next() {
		var c Comment
		if err := rows.Scan(&c.ID, &c.PostID, &c.UserID, &c.Username, &c.Content, &c.CreatedAt); err != nil {
			return nil, err
		}
		c.CreatedAtHuman = humanTime(c.CreatedAt)
		result = append(result, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i := range result {
		s.hydrate(&result[i], currentUserID)
	}
	return result, nil
}

// ListByPost handles GET /api/posts/{id}/comments. Public — guests see counts but no user_reaction.
func (s *Service) ListByPost(w http.ResponseWriter, r *http.Request, postID string) {
	var currentUserID string
	if user, ok := auth.UserFromContext(r.Context()); ok {
		currentUserID = user.ID
	}

	result, err := s.ListByPostData(postID, currentUserID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, result)
}

// hydrate fills in computed fields for one comment.
func (s *Service) hydrate(c *Comment, currentUserID string) {
	s.DB.QueryRow(`SELECT COUNT(*) FROM reactions WHERE comment_id = ? AND type = 'like'`, c.ID).Scan(&c.Likes)
	s.DB.QueryRow(`SELECT COUNT(*) FROM reactions WHERE comment_id = ? AND type = 'dislike'`, c.ID).Scan(&c.Dislikes)
	if currentUserID == "" {
		return
	}
	var reaction sql.NullString
	if err := s.DB.QueryRow(
			`SELECT type FROM reactions WHERE user_id = ? AND comment_id = ?`, currentUserID, c.ID,
		).Scan(&reaction); err == nil {
			c.UserReaction = reaction.String
		}
}

// humanTime renders a timestamp as a coarse relative time (same format as posts).
func humanTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return plural(int(d.Minutes()), "minute") + " ago"
	case d < 24*time.Hour:
		return plural(int(d.Hours()), "hour") + " ago"
	case d < 7*24*time.Hour:
		return plural(int(d.Hours()/24), "day") + " ago"
	case d < 30*24*time.Hour:
		return plural(int(d.Hours()/24/7), "week") + " ago"
	default:
		return t.Format("Jan 2, 2006")
	}
}

func plural(n int, unit string) string {
	if n == 1 {
		return "1 " + unit
	}
	return fmt.Sprintf("%d %ss", n, unit)
}
