.PHONY: build clean test run dev windows-amd64 linux-amd64 linux-arm64 darwin-universal all

BINARY_NAME=claude-proxy
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_VERSION?=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildVersion=$(BUILD_VERSION)"

# Build for current platform
build:
	go build $(LDFLAGS) -o $(BINARY_NAME) ./cmd/

# Cross-compile for Windows x64
windows-amd64:
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY_NAME)-windows-amd64.exe ./cmd/

# Cross-compile for Linux x64
linux-amd64:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY_NAME)-linux-amd64 ./cmd/

# Cross-compile for Linux ARM64
linux-arm64:
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BINARY_NAME)-linux-arm64 ./cmd/

# Cross-compile for macOS Universal (Intel + Apple Silicon)
darwin-universal:
	GOOS=darwin GOARCH=amd64,arm64 go build $(LDFLAGS) -o $(BINARY_NAME)-darwin-universal ./cmd/

# Cross-compile for all platforms
all: windows-amd64 linux-amd64 linux-arm64 darwin-universal

# Clean build artifacts
clean:
	rm -f $(BINARY_NAME) $(BINARY_NAME)-*
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
	@echo "  build          - Build binary for current platform"
	@echo "  windows-amd64  - Cross-compile for Windows x64"
	@echo "  linux-amd64    - Cross-compile for Linux x64"
	@echo "  linux-arm64    - Cross-compile for Linux ARM64"
	@echo "  darwin-universal - Cross-compile for macOS Universal"
	@echo "  all            - Cross-compile for all platforms"
	@echo "  clean          - Remove build artifacts"
	@echo "  test           - Run tests"
	@echo "  run            - Build and run with default config"
	@echo "  dev            - Run in development mode with hot reload"
	@echo "  init           - Initialize and tidy go modules"
	@echo "  deps           - Download dependencies"
	@echo "  fmt            - Format code"
	@echo "  lint           - Lint code"
	@echo "  help           - Show this help"