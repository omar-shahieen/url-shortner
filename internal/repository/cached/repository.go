// Package cached provides a caching decorator for repository.Repository.
//
// It composes:
//   - A hand-rolled LRU cache (internal/repository/cached/lru) that holds
//     *model.URL values keyed by short code.
//   - A bloom filter (internal/repository/cached/bloom) that lets us answer
//     "code definitely does not exist" in O(1) without a DB round-trip.
//
// Startup rebuild: all existing codes are loaded from the underlying
// repository into the bloom filter so it is accurate from the first request.
// The LRU cache stays cold and warms up naturally on Finds.
package cached

import (
	"context"
	"fmt"

	"github.com/omar-shahieen/url-shortner/internal/model"
	"github.com/omar-shahieen/url-shortner/internal/repository"
	"github.com/omar-shahieen/url-shortner/internal/repository/cached/bloom"
	"github.com/omar-shahieen/url-shortner/internal/repository/cached/lru"
)

const (
	// defaultCacheSize is the number of URLs held in the LRU cache.
	defaultCacheSize = 1024
	// defaultExpectedItems is used for bloom filter sizing when the caller
	// does not specify an expected item count.
	defaultExpectedItems = 100_000
	// defaultFalsePositiveRate is the target bloom filter FP rate.
	defaultFalsePositiveRate = 0.01
)

// CodeLister is a supplementary interface the underlying repository must
// satisfy so the cached layer can rebuild the bloom filter on startup.
type CodeLister interface {
	AllCodes(ctx context.Context) ([]string, error)
}

// Repository is a caching decorator around repository.Repository.
type Repository struct {
	inner  repository.Repository
	cache  *lru.Cache[*model.URL]
	filter *bloom.Filter
}

// New wraps inner with an LRU cache (capacity cacheSize) and a bloom filter
// (sized for expectedItems at a 1 % false-positive rate).
// Pass 0 for either to use the defaults.
func New(inner repository.Repository, cacheSize, expectedItems int) *Repository {
	if cacheSize <= 0 {
		cacheSize = defaultCacheSize
	}
	if expectedItems <= 0 {
		expectedItems = defaultExpectedItems
	}
	return &Repository{
		inner:  inner,
		cache:  lru.New[*model.URL](cacheSize),
		filter: bloom.New(expectedItems, defaultFalsePositiveRate),
	}
}

// Build rebuilds the bloom filter from all codes currently stored in inner.
// inner must implement CodeLister; if it does not, Build is a no-op.
// Call this once after New, before serving traffic.
func (r *Repository) Build(ctx context.Context) error {
	lister, ok := r.inner.(CodeLister)
	if !ok {
		return nil
	}
	codes, err := lister.AllCodes(ctx)
	if err != nil {
		return fmt.Errorf("cached: rebuild bloom filter: %w", err)
	}
	for _, code := range codes {
		r.filter.Add(code)
	}
	return nil
}

// Save stores url in the underlying repository, then seeds both the bloom
// filter and the LRU cache with the new entry.
func (r *Repository) Save(ctx context.Context, url *model.URL) error {
	if err := r.inner.Save(ctx, url); err != nil {
		return err
	}
	r.filter.Add(url.Code)
	r.cache.Put(url.Code, url)
	return nil
}

// FindByCode looks up a short code.
//
// Fast path (bloom miss): if the filter says the code definitely does not
// exist, return ErrNotFound without touching the DB.
//
// Cache hit: if the LRU has the entry, return it immediately.
//
// Cache miss: read from the underlying repository, populate the cache, and
// return the result.
func (r *Repository) FindByCode(ctx context.Context, code string) (*model.URL, error) {
	// bloom filter — definite miss
	if !r.filter.Test(code) {
		return nil, model.ErrNotFound
	}

	// LRU hit
	if url, ok := r.cache.Get(code); ok {
		return url, nil
	}

	// cache miss — go to the underlying repository
	url, err := r.inner.FindByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	r.cache.Put(code, url)
	return url, nil
}

// IncrementClicks increments the click counter in the underlying repository
// and invalidates the LRU entry so the next read gets a fresh copy.
func (r *Repository) IncrementClicks(ctx context.Context, code string) error {
	if err := r.inner.IncrementClicks(ctx, code); err != nil {
		return err
	}
	r.cache.Delete(code)
	return nil
}

// Exists checks the bloom filter first; if that says no, returns false
// without a DB call.  If the filter says maybe, delegates to the underlying
// repository for an authoritative answer.
func (r *Repository) Exists(ctx context.Context, code string) (bool, error) {
	if !r.filter.Test(code) {
		return false, nil
	}
	return r.inner.Exists(ctx, code)
}
