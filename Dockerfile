# Allow overriding base images
ARG GO_BASE=golang:1.24
ARG RUNTIME_BASE=alpine:latest

# -------------------------
# Stage 1: Builder
# -------------------------
FROM ${GO_BASE} AS builder

# Install zip (Debian-based images)
RUN apt-get update && apt-get install -y zip && rm -rf /var/lib/apt/lists/*

# Build environment
ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

WORKDIR /app

# Copy source
COPY . .

# Validate Makefile + run build
RUN test -f Makefile || (echo "ERROR: Makefile not found!" && exit 1) && \
    make build && \
    test -f target/.build/cds || (echo "ERROR: Build output target/.build/cds not found!" && exit 1)

# -------------------------
# Stage 2: Runtime
# -------------------------
FROM ${RUNTIME_BASE}

# Create non-root user (recommended for security)
RUN adduser -D appuser

# Use a clean app directory (NOT /root)
WORKDIR /app

# Copy binary + config
COPY --from=builder /app/target/.build/cds .
COPY --from=builder /app/config ./config
COPY --from=builder /app/config/repository ./repository
COPY --from=builder /app/dbscripts ./dbscripts
COPY --from=builder /app/version.txt .

# Switch to non-root
USER appuser

EXPOSE 8900

CMD ["./cds"]
