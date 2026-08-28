# ── Stage 1: build ──────────────────────────────────────────────────────────
FROM golang:1.24-alpine AS builder

WORKDIR /src

# Download dependencies first (layer-cached unless go.mod/go.sum change)
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build a fully static binary.
# modernc.org/sqlite is pure-Go, so CGO_ENABLED=0 is safe.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
      -ldflags="-s -w" \
      -trimpath \
      -o /url-shortener \
      ./cmd/server

# ── Stage 2: runtime ─────────────────────────────────────────────────────────
FROM scratch

# Import CA certificates so outbound HTTPS calls (if any) work.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the static binary.
COPY --from=builder /url-shortener /url-shortener

# SQLite database lives in a dedicated directory so it can be volume-mounted.
VOLUME ["/data"]

# The server writes url-shortener.db to the working directory, so set it to
# the volume mount point.
WORKDIR /data

EXPOSE 8080

ENTRYPOINT ["/url-shortener"]
