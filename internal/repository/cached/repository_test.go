package cached_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/omar-shahieen/url-shortner/internal/model"
	"github.com/omar-shahieen/url-shortner/internal/repository/cached"
)

// --- fake inner repository ---

type fakeRepo struct {
	mu    sync.Mutex
	store map[string]*model.URL
	finds int // counts DB round-trips
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{store: make(map[string]*model.URL)}
}

func (f *fakeRepo) Save(_ context.Context, url *model.URL) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.store[url.Code]; ok {
		return model.ErrAliasTaken
	}
	f.store[url.Code] = url
	return nil
}

func (f *fakeRepo) FindByCode(_ context.Context, code string) (*model.URL, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.finds++
	url, ok := f.store[code]
	if !ok {
		return nil, model.ErrNotFound
	}
	return url, nil
}

func (f *fakeRepo) IncrementClicks(_ context.Context, code string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	url, ok := f.store[code]
	if !ok {
		return model.ErrNotFound
	}
	url.ClickCount++
	return nil
}

func (f *fakeRepo) Exists(_ context.Context, code string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.store[code]
	return ok, nil
}

func (f *fakeRepo) AllCodes(_ context.Context) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	codes := make([]string, 0, len(f.store))
	for code := range f.store {
		codes = append(codes, code)
	}
	return codes, nil
}

// --- helpers ---

func makeURL(code string) *model.URL {
	return &model.URL{Code: code, OriginalURL: "https://example.com", CreatedAt: time.Now()}
}

// --- tests ---

func TestSaveAndFind(t *testing.T) {
	ctx := context.Background()
	inner := newFakeRepo()
	repo := cached.New(inner, 64, 1000)

	url := makeURL("abc")
	if err := repo.Save(ctx, url); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	found, err := repo.FindByCode(ctx, url.Code)
	if err != nil {
		t.Fatalf("FindByCode() error = %v", err)
	}
	if found.Code != url.Code {
		t.Errorf("FindByCode() code = %q, want %q", found.Code, url.Code)
	}

	// Second find should be served from the LRU — DB find count stays at 0.
	_, _ = repo.FindByCode(ctx, url.Code)
	if inner.finds != 0 {
		t.Errorf("expected 0 DB round-trips after cache warm-up, got %d", inner.finds)
	}
}

func TestBloomFilterShortCircuitsAbsentCode(t *testing.T) {
	ctx := context.Background()
	inner := newFakeRepo()
	repo := cached.New(inner, 64, 1000)

	_, err := repo.FindByCode(ctx, "never-added")
	if !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("FindByCode() error = %v, want ErrNotFound", err)
	}
	// The bloom filter must have answered — inner.finds must be 0.
	if inner.finds != 0 {
		t.Errorf("bloom filter should have short-circuited DB call; finds = %d", inner.finds)
	}
}

func TestBuildSeedsBloomFilter(t *testing.T) {
	ctx := context.Background()
	inner := newFakeRepo()
	_ = inner.Save(ctx, makeURL("preexisting"))

	repo := cached.New(inner, 64, 1000)
	if err := repo.Build(ctx); err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	// After Build, the bloom filter knows about "preexisting".
	// FindByCode should go through to the DB (cache is cold) but NOT be
	// blocked by the bloom filter returning false.
	_, err := repo.FindByCode(ctx, "preexisting")
	if err != nil {
		t.Fatalf("FindByCode() after Build() error = %v", err)
	}
}

func TestIncrementClicksInvalidatesCache(t *testing.T) {
	ctx := context.Background()
	inner := newFakeRepo()
	repo := cached.New(inner, 64, 1000)

	url := makeURL("xyz")
	if err := repo.Save(ctx, url); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Warm the cache.
	_, _ = repo.FindByCode(ctx, "xyz")

	// Increment — should evict from cache.
	if err := repo.IncrementClicks(ctx, "xyz"); err != nil {
		t.Fatalf("IncrementClicks() error = %v", err)
	}

	// Next FindByCode must hit the DB (cache evicted).
	before := inner.finds
	_, _ = repo.FindByCode(ctx, "xyz")
	if inner.finds == before {
		t.Error("expected a DB round-trip after IncrementClicks invalidated the cache")
	}
}

func TestExistsAbsentCode(t *testing.T) {
	ctx := context.Background()
	inner := newFakeRepo()
	repo := cached.New(inner, 64, 1000)

	exists, err := repo.Exists(ctx, "ghost")
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if exists {
		t.Error("Exists() = true, want false for never-added code")
	}
}

func TestExistsPresentCode(t *testing.T) {
	ctx := context.Background()
	inner := newFakeRepo()
	repo := cached.New(inner, 64, 1000)

	_ = repo.Save(ctx, makeURL("real"))

	exists, err := repo.Exists(ctx, "real")
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if !exists {
		t.Error("Exists() = false, want true for saved code")
	}
}
