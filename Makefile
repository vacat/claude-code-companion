.PHONY: build clean test run dev build-linux build-windows

BINARY_NAME=claude-proxy
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_VERSION?=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildVersion=$(BUILD_VERSION)"

# Build for current platform
build:
	go build $(LDFLAGS) -o $(BINARY_NAME) ./cmd/

# Build for Linux
build-linux:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY_NAME)-linux ./cmd/

# Build for Windows
build-windows:
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY_NAME)-windows.exe ./cmd/

# Build for all platforms
build-all: build build-linux build-windows

# Clean build artifacts
clean:
	rm -f $(BINARY_NAME) $(BINARY_NAME)-linux $(BINARY_NAME)-windows.exe
	rm -rf logs/

# Run tests
test:
	go test -v ./...

# Run with default config
run: build
	./$(BINARY_NAME) -config config.yaml

# Development mode with auto-reload (requires air: go install github.com/cosmtrek/air@latest)
dev:
	air

# Initialize go modules
init:
	go mod tidy

# Install dependencies
deps:
	go mod download

# Format code
fmt:
	go fmt ./...

# Lint code (requires golangci-lint)
lint:
	golangci-lint run

# Show help
help:
	@echo "Available targets:"
	@echo "  build        - Build binary for current platform"
	@echo "  build-linux  - Build binary for Linux"
	@echo "  build-windows- Build binary for Windows"  
	@echo "  build-all    - Build for all platforms"
	@echo "  clean        - Remove build artifacts"
	@echo "  test         - Run tests"
	@echo "  run          - Build and run with default config"
	@echo "  dev          - Run in development mode with hot reload"
	@echo "  init         - Initialize and tidy go modules"
	@echo "  deps         - Download dependencies"
	@echo "  fmt          - Format code"
	@echo "  lint         - Lint code"
	@echo "  help         - Show this help"