# =============================================================================
# Mizu - Multi-stage Docker Build
# =============================================================================
# Base images are pinned by digest (the tag is kept for readability);
# Dependabot proposes digest updates.

# Stage 1: Build the frontend (Astryx React app via Vite+)
# Bun installs, but the build tools must run under Node: the StyleX media-query
# parser fails when Vite runs on the Bun runtime, which is what the plain
# oven/bun image falls back to because it ships no Node.
FROM node:26-slim@sha256:5adad64f9a1fca2d51972caa65983634c8a774eb300d72b20f7f2a0f60368072 AS frontend
COPY --from=oven/bun:1.4.2@sha256:9114c058aeae42162ee16dd5084b95fe9473970bb6bcb5b232ab1630f0546895 /usr/local/bin/bun /usr/local/bin/bun

WORKDIR /frontend

# Cache deps
COPY web/package.json web/bun.lock ./
RUN bun install --frozen-lockfile

# Build (typecheck first so a type error fails the image build, matching the
# local `bun run build` script)
COPY web/ ./
RUN bun run build

# Stage 2: Build the Go binary
FROM golang:1.27-alpine@sha256:d200d2dd76b03a99b52651fd1325aff7dcd35422c35c01bac56571b0507f3deb AS builder

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

WORKDIR /build

# Cache Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Copy frontend build output into the embed directory (replaces any stale content)
COPY --from=frontend /frontend/dist/ ./internal/web/dist/

# Build with ldflags for version info, static binary
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-s -w \
        -X main.version=${VERSION} \
        -X main.commit=${COMMIT} \
        -X main.buildDate=${BUILD_DATE}" \
    -o /mizu \
    ./cmd/server

# Stage 3: Minimal runtime image
FROM alpine:3.24@sha256:be2656061b536c1883c4034790e21d0a09c5aaaf51968b89368a3c0371ea2c9a

# ca-certificates for HTTPS requests to Sonarr, ntfy and artwork providers
RUN apk add --no-cache ca-certificates

# Create non-root user
RUN addgroup -g 1000 afg && \
    adduser -u 1000 -G afg -s /bin/sh -D afg

# Create config directory
RUN mkdir -p /config && chown afg:afg /config

# Copy binary from builder
COPY --from=builder /mizu /usr/local/bin/mizu

# No environment variables: Mizu listens on :8978 and keeps all of its
# configuration in SQLite (/config/mizu.db), set from the web UI.

# Expose port
EXPOSE 8978

# Health check: liveness only (process + database). /api/health also
# reports Sonarr, and a Sonarr outage shouldn't mark Mizu unhealthy.
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:8978/api/health/live || exit 1

# Persistent storage
VOLUME /config

# Run from /config so the default relative DB path (mizu.db) resolves there
WORKDIR /config

# Run as non-root
USER afg

ENTRYPOINT ["mizu"]
