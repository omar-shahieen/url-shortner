package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/omar-shahieen/url-shortner/internal/model"
	"github.com/omar-shahieen/url-shortner/internal/repository/inmemory"
)

func TestServiceShortenAndResolve(t *testing.T) {
	ctx := context.Background()
	repository := inmemory.New()
	service := New(repository)

	shortened, err := service.Shorten(ctx, "https://example.com/articles/go", "", nil)
	if err != nil {
		t.Fatalf("Shorten() error = %v", err)
	}
	if shortened.IsCustomAlias {
		t.Error("Shorten() generated URL is marked as a custom alias")
	}
	if len(shortened.Code) != codeLength {
		t.Errorf("Shorten() code length = %d, want %d", len(shortened.Code), codeLength)
	}

	resolved, err := service.Resolve(ctx, shortened.Code)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if resolved.OriginalURL != shortened.OriginalURL {
		t.Errorf("Resolve() original URL = %q, want %q", resolved.OriginalURL, shortened.OriginalURL)
	}
	if resolved.ClickCount != 1 {
		t.Errorf("Resolve() click count = %d, want 1", resolved.ClickCount)
	}
}

func TestServiceRetriesGeneratedCodeCollision(t *testing.T) {
	ctx := context.Background()
	repository := inmemory.New()
	generator := Generator{}
	firstCode := generator.Generate("https://example.com/collision", 0)
	if err := repository.Save(ctx, &model.URL{Code: firstCode, OriginalURL: "https://example.com/existing"}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	shortened, err := New(repository).Shorten(ctx, "https://example.com/collision", "", nil)
	if err != nil {
		t.Fatalf("Shorten() error = %v", err)
	}
	if shortened.Code == firstCode {
		t.Errorf("Shorten() code = %q, want a collision retry", shortened.Code)
	}
}

func TestServiceCustomAliasAndExpiry(t *testing.T) {
	ctx := context.Background()
	service := New(inmemory.New())
	past := time.Now().Add(-time.Second)

	shortened, err := service.Shorten(ctx, "https://example.com/expired", "expired-link", &past)
	if err != nil {
		t.Fatalf("Shorten() error = %v", err)
	}
	if !shortened.IsCustomAlias {
		t.Error("Shorten() custom alias is not marked as custom")
	}

	if _, err := service.Resolve(ctx, shortened.Code); !errors.Is(err, model.ErrExpired) {
		t.Errorf("Resolve() error = %v, want errors.Is(_, ErrExpired)", err)
	}

	if _, err := service.Shorten(ctx, "https://example.com/another", shortened.Code, nil); !errors.Is(err, model.ErrAliasTaken) {
		t.Errorf("Shorten() duplicate alias error = %v, want errors.Is(_, ErrAliasTaken)", err)
	}
}
