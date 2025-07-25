.PHONY: build run docker-build docker-run clean dev install-air

# Build the Go application
build:
	go build -o shipper-deployment ./cmd/shipper

# Run the application locally
run:
	go run ./cmd/shipper

# Install Air for hot reload
install-air:
	go install github.com/air-verse/air@latest

# Run with hot reload for development (Docker-based)
dev:
	docker-compose -f docker-compose.dev.yml up --build

# Run with hot reload for development (local - requires air)
dev-local:
	air

# Run with Docker hot reload (alias for dev)
dev-docker:
	docker-compose -f docker-compose.dev.yml up --build

# Build Docker image
docker-build:
	docker build -t shipper-deployment .

# Run with Docker Compose
docker-run:
	docker-compose up -d

# Stop Docker Compose
docker-stop:
	docker-compose down

# Clean build artifacts
clean:
	rm -f shipper-deployment
	rm -f shipper.db
	rm -rf tmp/

# Run tests
test:
	go test ./...

# Format code
fmt:
	go fmt ./...

# Tidy modules
tidy:
	go mod tidy
