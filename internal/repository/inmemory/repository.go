// Package inmemory provides a concurrency-safe in-memory URL repository.
package inmemory

import (
	"context"
	"sync"

	"github.com/omar-shahieen/url-shortner/internal/model"
)

// Repository stores URLs in process memory.
type Repository struct {
	mu   sync.RWMutex
	urls map[string]*model.URL
}

// New returns an empty in-memory repository.
func New() *Repository {
	return &Repository{urls: make(map[string]*model.URL)}
}

// Save stores url unless another URL already has its code.
func (r *Repository) Save(_ context.Context, url *model.URL) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.urls[url.Code]; exists {
		return model.ErrAliasTaken
	}

	r.urls[url.Code] = cloneURL(url)
	return nil
}

// FindByCode returns a copy of the URL associated with code.
func (r *Repository) FindByCode(_ context.Context, code string) (*model.URL, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	url, exists := r.urls[code]
	if !exists {
		return nil, model.ErrNotFound
	}

	return cloneURL(url), nil
}

// IncrementClicks atomically increments the click count for code.
func (r *Repository) IncrementClicks(_ context.Context, code string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	url, exists := r.urls[code]
	if !exists {
		return model.ErrNotFound
	}

	url.ClickCount++
	return nil
}

// Exists reports whether code has been stored.
func (r *Repository) Exists(_ context.Context, code string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.urls[code]
	return exists, nil
}

func cloneURL(url *model.URL) *model.URL {
	clone := *url
	if url.ExpiresAt != nil {
		expiresAt := *url.ExpiresAt
		clone.ExpiresAt = &expiresAt
	}

	return &clone
}
