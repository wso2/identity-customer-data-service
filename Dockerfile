# Use Golang base image
FROM golang:1.21 AS build

# Set environment variables
ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

# Create a working directory
WORKDIR /app

# Copy go.mod and go.sum first (for dependency caching)
COPY go.mod go.sum ./

# Download dependencies
RUN go mod tidy

# Copy the rest of the project files
COPY . .

# Build the application
RUN go build -o gin-server

# Use a minimal image for running the server
FROM alpine:latest

# Set work directory
WORKDIR /root/

# Copy the built binary
COPY --from=build /app/gin-server .

# Expose the port (Ensure it's the same as in Choreo configurations)
EXPOSE 8080

# Run the binary
CMD ["./gin-server"]
