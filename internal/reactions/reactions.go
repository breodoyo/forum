// Package reactions implements toggleable likes/dislikes shared by posts
// and comments: submitting the same reaction twice clears it, submitting
// the opposite reaction flips it.
package reactions

import (
	"database/sql"
	"errors"
	"net/http"

	"forum/internal/auth"
	"forum/internal/httputil"
)

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

type reactRequest struct {
	Value int `json:"value"` // 1 = like, -1 = dislike
}

// ReactToPost handles POST /api/posts/{id}/react (auth required).
func (s *Service) ReactToPost(w http.ResponseWriter, r *http.Request, postID string) {
	s.react(w, r, "post_id", postID)
}

// ReactToComment handles POST /api/comments/{id}/react (auth required).
func (s *Service) ReactToComment(w http.ResponseWriter, r *http.Request, commentID string) {
	s.react(w, r, "comment_id", commentID)
}

func (s *Service) react(w http.ResponseWriter, r *http.Request, column, targetID string) {
	user, _ := auth.UserFromContext(r.Context()) // guaranteed by RequireAuth

	var req reactRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Value != 1 && req.Value != -1 {
		httputil.Error(w, http.StatusBadRequest, "value must be 1 (like) or -1 (dislike)")
		return
	}

	table := "posts"
	if column == "comment_id" {
		table = "comments"
	}
	var exists bool
	if err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM `+table+` WHERE id = ?)`, targetID).Scan(&exists); err != nil {
		httputil.ServerError(w, err)
		return
	}
	if !exists {
		httputil.Error(w, http.StatusNotFound, table+" not found")
		return
	}

	var existing int
	err := s.db.QueryRow(
		`SELECT value FROM reactions WHERE user_id = ? AND `+column+` = ?`, user.ID, targetID,
	).Scan(&existing)

	switch {
	case errors.Is(err, sql.ErrNoRows):
		_, err = s.db.Exec(
			`INSERT INTO reactions (user_id, `+column+`, value) VALUES (?, ?, ?)`,
			user.ID, targetID, req.Value,
		)
	case err != nil:
		httputil.ServerError(w, err)
		return
	case existing == req.Value:
		// Same reaction submitted again: clear it.
		_, err = s.db.Exec(`DELETE FROM reactions WHERE user_id = ? AND `+column+` = ?`, user.ID, targetID)
	default:
		// Opposite reaction: flip it.
		_, err = s.db.Exec(`UPDATE reactions SET value = ? WHERE user_id = ? AND `+column+` = ?`,
			req.Value, user.ID, targetID)
	}
	if err != nil {
		httputil.ServerError(w, err)
		return
	}

	var likes, dislikes int
	if err := s.db.QueryRow(
		`SELECT COALESCE(SUM(value = 1), 0), COALESCE(SUM(value = -1), 0) FROM reactions WHERE `+column+` = ?`,
		targetID,
	).Scan(&likes, &dislikes); err != nil {
		httputil.ServerError(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]int{"likes": likes, "dislikes": dislikes})
}
