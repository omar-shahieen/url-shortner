// Package repository defines persistence contracts and implementations.
package repository

import (
	"context"

	"github.com/omar-shahieen/url-shortner/internal/model"
)

// Repository persists shortened URLs.
type Repository interface {
	Save(ctx context.Context, url *model.URL) error
	FindByCode(ctx context.Context, code string) (*model.URL, error)
	IncrementClicks(ctx context.Context, code string) error
	Exists(ctx context.Context, code string) (bool, error)
}
