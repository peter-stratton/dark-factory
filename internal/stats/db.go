package stats

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// DB wraps a SQLite database connection for recording run statistics.
type DB struct {
	db *sql.DB
}

// Open opens or creates the SQLite database at path and runs migrations.
// Use ":memory:" for an in-memory database.
func Open(path string) (*DB, error) {
	sqlDB, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}
	sqlDB.SetMaxOpenConns(1)

	if err := migrate(sqlDB); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("migrate database: %w", err)
	}

	return &DB{db: sqlDB}, nil
}

// Close closes the database connection.
func (d *DB) Close() error {
	return d.db.Close()
}

// BeginTx begins a new database transaction.
func (d *DB) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return d.db.BeginTx(ctx, nil)
}
