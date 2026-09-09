package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitMemory(t *testing.T) {
	conn, err := Init(":memory:")
	if err != nil {
		t.Fatalf("Init(:memory:) failed: %v", err)
	}
	defer conn.Close()

	if err := conn.Ping(); err != nil {
		t.Fatalf("Ping failed: %v", err)
	}

	// Verify all tables exist
	tables := []string{
		"users",
		"sessions",
		"posts",
		"categories",
		"post_categories",
		"comments",
		"reactions",
	}

	for _, table := range tables {
		var count int
		err := conn.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&count)
		if err != nil {
			t.Fatalf("checking table %s: %v", table, err)
		}
		if count != 1 {
			t.Errorf("expected table %s to exist, count = %d", table, count)
		}
	}
}

func TestOpenFileDB(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "forum_db_test_*")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	conn, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open(%s) failed: %v", dbPath, err)
	}
	defer conn.Close()

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Errorf("expected db file %s to exist", dbPath)
	}

	// Verify WAL mode is enabled
	var journalMode string
	if err := conn.QueryRow(`PRAGMA journal_mode;`).Scan(&journalMode); err != nil {
		t.Fatalf("reading journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Errorf("expected journal_mode wal, got %s", journalMode)
	}
}

func TestSeedCategories(t *testing.T) {
	conn, err := Init(":memory:")
	if err != nil {
		t.Fatalf("Init(:memory:) failed: %v", err)
	}
	defer conn.Close()

	expectedCats := []string{"General", "Technology", "Discussions", "Feedback"}
	for _, cat := range expectedCats {
		var exists int
		err := conn.QueryRow(`SELECT count(*) FROM categories WHERE name = ?`, cat).Scan(&exists)
		if err != nil {
			t.Fatalf("query category %s: %v", cat, err)
		}
		if exists == 0 {
			t.Errorf("expected seed category %s to exist", cat)
		}
	}
}

func TestForeignKeysEnforced(t *testing.T) {
	conn, err := Init(":memory:")
	if err != nil {
		t.Fatalf("Init(:memory:) failed: %v", err)
	}
	defer conn.Close()

	// Attempt inserting a post with a non-existent user_id should fail
	_, err = conn.Exec(`INSERT INTO posts (id, user_id, title, content) VALUES ('p1', 'nonexistent_user', 'title', 'content')`)
	if err == nil {
		t.Fatal("expected foreign key constraint violation when inserting post with non-existent user_id, got nil")
	}
}

func TestCascadeDelete(t *testing.T) {
	conn, err := Init(":memory:")
	if err != nil {
		t.Fatalf("Init(:memory:) failed: %v", err)
	}
	defer conn.Close()

	// Insert a user
	_, err = conn.Exec(`INSERT INTO users (id, email, username, password_hash) VALUES ('u1', 'u1@example.com', 'user1', 'hash')`)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	// Insert post, session, comment for this user
	_, err = conn.Exec(`INSERT INTO sessions (id, user_id, expires_at) VALUES ('s1', 'u1', datetime('now', '+1 day'))`)
	if err != nil {
		t.Fatalf("insert session: %v", err)
	}

	_, err = conn.Exec(`INSERT INTO posts (id, user_id, title, content) VALUES ('p1', 'u1', 'Title', 'Content')`)
	if err != nil {
		t.Fatalf("insert post: %v", err)
	}

	_, err = conn.Exec(`INSERT INTO comments (id, post_id, user_id, content) VALUES ('c1', 'p1', 'u1', 'Nice post')`)
	if err != nil {
		t.Fatalf("insert comment: %v", err)
	}

	// Now delete the user
	_, err = conn.Exec(`DELETE FROM users WHERE id = 'u1'`)
	if err != nil {
		t.Fatalf("delete user: %v", err)
	}

	// Check that sessions, posts, comments were cascaded
	var count int
	conn.QueryRow(`SELECT count(*) FROM sessions WHERE user_id = 'u1'`).Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 sessions after user delete, got %d", count)
	}

	conn.QueryRow(`SELECT count(*) FROM posts WHERE id = 'p1'`).Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 posts after user delete, got %d", count)
	}

	conn.QueryRow(`SELECT count(*) FROM comments WHERE id = 'c1'`).Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 comments after user delete, got %d", count)
	}
}

func TestReactionConstraints(t *testing.T) {
	conn, err := Init(":memory:")
	if err != nil {
		t.Fatalf("Init(:memory:) failed: %v", err)
	}
	defer conn.Close()

	// Insert user and post
	_, err = conn.Exec(`INSERT INTO users (id, email, username, password_hash) VALUES ('u1', 'u1@example.com', 'user1', 'hash')`)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	_, err = conn.Exec(`INSERT INTO posts (id, user_id, title, content) VALUES ('p1', 'u1', 'Title', 'Content')`)
	if err != nil {
		t.Fatalf("insert post: %v", err)
	}

	// Insert first reaction
	_, err = conn.Exec(`INSERT INTO reactions (id, user_id, post_id, type) VALUES ('r1', 'u1', 'p1', 'like')`)
	if err != nil {
		t.Fatalf("insert reaction: %v", err)
	}

	// Insert duplicate reaction by same user on same post should violate unique constraint
	_, err = conn.Exec(`INSERT INTO reactions (id, user_id, post_id, type) VALUES ('r2', 'u1', 'p1', 'dislike')`)
	if err == nil {
		t.Fatal("expected unique constraint violation on duplicate reaction for same user and post")
	}
}
