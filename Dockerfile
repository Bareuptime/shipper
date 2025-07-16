# Build stage
FROM golang:1.21-alpine AS builder

# Install required packages
RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o bastion-deployment .

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
EXPOSE 8080

# Run the binary
CMD ["./bastion-deployment"]
