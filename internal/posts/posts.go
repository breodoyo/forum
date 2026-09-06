package posts

import (
	"database/sql"
	"net/http"
	"strings"
	"time"

	"github.com/gofrs/uuid"

	"forum/internal/auth"
	"forum/internal/httputil"
)

type Post struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	Username   string    `json:"username"`
	Title      string    `json:"title"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"created_at"`
	Categories []string  `json:"categories"`
	Likes      int       `json:"likes"`
	Dislikes   int       `json:"dislikes"`
}

type Service struct {
	DB *sql.DB
}

func New(db *sql.DB) *Service {
	return &Service{DB: db}
}

//Create 

type CreateRequest struct {
	Title      string   `json:"title"`
	Content    string   `json:"content"`
	Categories []string `json:"categories"`
}

// sanitizeCategories trims whitespace, drops empties, and de-duplicates category names while preserving order.
func sanitizeCategories(cats []string) []string {
	seen := make(map[string]bool, len(cats))
	var out []string
	for _, c := range cats {
		c = strings.TrimSpace(c)
		if c == "" || seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, c)
	}
	return out
}

func (s *Service) Create(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req CreateRequest
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Title = strings.TrimSpace(req.Title)
	req.Content = strings.TrimSpace(req.Content)
	req.Categories = sanitizeCategories(req.Categories)

	if req.Title == "" || req.Content == "" {
		httputil.WriteError(w, http.StatusBadRequest, "title and content are required")
		return
	}
	if len(req.Categories) == 0 {
		httputil.WriteError(w, http.StatusBadRequest, "at least one category is required")
		return
	}

	id, err := uuid.NewV4()
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	tx, err := s.DB.Begin()
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		`INSERT INTO posts (id, user_id, title, content) VALUES (?, ?, ?, ?)`,
		id.String(), user.ID, req.Title, req.Content,
	); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	for _, name := range req.Categories {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO categories (name) VALUES (?)`, name); err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		var catID int
		if err := tx.QueryRow(`SELECT id FROM categories WHERE name = ?`, name).Scan(&catID); err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		if _, err := tx.Exec(
			`INSERT OR IGNORE INTO post_categories (post_id, category_id) VALUES (?, ?)`,
			id.String(), catID,
		); err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "internal server error")
			return
		}
	}

	if err := tx.Commit(); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, map[string]string{"id": id.String()})
}

// List returns posts, optionally filtered by: category, filter-created, filter-liked

func (s *Service) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	category := strings.TrimSpace(q.Get("category"))
	filter := strings.TrimSpace(q.Get("filter")) // "created" | "liked"

	var currentUserID string
	if user, ok := auth.UserFromContext(r.Context()); ok {
		currentUserID = user.ID
	}
	if (filter == "created" || filter == "liked") && currentUserID == "" {
		httputil.WriteError(w, http.StatusUnauthorized, "authentication required for this filter")
		return
	}

	query := `
		SELECT DISTINCT p.id, p.user_id, u.username, p.title, p.content, p.created_at
		FROM posts p
		JOIN users u ON u.id = p.user_id`
	var args []any
	var conditions []string

	if category != "" {
		query += ` JOIN post_categories pc ON pc.post_id = p.id JOIN categories c ON c.id = pc.category_id`
		conditions = append(conditions, `c.name = ?`)
		args = append(args, category)
	}
	if filter == "created" {
		conditions = append(conditions, `p.user_id = ?`)
		args = append(args, currentUserID)
	}
	if filter == "liked" {
		conditions = append(conditions, `p.id IN (
			SELECT post_id FROM reactions
			WHERE user_id = ? AND post_id IS NOT NULL AND type = 'like'
		)`)
		args = append(args, currentUserID)
	}
	if len(conditions) > 0 {
		query += ` WHERE ` + strings.Join(conditions, " AND ")
	}
	query += ` ORDER BY p.created_at DESC`

	rows, err := s.DB.Query(query, args...)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	defer rows.Close()

	result := []Post{}
	for rows.Next() {
		var p Post
		if err := rows.Scan(&p.ID, &p.UserID, &p.Username, &p.Title, &p.Content, &p.CreatedAt); err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		result = append(result, p)
	}
	if err := rows.Err(); err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	for i := range result {
		s.attachCategories(&result[i])
		s.attachReactionCounts(&result[i])
	}

	httputil.WriteJSON(w, http.StatusOK, result)
}

//Get single post

func (s *Service) Get(w http.ResponseWriter, r *http.Request, id string) {
	var p Post
	err := s.DB.QueryRow(`
		SELECT p.id, p.user_id, u.username, p.title, p.content, p.created_at
		FROM posts p JOIN users u ON u.id = p.user_id
		WHERE p.id = ?`, id,
	).Scan(&p.ID, &p.UserID, &p.Username, &p.Title, &p.Content, &p.CreatedAt)
	if err == sql.ErrNoRows {
		httputil.WriteError(w, http.StatusNotFound, "post not found")
		return
	}
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	s.attachCategories(&p)
	s.attachReactionCounts(&p)
	httputil.WriteJSON(w, http.StatusOK, p)
}

// ---------- helpers ----------

func (s *Service) attachCategories(p *Post) {
	rows, err := s.DB.Query(`
		SELECT c.name FROM categories c
		JOIN post_categories pc ON pc.category_id = c.id
		WHERE pc.post_id = ?
		ORDER BY c.name`, p.ID)
	if err != nil {
		return
	}
	defer rows.Close()
	p.Categories = []string{}
	for rows.Next() {
		var name string
		if rows.Scan(&name) == nil {
			p.Categories = append(p.Categories, name)
		}
	}
}

func (s *Service) attachReactionCounts(p *Post) {
	s.DB.QueryRow(`SELECT COUNT(*) FROM reactions WHERE post_id = ? AND type = 'like'`, p.ID).Scan(&p.Likes)
	s.DB.QueryRow(`SELECT COUNT(*) FROM reactions WHERE post_id = ? AND type = 'dislike'`, p.ID).Scan(&p.Dislikes)
}