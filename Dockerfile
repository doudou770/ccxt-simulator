# Build stage
FROM golang:1.23-alpine AS builder

# Install dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build arguments for version info
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown

# Build the application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.BuildTime=${BUILD_TIME}" \
    -o /app/bin/server ./cmd/server

# Runtime stage
FROM alpine:3.19

# Install ca-certificates for HTTPS connections
RUN apk --no-cache add ca-certificates tzdata

# Set timezone
ENV TZ=Asia/Shanghai

# Create non-root user
RUN adduser -D -g '' appuser

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/bin/server /app/server

# Copy config file (will be overridden by volume mount)
COPY config.yaml /app/config.yaml

# Change ownership
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 11188

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:11188/health || exit 1

# Run the application
ENTRYPOINT ["/app/server"]
