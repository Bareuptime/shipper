.PHONY: build run dev test clean fmt tidy docker-build docker-run docker-stop

# Build the application
build:
	go build -o shipper-deployment ./cmd/shipper

# Run the application locally
run:
	go run ./cmd/shipper

# Development with hot reload (Docker-based)
dev:
	docker-compose -f docker-compose.dev.yml up --build

# Run tests
test:
	go test ./test -v

# Clean build artifacts
clean:
	rm -f shipper-deployment

# Format code
fmt:
	go fmt ./...

# Tidy modules
tidy:
	go mod tidy

# Build Docker image
docker-build:
	docker build -t shipper-deployment .

# Stop Docker Compose
docker-stop:
	docker-compose down
