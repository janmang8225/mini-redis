# ── Stage 1: build ────────────────────────────────────────────────────────────
FROM golang:1.23-alpine AS builder

WORKDIR /app

# cache deps first — only re-runs if go.mod/go.sum change
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# build a statically-linked binary — no libc dependency
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o miniredis \
    ./cmd/miniredis

# ── Stage 2: runtime ──────────────────────────────────────────────────────────
FROM scratch

# copy CA certs (needed if we ever make outbound HTTPS calls)
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# copy just the binary
COPY --from=builder /app/miniredis /miniredis

# copy default config
COPY --from=builder /app/config.yaml /config.yaml

# data directory for AOF + snapshots
VOLUME ["/data"]

# redis-compatible port + dashboard port
EXPOSE 6379 8080

ENTRYPOINT ["/miniredis", "--config", "/config.yaml"]