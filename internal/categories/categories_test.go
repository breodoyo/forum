package categories_test

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"forum/internal/categories"
	"forum/internal/db"
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

func TestList_SeedCategories(t *testing.T) {
	conn := newTestDB(t)
	svc := categories.NewService(conn)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/categories", nil)
	svc.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var cats []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &cats); err != nil {
		t.Fatalf("decode: %v", err)
	}

	expected := []string{"Discussions", "Feedback", "General", "Technology"}
	if len(cats) != len(expected) {
		t.Fatalf("expected %d categories, got %d", len(expected), len(cats))
	}
	for i, cat := range cats {
		if cat.Name != expected[i] {
			t.Errorf("cats[%d].Name = %q, want %q", i, cat.Name, expected[i])
		}
	}
}

func TestListData(t *testing.T) {
	conn := newTestDB(t)
	svc := categories.NewService(conn)

	cats, err := svc.ListData()
	if err != nil {
		t.Fatalf("ListData: %v", err)
	}
	if len(cats) != 4 {
		t.Fatalf("expected 4 categories, got %d", len(cats))
	}
}
