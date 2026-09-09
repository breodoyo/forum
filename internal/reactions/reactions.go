// Package reactions implements toggleable likes/dislikes shared by posts
// and comments: submitting the same reaction twice clears it, submitting
// the opposite reaction flips it.
package reactions

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/gofrs/uuid"

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
	Type string `json:"type"` // "like" or "dislike"
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
	if req.Type != "like" && req.Type != "dislike" {
		httputil.Error(w, http.StatusBadRequest, `type must be "like" or "dislike"`)
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

	var existing string
	err := s.db.QueryRow(
		`SELECT type FROM reactions WHERE user_id = ? AND `+column+` = ?`, user.ID, targetID,
	).Scan(&existing)

	switch {
	case errors.Is(err, sql.ErrNoRows):
		id, err := uuid.NewV4()
		if err != nil {
			httputil.ServerError(w, err)
			return
		}
		_, err = s.db.Exec(
			`INSERT INTO reactions (id, user_id, `+column+`, type) VALUES (?, ?, ?, ?)`,
			id.String(), user.ID, targetID, req.Type,
		)
		if err != nil {
			httputil.ServerError(w, err)
			return
		}
	case err != nil:
		httputil.ServerError(w, err)
		return
	case existing == req.Type:
		// Same reaction submitted again: clear it.
		if _, err := s.db.Exec(`DELETE FROM reactions WHERE user_id = ? AND `+column+` = ?`, user.ID, targetID); err != nil {
			httputil.ServerError(w, err)
			return
		}
		req.Type = "" // toggled off
	default:
		// Opposite reaction: flip it.
		if _, err := s.db.Exec(`UPDATE reactions SET type = ? WHERE user_id = ? AND `+column+` = ?`,
			req.Type, user.ID, targetID); err != nil {
			httputil.ServerError(w, err)
			return
		}
	}

	var likes, dislikes int
	if err := s.db.QueryRow(
		`SELECT COALESCE(SUM(type = 'like'), 0), COALESCE(SUM(type = 'dislike'), 0) FROM reactions WHERE `+column+` = ?`,
		targetID,
	).Scan(&likes, &dislikes); err != nil {
		httputil.ServerError(w, err)
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"likes":         likes,
		"dislikes":      dislikes,
		"user_reaction": req.Type,
	})
}
