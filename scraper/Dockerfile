FROM golang:alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o wleowleo

# Create final image
FROM alpine:latest

WORKDIR /app

# Install Chrome and FFmpeg
RUN apk add --no-cache \
    chromium \
    ffmpeg \
    ca-certificates \
    tzdata

# Set Chrome environment variables
ENV CHROME_BIN=/usr/bin/chromium-browser \
    CHROME_PATH=/usr/lib/chromium/

# Create directories for output and temp files
RUN mkdir -p /app/output /app/temp

# Copy binary from builder stage
COPY --from=builder /app/wleowleo /app/

# Set the entrypoint
ENTRYPOINT ["/app/wleowleo"]