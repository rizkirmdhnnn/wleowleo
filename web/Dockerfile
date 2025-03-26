FROM golang:alpine

WORKDIR /app

# Install required dependencies
RUN apk add --no-cache git

# Copy go.mod and go.sum files
COPY go.mod go.sum* ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN go build -o web-server .

# Expose port
EXPOSE 8080

# Run the application
CMD ["/app/web-server"]