# Add a build-time argument for the base images so they can be overridden during docker build
ARG GO_BASE=golang:1.24
ARG RUNTIME_BASE=alpine:latest

# Stage 1: Builder
FROM ${GO_BASE} AS builder

# Install zip for packaging
RUN apt-get update && apt-get install -y zip && rm -rf /var/lib/apt/lists/*

# Set environment variables
ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

# Set working directory
WORKDIR /app

# Copy everything
COPY . .

# Build using the Makefile (resolves version, directories, etc.)
# Ensure Makefile exists and make build produces /app/target/.build/cds
RUN test -f Makefile || (echo "ERROR: Makefile not found!" && exit 1) && \
    make build && \
    test -f target/.build/cds || (echo "ERROR: Build output target/.build/cds not found after make build!" && exit 1)

# Stage 2: Minimal runtime image
FROM ${RUNTIME_BASE}

# Set work directory
WORKDIR /root/

# Copy built binary from the Makefile output
COPY --from=builder /app/target/.build/cds .

# Copy configuration files
COPY --from=builder /app/config ./config

# Optionally copy configuration/repository if required at runtime
COPY --from=builder /app/config/repository ./repository
COPY --from=builder /app/dbscripts ./dbscripts
COPY --from=builder /app/version.txt .

# Expose the app port
EXPOSE 8080

# Start the service
CMD ["./cds"]
