# Build stage
FROM golang:1.25-alpine AS builder

# Install git (required for go modules)
RUN apk add --no-cache git

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o mariadb-extractor .

# Final stage
FROM alpine:latest

# Install MariaDB client tools and ca-certificates
RUN apk add --no-cache \
    mariadb-client \
    gzip \
    ca-certificates \
    && rm -rf /var/cache/apk/*

# Create a non-root user
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# Set working directory
WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /app/mariadb-extractor .

# Change ownership to non-root user
RUN chown appuser:appgroup mariadb-extractor

# Switch to non-root user
USER appuser

# Create volume for output files
VOLUME ["/app/output"]

# Set the binary as entrypoint
ENTRYPOINT ["./mariadb-extractor"]

# Default command
CMD ["--help"]