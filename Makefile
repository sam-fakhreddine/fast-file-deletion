# Fast File Deletion - Build Makefile
# Requirements: 9.2, 9.3

# Binary name
BINARY_NAME=fast-file-deletion

# Build directory
BUILD_DIR=bin

# Version (can be overridden)
VERSION?=dev

# Build flags for smaller binaries
LDFLAGS=-ldflags="-s -w -X main.version=$(VERSION)"

# Default target
.PHONY: all
all: clean build-windows

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@mkdir -p $(BUILD_DIR)

# Build for Windows AMD64
.PHONY: build-windows-amd64
build-windows-amd64:
	@echo "Building for Windows AMD64..."
	@GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/fast-file-deletion

# Build for Windows ARM64
.PHONY: build-windows-arm64
build-windows-arm64:
	@echo "Building for Windows ARM64..."
	@GOOS=windows GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-arm64.exe ./cmd/fast-file-deletion

# Build all Windows targets
.PHONY: build-windows
build-windows: build-windows-amd64 build-windows-arm64

# Build for Linux AMD64 (for testing)
.PHONY: build-linux-amd64
build-linux-amd64:
	@echo "Building for Linux AMD64..."
	@GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/fast-file-deletion

# Build for Linux ARM64 (for testing)
.PHONY: build-linux-arm64
build-linux-arm64:
	@echo "Building for Linux ARM64..."
	@GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/fast-file-deletion

# Build all Linux targets
.PHONY: build-linux
build-linux: build-linux-amd64 build-linux-arm64

# Build for macOS AMD64 (for testing)
.PHONY: build-darwin-amd64
build-darwin-amd64:
	@echo "Building for macOS AMD64..."
	@GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/fast-file-deletion

# Build for macOS ARM64 (for testing)
.PHONY: build-darwin-arm64
build-darwin-arm64:
	@echo "Building for macOS ARM64 (Apple Silicon)..."
	@GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/fast-file-deletion

# Build all macOS targets
.PHONY: build-darwin
build-darwin: build-darwin-amd64 build-darwin-arm64

# Build for all platforms
.PHONY: build-all
build-all: clean build-windows build-linux build-darwin
	@echo "All builds complete!"
	@ls -lh $(BUILD_DIR)

# Run tests
.PHONY: test
test:
	@echo "Running tests..."
	@go test ./... -v

# Run tests with race detection
.PHONY: test-race
test-race:
	@echo "Running tests with race detection..."
	@go test -race ./... -v

# Run tests with coverage
.PHONY: test-coverage
test-coverage:
	@echo "Running tests with coverage..."
	@go test -cover ./... -coverprofile=coverage.out
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Install locally
.PHONY: install
install:
	@echo "Installing $(BINARY_NAME)..."
	@go install $(LDFLAGS) ./cmd/fast-file-deletion

# Show help
.PHONY: help
help:
	@echo "Fast File Deletion - Build Targets"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  all                  - Clean and build Windows binaries (default)"
	@echo "  clean                - Remove build artifacts"
	@echo "  build-windows        - Build all Windows binaries"
	@echo "  build-windows-amd64  - Build Windows AMD64 binary"
	@echo "  build-windows-arm64  - Build Windows ARM64 binary"
	@echo "  build-linux          - Build all Linux binaries (for testing)"
	@echo "  build-linux-amd64    - Build Linux AMD64 binary"
	@echo "  build-linux-arm64    - Build Linux ARM64 binary"
	@echo "  build-darwin         - Build all macOS binaries (for testing)"
	@echo "  build-darwin-amd64   - Build macOS AMD64 binary"
	@echo "  build-darwin-arm64   - Build macOS ARM64 binary (Apple Silicon)"
	@echo "  build-all            - Build for all platforms"
	@echo "  test                 - Run all tests"
	@echo "  test-race            - Run tests with race detection"
	@echo "  test-coverage        - Run tests with coverage report"
	@echo "  install              - Install binary locally"
	@echo "  help                 - Show this help message"
