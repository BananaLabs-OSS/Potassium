package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	_ "modernc.org/sqlite"
)

// Index represents a database index to create during migration.
type Index struct {
	Name  string
	Query string
}

// Connect opens a SQLite database connection with WAL mode and foreign keys enabled.
func Connect(databaseURL string) (*bun.DB, error) {
	path := strings.TrimPrefix(databaseURL, "sqlite://")

	sqldb, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite: %w", err)
	}

	if _, err := sqldb.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("failed to set WAL mode: %w", err)
	}

	if _, err := sqldb.Exec("PRAGMA foreign_keys=ON"); err != nil {
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	db := bun.NewDB(sqldb, sqlitedialect.New())

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Printf("Connected to SQLite: %s", path)
	return db, nil
}

// Migrate creates tables and indexes if they don't already exist.
func Migrate(ctx context.Context, db *bun.DB, tables []interface{}, indexes []Index) error {
	log.Printf("Running database migrations...")

	for _, model := range tables {
		_, err := db.NewCreateTable().
			Model(model).
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to create table for %T: %w", model, err)
		}
	}

	for _, idx := range indexes {
		if _, err := db.ExecContext(ctx, idx.Query); err != nil {
			return fmt.Errorf("failed to create index %s: %w", idx.Name, err)
		}
	}

	log.Printf("Migrations complete")
	return nil
}
