// Package categories handles listing and validating forum categories
// (subforums). Categories are seeded via the schema; this package does
// not currently expose user-facing category creation.
package categories

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"forum/internal"
	"forum/internal/httputil"
)

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// List handles GET /api/categories. Public — no auth required.
func (s *Service) List(w http.ResponseWriter, r *http.Request) {
	cats, err := s.listAll()
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, "failed to load categories")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cats)
}

// ListData returns all categories for use by server-rendered pages and
// other services.
func (s *Service) ListData() ([]internal.Category, error) {
	return s.listAll()
}

// listAll is the internal helper other services (e.g. posts) can call
// to resolve category rows without going through HTTP.
func (s *Service) listAll() ([]internal.Category, error) {
	rows, err := s.db.Query(`SELECT id, name FROM categories ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cats []internal.Category
	for rows.Next() {
		var c internal.Category
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			return nil, err
		}
		cats = append(cats, c)
	}
	return cats, rows.Err()
}

// ValidateIDs checks that every id in categoryIDs exists in the
// categories table. Used by posts.Create to reject bogus category ids
// before inserting a post. Returns the matching rows in no particular
// order, plus an error if any requested id was not found.
func (s *Service) ValidateIDs(categoryIDs []int) ([]internal.Category, error) {
	if len(categoryIDs) == 0 {
		return nil, nil
	}

	placeholders := make([]any, len(categoryIDs))
	query := `SELECT id, name FROM categories WHERE id IN (`
	for i, id := range categoryIDs {
		if i > 0 {
			query += ","
		}
		query += "?"
		placeholders[i] = id
	}
	query += ")"

	rows, err := s.db.Query(query, placeholders...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var found []internal.Category
	for rows.Next() {
		var c internal.Category
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			return nil, err
		}
		found = append(found, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(found) != len(categoryIDs) {
		return found, sql.ErrNoRows // signals a bad/nonexistent id to the caller
	}
	return found, nil
}
