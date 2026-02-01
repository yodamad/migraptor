# Build stage
FROM golang:1.25.6-alpine AS builder

WORKDIR /app

# Install git and other build dependencies
RUN apk add --no-cache git ca-certificates

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o migraptor ./cmd/migrate

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/migraptor .

# Create a non-root user for security and writable directories
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser && \
    mkdir -p /tmp /home/appuser && \
    chown -R appuser:appuser /app /tmp /home/appuser

USER appuser

# Set the entrypoint
ENTRYPOINT ["./migraptor"]

# Default command (can be overridden)
CMD ["--help"]

