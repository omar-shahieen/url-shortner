package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/omar-shahieen/url-shortner/internal/model"
)

func TestRepositoryCRUD(t *testing.T) {
	ctx := context.Background()
	repository, database := newTestRepository(t)
	defer database.Close()

	expiresAt := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	createdAt := time.Date(2026, time.January, 1, 3, 4, 5, 0, time.UTC)
	url := &model.URL{
		Code:          "go-guide",
		OriginalURL:   "https://example.com/guides/go",
		CreatedAt:     createdAt,
		ExpiresAt:     &expiresAt,
		ClickCount:    3,
		IsCustomAlias: true,
	}

	if err := repository.Save(ctx, url); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	exists, err := repository.Exists(ctx, url.Code)
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if !exists {
		t.Fatal("Exists() = false, want true")
	}

	found, err := repository.FindByCode(ctx, url.Code)
	if err != nil {
		t.Fatalf("FindByCode() error = %v", err)
	}
	if found.Code != url.Code || found.OriginalURL != url.OriginalURL ||
		!found.CreatedAt.Equal(url.CreatedAt) || found.ExpiresAt == nil ||
		!found.ExpiresAt.Equal(*url.ExpiresAt) || found.ClickCount != url.ClickCount ||
		found.IsCustomAlias != url.IsCustomAlias {
		t.Errorf("FindByCode() = %#v, want %#v", found, url)
	}

	if err := repository.IncrementClicks(ctx, url.Code); err != nil {
		t.Fatalf("IncrementClicks() error = %v", err)
	}
	found, err = repository.FindByCode(ctx, url.Code)
	if err != nil {
		t.Fatalf("FindByCode() after increment error = %v", err)
	}
	if found.ClickCount != 4 {
		t.Errorf("click count = %d, want 4", found.ClickCount)
	}
}

func TestRepositoryErrors(t *testing.T) {
	ctx := context.Background()
	repository, database := newTestRepository(t)
	defer database.Close()

	if _, err := repository.FindByCode(ctx, "missing"); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("FindByCode() error = %v, want errors.Is(_, ErrNotFound)", err)
	}
	if err := repository.IncrementClicks(ctx, "missing"); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("IncrementClicks() error = %v, want errors.Is(_, ErrNotFound)", err)
	}

	url := &model.URL{Code: "duplicate", OriginalURL: "https://example.com", CreatedAt: time.Now()}
	if err := repository.Save(ctx, url); err != nil {
		t.Fatalf("first Save() error = %v", err)
	}
	if err := repository.Save(ctx, url); !errors.Is(err, model.ErrAliasTaken) {
		t.Errorf("second Save() error = %v, want errors.Is(_, ErrAliasTaken)", err)
	}
}

func newTestRepository(t *testing.T) (*Repository, *sql.DB) {
	t.Helper()

	database, err := sql.Open("sqlite", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	database.SetMaxOpenConns(1)

	repository := New(database)
	if err := repository.Initialize(context.Background()); err != nil {
		database.Close()
		t.Fatalf("Initialize() error = %v", err)
	}

	return repository, database
}
