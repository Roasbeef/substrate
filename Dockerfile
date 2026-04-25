# Dockerfile for substrate: the agent command center for coordinating
# Claude Code agents via gRPC mail, identity persistence, and a web
# dashboard.
#
# Three-stage build:
#   1. Frontend: bun builds the React SPA
#   2. Go: compiles substrated (daemon) and substrate (CLI) with the
#      embedded frontend assets
#   3. Runtime: minimal Alpine image with both binaries

# ---- Stage 1: Frontend build ----
FROM oven/bun:1-alpine AS frontend

WORKDIR /app/web/frontend
COPY web/frontend/package.json web/frontend/bun.lock* ./
RUN bun install --frozen-lockfile

COPY web/frontend/ .
RUN bun run build

# ---- Stage 2: Go build ----
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git build-base

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Overlay the built frontend assets so the go:embed directive in
# web/frontend_embed.go picks them up.
COPY --from=frontend /app/web/frontend/dist ./web/frontend/dist

ARG COMMIT=dev

# Build the daemon and CLI. CGO is required for SQLite; FTS5 is used by
# the messages_fts virtual table created in migration 000001.
ENV CGO_CFLAGS="-DSQLITE_ENABLE_FTS5"

RUN go build -tags "osusergo,netgo" \
    -ldflags "-s -w \
      -X github.com/roasbeef/subtrate/internal/build.Commit=${COMMIT}" \
    -o /out/substrated ./cmd/substrated

RUN go build -tags "osusergo,netgo" \
    -ldflags "-s -w \
      -X github.com/roasbeef/subtrate/internal/build.Commit=${COMMIT}" \
    -o /out/substrate ./cmd/substrate

# ---- Stage 3: Runtime ----
FROM alpine:3.20

RUN apk add --no-cache ca-certificates sqlite bash

COPY --from=builder /out/substrated /usr/local/bin/substrated
COPY --from=builder /out/substrate /usr/local/bin/substrate

# Create data directories for the SQLite database and log files.
RUN mkdir -p /data /var/log/substrated

EXPOSE 8080 10009

# Bind gRPC to 0.0.0.0 so other pods can reach it via the K8s Service.
ENTRYPOINT ["substrated"]
CMD ["--db", "/data/subtrate.db", "--web", ":8080", "--grpc", "0.0.0.0:10009", "--log-dir", "/var/log/substrated"]
