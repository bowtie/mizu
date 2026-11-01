# =============================================================================
# Mizu - Multi-stage Docker Build
# =============================================================================

# Stage 1: Build the frontend (Astryx React app via Vite+)
FROM oven/bun:1.4.2 AS frontend

WORKDIR /frontend

# Cache deps
COPY web/package.json web/bun.lock ./
RUN bun install --frozen-lockfile

# Build (typecheck first so a type error fails the image build, matching the
# local `bun run build` script)
COPY web/ ./
RUN bun run build

# Stage 2: Build the Go binary
FROM golang:1.27-alpine AS builder

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
FROM alpine:3.24

# Install ca-certificates for HTTPS requests + tzdata for timezone support
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 afg && \
    adduser -u 1000 -G afg -s /bin/sh -D afg

# Create config directory
RUN mkdir -p /config && chown afg:afg /config

# Copy binary from builder
COPY --from=builder /mizu /usr/local/bin/mizu

# Bind all interfaces: a loopback bind inside the container is unreachable
# through the published port or a reverse proxy. Override with -e MIZU_ADDR=...
ENV MIZU_ADDR=0.0.0.0:8978

# Timezone only — all app config lives in SQLite (app_settings table)
ENV TZ=UTC

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
