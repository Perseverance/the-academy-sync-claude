# Multi-stage Dockerfile for Go services
# Usage: docker build --build-arg SERVICE_NAME=<service-name> -t <image-name> .

# Stage 1: Build stage
FROM golang:1.22-alpine AS builder

# Install git for Go module downloads and air for live reloading
RUN apk add --no-cache git && \
    go install github.com/cosmtrek/air@v1.49.0

# Set working directory
WORKDIR /app

# Accept build argument for service name
ARG SERVICE_NAME
RUN case "$SERVICE_NAME" in \
        "backend-api"|"automation-engine"|"notification-service") \
            echo "Building service: $SERVICE_NAME" ;; \
        *) \
            echo "ERROR: SERVICE_NAME must be one of: backend-api, automation-engine, notification-service" && \
            echo "Provided: $SERVICE_NAME" && \
            false ;; \
    esac

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

# Stage 2: Development image with air for live reloading
FROM golang:1.22-alpine AS development

# Install git and air
RUN apk add --no-cache git && \
    go install github.com/cosmtrek/air@v1.49.0

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Default command for development (can be overridden)
CMD ["air"]

# Stage 3: Final minimal production image
FROM gcr.io/distroless/static-debian12 AS production

# Copy only the compiled binary from builder stage
COPY --from=builder /app/service /service

# Set the binary as the entrypoint
ENTRYPOINT ["/service"]