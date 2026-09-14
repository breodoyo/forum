// Command server is the forum's HTTP entrypoint: it wires the SQLite
// database, all feature services, page templates, and routes, then
// serves the API plus the server-rendered frontend.
package main

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"

	"forum/internal/auth"
	"forum/internal/categories"
	"forum/internal/comments"
	"forum/internal/db"
	"forum/internal/httputil"
	"forum/internal/posts"
	"forum/internal/reactions"
	"forum/internal/render"
)

func main() {
	dbPath := getenv("DB_PATH", "./data/forum.db")
	if err := os.MkdirAll(dirOf(dbPath), 0o755); err != nil {
		log.Fatalf("creating db directory: %v", err)
	}

	conn, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("opening database: %v", err)
	}
	defer conn.Close()

	if err := render.Load("web/templates"); err != nil {
		log.Fatalf("loading templates: %v", err)
	}

	authSvc := auth.New(conn)
	catSvc := categories.NewService(conn)
	postSvc := posts.New(conn)
	commentSvc := comments.New(conn)
	reactSvc := reactions.NewService(conn)

	// renderPage renders a page after attaching the template data every
	// layout partial needs (the current user and the category list for
	// the sidebar).
	renderPage := func(w http.ResponseWriter, r *http.Request, name string, data map[string]any) {
		if data == nil {
			data = map[string]any{}
		}
		if _, ok := data["User"]; !ok {
			if user, found := auth.UserFromContext(r.Context()); found {
				data["User"] = user
			}
		}
		if _, ok := data["Categories"]; !ok {
			if cats, err := catSvc.ListData(); err == nil {
				data["Categories"] = cats
			}
		}
		render.Page(w, name, data)
	}

	mux := http.NewServeMux()

	// -- pages (server-rendered HTML) --
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		var currentUserID string
		if user, ok := auth.UserFromContext(r.Context()); ok {
			currentUserID = user.ID
		}

		q := r.URL.Query()
		category := strings.TrimSpace(q.Get("category"))
		filter := strings.TrimSpace(q.Get("filter"))

		postsList, err := postSvc.ListPosts(filter, category, currentUserID)
		if errors.Is(err, posts.ErrUnauthorized) {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		if err != nil {
			http.Error(w, "failed to load posts", http.StatusInternalServerError)
			return
		}

		cats, err := catSvc.ListData()
		if err != nil {
			http.Error(w, "failed to load categories", http.StatusInternalServerError)
			return
		}

		activeFilter := filter
		if filter != "my-posts" && filter != "my-liked" {
			activeFilter = "all"
		}
		renderPage(w, r, "home.html", map[string]any{
			"Posts":          postsList,
			"Categories":     cats,
			"ActiveFilter":   activeFilter,
			"ActiveCategory": category,
		})
	})
	mux.HandleFunc("GET /login", func(w http.ResponseWriter, r *http.Request) {
		renderPage(w, r, "login.html", nil)
	})
	mux.HandleFunc("GET /register", func(w http.ResponseWriter, r *http.Request) {
		renderPage(w, r, "register.html", nil)
	})
	mux.HandleFunc("GET /post/new", func(w http.ResponseWriter, r *http.Request) {
		cats, err := catSvc.ListData()
		if err != nil {
			http.Error(w, "failed to load categories", http.StatusInternalServerError)
			return
		}
		renderPage(w, r, "new-post.html", map[string]any{"Categories": cats})
	})
	mux.HandleFunc("GET /post/{id}", func(w http.ResponseWriter, r *http.Request) {
		var currentUserID string
		if user, ok := auth.UserFromContext(r.Context()); ok {
			currentUserID = user.ID
		}

		p, err := postSvc.GetPost(r.PathValue("id"), currentUserID)
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			http.Error(w, "failed to load post", http.StatusInternalServerError)
			return
		}

		commentsList, err := commentSvc.ListByPostData(p.ID, currentUserID)
		if err != nil {
			http.Error(w, "failed to load comments", http.StatusInternalServerError)
			return
		}

		renderPage(w, r, "post.html", map[string]any{
			"Post":     p,
			"Comments": commentsList,
		})
	})

	// -- static assets --
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./web/static"))))

	// -- auth (JSON API) --
	mux.HandleFunc("POST /api/register", authSvc.Register)
	mux.HandleFunc("POST /api/login", authSvc.Login)
	mux.HandleFunc("POST /api/logout", authSvc.Logout)

	// -- categories (public) --
	mux.HandleFunc("GET /api/categories", catSvc.List)

	// -- posts --
	mux.HandleFunc("GET /api/posts", postSvc.List)
	mux.HandleFunc("GET /api/posts/{id}", func(w http.ResponseWriter, r *http.Request) {
		postSvc.Get(w, r, r.PathValue("id"))
	})
	mux.HandleFunc("POST /api/posts", authSvc.RequireAuth(postSvc.Create))

	// -- comments (nested under a post) --
	mux.HandleFunc("GET /api/posts/{id}/comments", func(w http.ResponseWriter, r *http.Request) {
		commentSvc.ListByPost(w, r, r.PathValue("id"))
	})
	mux.HandleFunc("POST /api/posts/{id}/comments", authSvc.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		commentSvc.Create(w, r, r.PathValue("id"))
	}))

	// -- reactions --
	mux.HandleFunc("POST /api/posts/{id}/react", authSvc.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		reactSvc.ReactToPost(w, r, r.PathValue("id"))
	}))
	mux.HandleFunc("POST /api/comments/{id}/react", authSvc.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		reactSvc.ReactToComment(w, r, r.PathValue("id"))
	}))

	// Catch-all for unmatched /api/* routes
	apiNotFound := func(w http.ResponseWriter, r *http.Request) {
		httputil.Error(w, http.StatusNotFound, "not found")
	}
	mux.HandleFunc("GET /api/", apiNotFound)
	mux.HandleFunc("POST /api/", apiNotFound)
	mux.HandleFunc("PUT /api/", apiNotFound)
	mux.HandleFunc("DELETE /api/", apiNotFound)

	handler := authSvc.OptionalAuth(http.HandlerFunc(mux.ServeHTTP))

	addr := ":" + getenv("PORT", "8080")
	log.Printf("forum listening on %s (db: %s)", addr, dbPath)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatal(err)
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func dirOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[:i]
		}
	}
	return "."
}
