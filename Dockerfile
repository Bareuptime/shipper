# Build stage
FROM golang:1.24-alpine AS builder

# Install required packages for CGO and SQLite
RUN apk add --no-cache \
    gcc \
    musl-dev \
    sqlite-dev \
    build-base

# Set CGO environment and SQLite build tags
ENV CGO_ENABLED=1
ENV GOOS=linux
ENV CGO_CFLAGS="-D_LARGEFILE64_SOURCE"

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN go build -tags 'sqlite_omit_load_extension' -a -installsuffix cgo -o bastion-deployment ./cmd/bastion

# Runtime stage
FROM alpine:latest

# Install sqlite
RUN apk add --no-cache sqlite

WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/bastion-deployment .

# Create directory for database
RUN mkdir -p /root/data

# Expose port
EXPOSE 16166

# Run the binary
CMD ["./bastion-deployment"]
