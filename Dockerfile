# Compiles CDS from this repository, for local development, CI and the
# deployment pipeline. It stays at the repository root because that is where a
# bare `docker build .` looks for it.
#
# To package a *release* instead, use distribution/dockerfiles/alpine/cds, which
# downloads a published platform pack rather than compiling. That is the same
# input contract the other WSO2 product images use.

# Allow overriding base images
ARG GO_BASE=golang:1.26.5
# Pinned rather than :latest, so a rebuild of an older commit produces the same
# image and an upstream Alpine change cannot break a release build unannounced.
ARG RUNTIME_BASE=alpine:3.21

# -------------------------
# Stage 1: Builder
# -------------------------
FROM ${GO_BASE} AS builder

# Install zip (Debian-based images)
RUN apt-get update && apt-get install -y zip && rm -rf /var/lib/apt/lists/*

# Supplied automatically by buildx when --platform is used. They are passed
# through empty rather than defaulted, because Go treats an empty GOOS/GOARCH as
# unset and builds for the builder's own platform. Defaulting to amd64 instead
# would make a plain `docker build` on an arm64 host produce an amd64 binary,
# since the legacy builder supplies no TARGETARCH at all.
ARG TARGETOS
ARG TARGETARCH

# Build environment
ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=${TARGETOS} \
    GOARCH=${TARGETARCH}

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
RUN addgroup -g 10001 appgroup && \
    adduser  -D -u 10001 -G appgroup appuser

# Use /app (matches your Helm volume mounts)
WORKDIR /app

# Copy binary + config
COPY --from=builder /app/target/.build/cds .
COPY --from=builder /app/config ./config
COPY --from=builder /app/config/repository ./repository
COPY --from=builder /app/dbscripts ./dbscripts
COPY --from=builder /app/version.txt .
COPY --from=builder /app/LICENSE.txt .

# Ensure correct permissions
RUN chown -R 10001:10001 /app && \
    chmod 755 /app/cds

# Switch to non-root (UID/GID 10001)
USER 10001:10001

EXPOSE 8900

CMD ["./cds"]
