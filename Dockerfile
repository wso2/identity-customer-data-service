# Allow overriding base images
ARG GO_BASE=golang:1.24
ARG RUNTIME_BASE=acrasgardeomainshared001.azurecr.io/eclipse-temurin:21.0.9_10-jdk-jammy_2025-12-08_14-14-42@sha256:1e8c81c5d9b5ceba3a0251bfec2886f5927b79e255e16687d27082cf33c41aa5

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

# -------------------------
# Create user/group with UID/GID 10001
# -------------------------
RUN \
groupadd -g 10001 -r appgroup && useradd -u 10001 -r appuser \
-g appgroup -d /app

# Use /app (matches your Helm volume mounts)
WORKDIR /app

# Copy binary + config
COPY --from=builder /app/target/.build/cds .
COPY --from=builder /app/config ./config
COPY --from=builder /app/config/repository ./repository
COPY --from=builder /app/dbscripts ./dbscripts
COPY --from=builder /app/version.txt .

# Ensure correct permissions
RUN chown -R 10001:10001 /app && \
    chmod 755 /app/cds

# Switch to non-root (UID/GID 10001)
USER 10001:10001

EXPOSE 8900

CMD ["./cds"]
