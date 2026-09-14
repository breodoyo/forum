package comments_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"forum/internal/auth"
	"forum/internal/comments"
	"forum/internal/db"
	"forum/internal/posts"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	conn, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func register(t *testing.T, authSvc *auth.Service, username string) *http.Cookie {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"email": username + "@example.com", "username": username, "password": "supersecret"})
	rec := httptest.NewRecorder()
	authSvc.Register(rec, httptest.NewRequest("POST", "/api/register", bytes.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == "forum_session" {
			return c
		}
	}
	t.Fatal("no session cookie after registration")
	return nil
}

func call(authSvc *auth.Service, cookie *http.Cookie, method, path string, body []byte, h http.HandlerFunc) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	authSvc.RequireAuth(h).ServeHTTP(rec, req)
	return rec
}

func TestCreateComment_Success(t *testing.T) {
	conn := newTestDB(t)
	authSvc := auth.New(conn)
	postSvc := posts.New(conn)
	commentSvc := comments.New(conn)

	cookie := register(t, authSvc, "alice")

	createBody, _ := json.Marshal(map[string]any{"title": "T", "body": "B", "categories": []string{"General"}})
	rec := call(authSvc, cookie, "POST", "/api/posts", createBody, postSvc.Create)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create post: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var post map[string]string
	json.Unmarshal(rec.Body.Bytes(), &post)
	postID := post["id"]

	cmtBody, _ := json.Marshal(map[string]string{"body": "nice post"})
	rec = call(authSvc, cookie, "POST", "/api/posts/"+postID+"/comments", cmtBody,
		func(w http.ResponseWriter, r *http.Request) { commentSvc.Create(w, r, postID) })
	if rec.Code != http.StatusCreated {
		t.Fatalf("create comment: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var created comments.Comment
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode comment: %v", err)
	}
	if created.ID == "" {
		t.Error("expected non-empty comment id")
	}
	if created.Username != "alice" {
		t.Errorf("username = %q, want %q", created.Username, "alice")
	}
	if created.Content != "nice post" {
		t.Errorf("content = %q, want %q", created.Content, "nice post")
	}
	if created.Likes != 0 || created.Dislikes != 0 {
		t.Errorf("expected 0 likes/dislikes, got %d/%d", created.Likes, created.Dislikes)
	}
	if created.CreatedAtHuman == "" {
		t.Error("expected non-empty created_at_human")
	}
}

func TestCreateComment_EmptyContent(t *testing.T) {
	conn := newTestDB(t)
	authSvc := auth.New(conn)
	postSvc := posts.New(conn)
	commentSvc := comments.New(conn)

	cookie := register(t, authSvc, "bob")

	createBody, _ := json.Marshal(map[string]any{"title": "T", "body": "B", "categories": []string{"General"}})
	rec := call(authSvc, cookie, "POST", "/api/posts", createBody, postSvc.Create)
	var post map[string]string
	json.Unmarshal(rec.Body.Bytes(), &post)

	cmtBody, _ := json.Marshal(map[string]string{"body": "   "})
	rec = call(authSvc, cookie, "POST", "/api/posts/"+post["id"]+"/comments", cmtBody,
		func(w http.ResponseWriter, r *http.Request) { commentSvc.Create(w, r, post["id"]) })
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty comment, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateComment_PostNotFound(t *testing.T) {
	conn := newTestDB(t)
	authSvc := auth.New(conn)
	commentSvc := comments.New(conn)

	cookie := register(t, authSvc, "carol")

	cmtBody, _ := json.Marshal(map[string]string{"body": "orphan"})
	rec := call(authSvc, cookie, "POST", "/api/posts/nonexistent/comments", cmtBody,
		func(w http.ResponseWriter, r *http.Request) { commentSvc.Create(w, r, "nonexistent") })
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestListComments_ReturnsHydrated(t *testing.T) {
	conn := newTestDB(t)
	authSvc := auth.New(conn)
	postSvc := posts.New(conn)
	commentSvc := comments.New(conn)

	cookie := register(t, authSvc, "dave")

	createBody, _ := json.Marshal(map[string]any{"title": "T", "body": "B", "categories": []string{"General"}})
	rec := call(authSvc, cookie, "POST", "/api/posts", createBody, postSvc.Create)
	var post map[string]string
	json.Unmarshal(rec.Body.Bytes(), &post)

	cmtBody, _ := json.Marshal(map[string]string{"body": "hello"})
	rec = call(authSvc, cookie, "POST", "/api/posts/"+post["id"]+"/comments", cmtBody,
		func(w http.ResponseWriter, r *http.Request) { commentSvc.Create(w, r, post["id"]) })
	if rec.Code != http.StatusCreated {
		t.Fatalf("create comment: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/posts/"+post["id"]+"/comments", nil)
	commentSvc.ListByPost(rec, req, post["id"])
	if rec.Code != http.StatusOK {
		t.Fatalf("list comments: expected 200, got %d", rec.Code)
	}

	var list []comments.Comment
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(list))
	}
	if list[0].Content != "hello" {
		t.Errorf("content = %q, want %q", list[0].Content, "hello")
	}
}
