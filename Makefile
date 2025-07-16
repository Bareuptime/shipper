.PHONY: build run docker-build docker-run clean

# Build the Go application
build:
	go build -o bastion-deployment .

# Run the application locally
run:
	go run .

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

# Run tests
test:
	go test ./...

# Format code
fmt:
	go fmt ./...

# Tidy modules
tidy:
	go mod tidy
