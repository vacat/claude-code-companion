.PHONY: build clean test run dev windows-amd64 linux-amd64 linux-arm64 darwin-amd64 darwin-arm64 all

BINARY_NAME=claude-proxy

# Generate version in format: YYYYMMDD-<short-hash>[-dirty][-release]
define GET_VERSION
$(shell \
	if command -v date >/dev/null 2>&1; then \
		DATE=$$(date +%Y%m%d); \
	else \
		DATE=$$(powershell -Command "Get-Date -Format 'yyyyMMdd'" 2>/dev/null || echo "$(shell echo %date:~10,4%%date:~4,2%%date:~7,2%)"); \
	fi; \
	HASH=$$(git rev-parse --short HEAD 2>/dev/null || echo "unknown"); \
	if [ "$$HASH" != "unknown" ]; then \
		VERSION="$$DATE-$$HASH"; \
		if ! git diff-index --quiet HEAD 2>/dev/null; then \
			VERSION="$$VERSION-dirty"; \
		fi; \
		if [ "$$RELEASE_BUILD" = "true" ]; then \
			VERSION="$$VERSION-release"; \
		fi; \
		echo "$$VERSION"; \
	else \
		echo "dev"; \
	fi \
)
endef

VERSION?=$(GET_VERSION)
LDFLAGS=-ldflags "-X main.Version=$(VERSION)"

# Build for current platform
build:
	@echo "Building with version: $(VERSION)"
	go build $(LDFLAGS) -o $(BINARY_NAME) .

# Cross-compile for Windows x64
windows-amd64:
	@echo "Building Windows AMD64 with version: $(VERSION)"
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY_NAME)-windows-amd64.exe .

# Cross-compile for Linux x64
linux-amd64:
	@echo "Building Linux AMD64 with version: $(VERSION)"
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY_NAME)-linux-amd64 .

# Cross-compile for Linux ARM64
linux-arm64:
	@echo "Building Linux ARM64 with version: $(VERSION)"
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BINARY_NAME)-linux-arm64 .

# Cross-compile for macOS Intel
darwin-amd64:
	@echo "Building macOS Intel with version: $(VERSION)"
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY_NAME)-darwin-amd64 .

# Cross-compile for macOS Apple Silicon
darwin-arm64:
	@echo "Building macOS Apple Silicon with version: $(VERSION)"
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BINARY_NAME)-darwin-arm64 .

# Cross-compile for all platforms
all: windows-amd64 linux-amd64 linux-arm64 darwin-amd64 darwin-arm64

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
	@echo "  darwin-amd64   - Cross-compile for macOS Intel"
	@echo "  darwin-arm64   - Cross-compile for macOS Apple Silicon"
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