// Package sqlite provides a SQLite-backed URL repository.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/omar-shahieen/url-shortner/internal/model"
	_ "modernc.org/sqlite"
)

// Repository persists URLs in SQLite.
type Repository struct {
	db *sql.DB
}

// New creates a repository using db. Call Initialize before using it.
func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Initialize creates the URL table when it does not already exist.
func (r *Repository) Initialize(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS urls (
			code TEXT PRIMARY KEY,
			original_url TEXT NOT NULL,
			created_at TEXT NOT NULL,
			expires_at TEXT,
			click_count INTEGER NOT NULL DEFAULT 0,
			is_custom_alias INTEGER NOT NULL DEFAULT 0
		)
	`)
	return err
}

// Save inserts url unless another URL already uses its code.
func (r *Repository) Save(ctx context.Context, url *model.URL) error {
	var expiresAt any
	if url.ExpiresAt != nil {
		expiresAt = url.ExpiresAt.UTC().Format(time.RFC3339Nano)
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO urls (code, original_url, created_at, expires_at, click_count, is_custom_alias)
		VALUES (?, ?, ?, ?, ?, ?)
	`, url.Code, url.OriginalURL, url.CreatedAt.UTC().Format(time.RFC3339Nano), expiresAt, url.ClickCount, url.IsCustomAlias)
	if err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed") {
		return model.ErrAliasTaken
	}
	return err
}

// FindByCode returns the URL associated with code.
func (r *Repository) FindByCode(ctx context.Context, code string) (*model.URL, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT code, original_url, created_at, expires_at, click_count, is_custom_alias
		FROM urls WHERE code = ?
	`, code)

	var url model.URL
	var createdAt string
	var expiresAt sql.NullString
	if err := row.Scan(&url.Code, &url.OriginalURL, &createdAt, &expiresAt, &url.ClickCount, &url.IsCustomAlias); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, model.ErrNotFound
		}
		return nil, err
	}

	parsedCreatedAt, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}
	url.CreatedAt = parsedCreatedAt

	if expiresAt.Valid {
		parsedExpiresAt, err := time.Parse(time.RFC3339Nano, expiresAt.String)
		if err != nil {
			return nil, fmt.Errorf("parse expires_at: %w", err)
		}
		url.ExpiresAt = &parsedExpiresAt
	}

	return &url, nil
}

// IncrementClicks increments the URL click count.
func (r *Repository) IncrementClicks(ctx context.Context, code string) error {
	result, err := r.db.ExecContext(ctx, `UPDATE urls SET click_count = click_count + 1 WHERE code = ?`, code)
	if err != nil {
		return err
	}

	updated, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if updated == 0 {
		return model.ErrNotFound
	}

	return nil
}

// Exists reports whether code has been stored.
func (r *Repository) Exists(ctx context.Context, code string) (bool, error) {
	var exists bool
	if err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM urls WHERE code = ?)`, code).Scan(&exists); err != nil {
		return false, err
	}

	return exists, nil
}
