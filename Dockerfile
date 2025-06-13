# Multi-stage Dockerfile for Go services
# Usage: docker build --build-arg SERVICE_NAME=<service-name> -t <image-name> .

# Stage 1: Build stage
FROM golang:1.22-alpine AS builder

# Install git for Go module downloads
RUN apk add --no-cache git

# Set working directory
WORKDIR /app

# Accept build argument for service name
ARG SERVICE_NAME
RUN test -n "$SERVICE_NAME" || (echo "SERVICE_NAME build argument is required" && false)

# Copy go.mod and go.sum for dependency caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy all source code
COPY . .

# Build the specific service as a statically-linked binary for Linux
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o /app/service \
    ./cmd/${SERVICE_NAME}

# Stage 2: Final minimal image
FROM gcr.io/distroless/static-debian12

# Copy only the compiled binary from builder stage
COPY --from=builder /app/service /service

# Set the binary as the entrypoint
ENTRYPOINT ["/service"]