package db

import (
	"database/sql"
	_ "embed"
	"fmt"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed schema.sql
var SchemaSQL string

// Open opens a SQLite database at the given path, configures connection settings,
// and applies the schema migration.
func Open(path string) (*sql.DB, error) {
	var dsn string
	if path == ":memory:" {
		dsn = "file::memory:?mode=memory&cache=shared&_foreign_keys=on"
	} else if strings.Contains(path, "?") {
		dsn = path + "&_foreign_keys=on&_busy_timeout=5000"
	} else {
		dsn = path + "?_foreign_keys=on&_busy_timeout=5000"
	}

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}

	if path == ":memory:" {
		db.SetMaxOpenConns(1)
	} else {
		// Enable WAL mode for better concurrency and durability on file databases
		if _, err := db.Exec(`PRAGMA journal_mode = WAL;`); err != nil {
			db.Close()
			return nil, fmt.Errorf("enable WAL mode: %w", err)
		}
	}

	// Ensure foreign keys are enabled
	if _, err := db.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}

	if err := Migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate db: %w", err)
	}

	return db, nil
}

// Init initializes the database at the given path and returns the active connection.
// It is an alias for Open, commonly used in tests and bootstrap scripts.
func Init(path string) (*sql.DB, error) {
	return Open(path)
}

// Migrate executes the embedded schema.sql against the given database.
func Migrate(db *sql.DB) error {
	if _, err := db.Exec(SchemaSQL); err != nil {
		return fmt.Errorf("executing schema: %w", err)
	}
	return nil
}
