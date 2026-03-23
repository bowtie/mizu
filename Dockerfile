# =============================================================================
# Mizu - Multi-stage Docker Build
# =============================================================================

# Stage 1: Build the Go binary
FROM golang:1.22-alpine AS builder

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

WORKDIR /build

# Cache Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build with ldflags for version info, static binary
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-s -w \
        -X main.version=${VERSION} \
        -X main.commit=${COMMIT} \
        -X main.buildDate=${BUILD_DATE}" \
    -o /mizu \
    ./cmd/server

# Stage 2: Minimal runtime image
FROM alpine:3.19

# Install ca-certificates for HTTPS requests + tzdata for timezone support
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 afg && \
    adduser -u 1000 -G afg -s /bin/sh -D afg

# Create config directory
RUN mkdir -p /config && chown afg:afg /config

# Copy binary from builder
COPY --from=builder /mizu /usr/local/bin/mizu

# Environment defaults
ENV SONARR_URL=http://localhost:8989 \
    SONARR_API_KEY= \
    SCAN_INTERVAL=6h \
    LISTEN_ADDR=:8978 \
    DB_PATH=/config/mizu.db \
    LOG_LEVEL=info \
    TZ=UTC

# Expose port
EXPOSE 8978

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:8978/api/health || exit 1

# Persistent storage
VOLUME /config

# Run as non-root
USER afg

ENTRYPOINT ["mizu"]
