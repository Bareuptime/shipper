.PHONY: build run docker-build docker-run clean dev install-air

# Build the Go application
build:
	go build -o bastion-deployment .

# Run the application locally
run:
	go run .

# Install Air for hot reload
install-air:
	go install github.com/air-verse/air@latest

# Run with hot reload for development
dev:
	air

# Run development script
dev-script:
	./dev.sh

# Run with Docker hot reload
dev-docker:
	docker-compose -f docker-compose.dev.yml up --build

# Build Docker image
docker-build:
	docker build -t bastion-deployment .

# Run with Docker Compose
docker-run:
	docker-compose up -d

# Stop Docker Compose
docker-stop:
	docker-compose down

# Clean build artifacts
clean:
	rm -f bastion-deployment
	rm -f bastion.db
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
