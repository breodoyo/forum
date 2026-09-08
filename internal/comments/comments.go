package comments

import (
	"database/sql"
	"net/http"
	"strings"
	"time"

	"github.com/gofrs/uuid"

	"forum/internal/auth"
	"forum/internal/httputil"
)

type Comment struct {
	ID        string    `json:"id"`
	PostID    string    `json:"post_id"`
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	Likes     int       `json:"likes"`
	Dislikes  int       `json:"dislikes"`
}

type Service struct {
	DB *sql.DB
}

func New(db *sql.DB) *Service {
	return &Service{DB: db}
}

type CreateRequest struct {
	Content string `json:"content"`
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

	httputil.WriteJSON(w, http.StatusCreated, map[string]string{"id": id.String()})
}

// ListByPost returns every comment on a post, visible to guests and registered users alike.
func (s *Service) ListByPost(w http.ResponseWriter, r *http.Request, postID string) {
	rows, err := s.DB.Query(`
		SELECT c.id, c.post_id, c.user_id, u.username, c.content, c.created_at
		FROM comments c JOIN users u ON u.id = c.user_id
		WHERE c.post_id = ?
		ORDER BY c.created_at ASC`, postID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	defer rows.Close()

	result := []Comment{}
	for rows.Next() {
		var c Comment
		if err := rows.Scan(&c.ID, &c.PostID, &c.UserID, &c.Username, &c.Content, &c.CreatedAt); err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		result = append(result, c)
	}
	if err := rows.Err(); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	for i := range result {
		s.DB.QueryRow(`SELECT COUNT(*) FROM reactions WHERE comment_id = ? AND type = 'like'`, result[i].ID).Scan(&result[i].Likes)
		s.DB.QueryRow(`SELECT COUNT(*) FROM reactions WHERE comment_id = ? AND type = 'dislike'`, result[i].ID).Scan(&result[i].Dislikes)
	}

	httputil.WriteJSON(w, http.StatusOK, result)
}