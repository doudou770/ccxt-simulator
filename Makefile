.PHONY: build run test clean migrate

# Build the application
build:
	go build -o bin/server.exe ./cmd/server

# Run the application
run:
	go run ./cmd/server

# Run tests
test:
	go test -v ./...

# Run tests with coverage
test-cover:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html

# Download dependencies
deps:
	go mod download
	go mod tidy

# Format code
fmt:
	go fmt ./...

# Lint code
lint:
	golangci-lint run

# Generate mocks (if using mockgen)
mock:
	go generate ./...

# Docker build
docker-build:
	docker build -t ccxt-simulator:latest .

# Docker run
docker-run:
	docker-compose up -d

# Docker stop
docker-stop:
	docker-compose down
