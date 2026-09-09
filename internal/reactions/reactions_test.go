package reactions_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"forum/internal/auth"
	"forum/internal/db"
	"forum/internal/posts"
	"forum/internal/reactions"
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
		t.Fatalf("register: expected 201, got %d", rec.Code)
	}
	return rec.Result().Cookies()[0]
}

func call(authSvc *auth.Service, cookie *http.Cookie, method, path string, body []byte, h http.HandlerFunc) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	authSvc.RequireAuth(h).ServeHTTP(rec, req)
	return rec
}

// TestToggleLikeDislike verifies: liking sets the count, liking again clears
// it, and disliking after a like flips it rather than stacking.
func TestToggleLikeDislike(t *testing.T) {
	conn := newTestDB(t)
	authSvc := auth.New(conn)
	postSvc := posts.New(conn)
	reactSvc := reactions.NewService(conn)

	cookie := register(t, authSvc, "alice")

	createBody, _ := json.Marshal(map[string]any{"title": "T", "body": "B", "categories": []string{"General"}})
	rec := call(authSvc, cookie, "POST", "/api/posts", createBody, postSvc.Create)
	var created map[string]string
	json.Unmarshal(rec.Body.Bytes(), &created)
	postID := created["id"]

	react := func(reaction string) map[string]int {
		body, _ := json.Marshal(map[string]string{"type": reaction})
		rec := call(authSvc, cookie, "POST", "/api/posts/"+postID+"/react", body,
			func(w http.ResponseWriter, r *http.Request) { reactSvc.ReactToPost(w, r, postID) })
		if rec.Code != http.StatusOK {
			t.Fatalf("react: expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var counts map[string]int
		json.Unmarshal(rec.Body.Bytes(), &counts)
		return counts
	}

	if c := react("like"); c["likes"] != 1 || c["dislikes"] != 0 {
		t.Fatalf("after like: expected 1/0, got %+v", c)
	}
	if c := react("like"); c["likes"] != 0 || c["dislikes"] != 0 {
		t.Fatalf("after repeating like (should clear): expected 0/0, got %+v", c)
	}
	if c := react("dislike"); c["likes"] != 0 || c["dislikes"] != 1 {
		t.Fatalf("after dislike: expected 0/1, got %+v", c)
	}
	if c := react("like"); c["likes"] != 1 || c["dislikes"] != 0 {
		t.Fatalf("after flipping to like: expected 1/0, got %+v", c)
	}
}
