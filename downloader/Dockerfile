FROM golang:alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o wleowleo-downloader

# Create final image
FROM alpine:latest

WORKDIR /app

# Install FFmpeg and CA certificates
RUN apk add --no-cache \
    ffmpeg \
    ca-certificates \
    tzdata

# Create a non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Create directories for output and temp files with proper permissions
RUN mkdir -p /app/output /app/temp && \
    chmod -R 777 /app/output /app/temp && \
    chown -R appuser:appgroup /app

# Copy binary from builder stage
COPY --from=builder /app/wleowleo-downloader /app/

# Switch to non-root user
USER appuser

# Set the entrypoint
ENTRYPOINT ["/app/wleowleo-downloader"]
