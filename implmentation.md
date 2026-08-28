## Implementation Plan — Go URL Shortener

### Phase 0 — Project setup

- `go mod init`, repo structure (`cmd/`, `internal/handler|middleware|service|repository|model`)
- Dependencies: just `modernc.org/sqlite` (pure-Go SQLite driver) — **LRU cache and bloom filter are hand-rolled, no external deps for either**
- `taskfile.yml` or documented `go build`/`go test ./...` commands

### Phase 1 — Domain model (no I/O)

- `model.URL`: `Code`, `OriginalURL`, `CreatedAt`, `ExpiresAt *time.Time`, `ClickCount`, `IsCustomAlias bool` , `CreatedAt` , `UpdatedAt`
- Sentinel errors: `ErrNotFound`, `ErrAliasTaken`, `ErrInvalidAlias`, `ErrCodeGenerationExhausted`, `ErrExpired`
- Unit tests for model helpers (e.g. `IsExpired()`)
- add validation and json parsing to url struct

### Phase 2 — Code generation (pure functions)

- `Generator`: sha256(longURL + salt) → mod 62^7 → fixed 7-char base62 encode
- `AliasValidator`: charset (alnum + `-`/`_`), length bounds, reserved-word list (`api`, `stats`, `health`), case-sensitive
- Table-driven unit tests, zero dependencies

### Phase 3 — Repository interface + in-memory implementation

- `Repository` interface: `Save`, `FindByCode`, `IncrementClicks`, `Exists`
- `inmemory.Repository` (`map[string]*model.URL` + `sync.RWMutex`)
- `service.Service`: collision-retry loop (non-idempotent generation, atomic salt counter), TTL check on read, click increment
- Integration tests: `service` + `inmemory.Repository` together

→ Fully working, fully tested core with no HTTP, no real DB.

### Phase 4 — HTTP layer

- `net/http` with Go 1.22+ pattern routing: `POST /api/shorten`, `GET /{code}`, `GET /api/stats/{code}`
- Handler tests via `httptest`, still backed by `inmemory.Repository`
- Browser preview page at `GET /` for creating and opening short links

→ `go run cmd/server/main.go` gives a working, curl-able shortener — not yet persistent.

### Phase 5 — SQLite repository

- Schema: `code TEXT PRIMARY KEY, original_url TEXT, created_at, expires_at, click_count`
- `sqlite.Repository` implementing `Repository`; swapped in via `main.go` with **zero changes** to `service`/`handler`
- Simple `CREATE TABLE IF NOT EXISTS` on startup, no migration framework needed

### Phase 6 — Caching decorator (hand-rolled LRU + bloom filter)

- **LRU**: `map[string]*node` + doubly-linked list with sentinel head/tail nodes, O(1) get/put/evict, guarded by `sync.Mutex` (not `RWMutex` — `Get` mutates list order, so no pure-read path exists)
- **Bloom filter**: `[]uint64` bit array, `optimalM`/`optimalK` sizing from expected items + target false-positive rate, double-hashing (`h1 + i*h2`) via two seeded FNV-1a hashes for the k positions, guarded by `sync.RWMutex` (`Test` is genuinely read-only here)
- `cached.Repository` wraps `Repository`, composes both
- **Startup rebuild**: load all existing codes from SQLite into the bloom filter on boot (cache can stay cold and warm up naturally)
- Tests: LRU hit/miss/eviction ordering; bloom filter must never false-negative and should stay near target false-positive rate under randomized testing

### Phase 7 — Middleware

- Token-bucket rate limiter (per-IP), `middleware.RateLimit`, applied to `POST /api/shorten` only
- Logging + panic-recovery middleware

### Phase 8 — TTL expiry sweep (optional polish)

- Background goroutine + `time.Ticker`, purges expired SQLite rows (lazy expiry-on-read already covers correctness; this is storage hygiene)
- Graceful shutdown via `context.Context` cancellation

### Phase 9 — Wrap-up

- README with setup/run instructions
- `go vet` / `golangci-lint` pass
- Optional: Dockerfile

---

Everything from Phase 0 on is now a settled decision — no open forks left in the plan. Ready to start on Phase 1, or want to review anything else before writing code?
