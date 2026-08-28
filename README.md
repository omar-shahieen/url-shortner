# URL Shortener

A production-quality URL shortener written in pure Go — no frameworks, no ORM, minimal external dependencies.

## Features

| Layer | Detail |
|---|---|
| **Storage** | SQLite via `modernc.org/sqlite` (pure-Go, no CGo) |
| **Cache** | Hand-rolled LRU (doubly-linked list + hash map, O(1) get/put/evict) |
| **Bloom filter** | Hand-rolled bit-array with optimal *m*/*k* sizing, double-hashing via FNV-1a |
| **Rate limiting** | Per-IP token-bucket on `POST /api/shorten` |
| **Middleware** | Structured request logger (`log/slog`) + panic recovery |
| **TTL sweep** | Background goroutine purges expired rows every 5 minutes |
| **Shutdown** | Graceful — drains in-flight requests within 10 s on SIGINT/SIGTERM |

## API

| Method | Path | Description |
|---|---|---|
| `GET` | `/` | Browser preview page |
| `POST` | `/api/shorten` | Create a short URL |
| `GET` | `/{code}` | Redirect to the original URL |
| `GET` | `/api/stats/{code}` | Return URL metadata + click count |
| `GET` | `/health` | DB health check (`200 ok` / `503 unavailable`) |

### Shorten a URL

```bash
curl -X POST http://localhost:8080/api/shorten \
  -H 'Content-Type: application/json' \
  -d '{"originalUrl":"https://go.dev/doc/","customAlias":"go-docs","expiresAt":"2027-01-01T00:00:00Z"}'
```

```json
{
  "code": "go-docs",
  "originalUrl": "https://go.dev/doc/",
  "createdAt": "2026-08-28T09:00:00Z",
  "expiresAt": "2027-01-01T00:00:00Z",
  "clickCount": 0,
  "isCustomAlias": true
}
```

### Redirect

```bash
curl -L http://localhost:8080/go-docs
```

### Stats

```bash
curl http://localhost:8080/api/stats/go-docs
```

## Getting started

### Prerequisites

- Go 1.22+

### Run

```bash
go run ./cmd/server
```

The server starts on **http://localhost:8080** and creates `url-shortener.db` in the working directory on first run.

### Test

```bash
go test ./...
```

### Build

```bash
go build -o url-shortener ./cmd/server
./url-shortener
```

### Using Taskfile

If you have [Task](https://taskfile.dev) installed:

```bash
task run    # go run ./cmd/server
task test   # go test ./...
task build  # go build -o url-shortener ./cmd/server
```

## Project layout

```
cmd/server/          main.go — wires all layers together
internal/
  handler/           HTTP handlers (Shorten, Redirect, Stats, Preview)
  middleware/        Logger, Recoverer, RateLimiter
  model/             URL struct, sentinel errors, validation
  repository/
    repository.go    Repository interface
    inmemory/        In-memory implementation (tests)
    sqlite/          SQLite implementation (production)
    cached/
      repository.go  LRU + bloom filter decorator
      lru/           Hand-rolled generic LRU cache
      bloom/         Hand-rolled bloom filter
  routers/           Route registration + health endpoint
  service/           Business logic (Shorten, Resolve, Stats)
  sweep/             Background TTL expiry goroutine
```

## Configuration

All configuration is currently via constants in `main.go`:

| Setting | Default | Description |
|---|---|---|
| Listen address | `:8080` | HTTP port |
| DB file | `url-shortener.db` | SQLite database path |
| LRU capacity | `1 024` | Maximum cached URL entries |
| Bloom filter size | `100 000` | Expected total URL count |
| Rate limit | 10 req/s, burst 20 | Per-IP limit on `POST /api/shorten` |
| Sweep interval | 5 minutes | How often expired rows are purged |
| Shutdown timeout | 10 seconds | Graceful drain window |

## Design decisions

- **No migration framework** — `CREATE TABLE IF NOT EXISTS` on startup is enough.
- **No external cache dependency** — the hand-rolled LRU avoids Redis/Memcached overhead for a single-node deployment.
- **Bloom filter for definite misses** — resolving unknown codes (e.g. typos, bots) never hits the DB.
- **`sync.Mutex` (not `RWMutex`) for LRU** — `Get` mutates list order, so there is no pure read path.
- **`sync.RWMutex` for bloom filter** — `Test` is genuinely read-only; only `Add` acquires an exclusive lock.
