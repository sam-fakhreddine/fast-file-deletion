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

# Run tests (quick mode, no verbose output to prevent crashes)
.PHONY: test
test:
	@echo "Running tests in quick mode..."
	@go test ./...

# Run tests in quick mode (explicit)
.PHONY: test-quick
test-quick:
	@echo "Running tests in quick mode..."
	@TEST_INTENSITY=quick go test ./...

# Run tests in thorough mode
.PHONY: test-thorough
test-thorough:
	@echo "Running tests in thorough mode..."
	@TEST_INTENSITY=thorough go test ./...

# Run stress tests only
.PHONY: test-stress
test-stress:
	@echo "Running stress tests..."
	@go test -tags=stress ./...

# Run all tests (thorough mode + stress tests)
.PHONY: test-all
test-all:
	@echo "Running all tests (thorough + stress)..."
	@TEST_INTENSITY=thorough go test -tags=stress ./...

# Run tests with race detection
.PHONY: test-race
test-race:
	@echo "Running tests with race detection..."
	@go test -race ./...

# Run tests with coverage
.PHONY: test-coverage
test-coverage:
	@echo "Running tests with coverage..."
	@go test -cover ./... -coverprofile=coverage.out
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run benchmarks (Windows only)
.PHONY: benchmark
benchmark:
	@echo "Running Go benchmarks..."
	@go test -bench=. -benchmem ./internal/backend

# Verify Windows-specific code compiles
.PHONY: verify-windows
verify-windows:
	@echo "Verifying Windows-specific code compiles..."
	@GOOS=windows GOARCH=amd64 go build -o /dev/null ./cmd/fast-file-deletion
	@echo "✓ Windows AMD64 code compiles successfully"
	@GOOS=windows GOARCH=arm64 go build -o /dev/null ./cmd/fast-file-deletion
	@echo "✓ Windows ARM64 code compiles successfully"

# Verify generic code compiles on all platforms
.PHONY: verify-all
verify-all:
	@echo "Verifying code compiles on all platforms..."
	@GOOS=windows GOARCH=amd64 go build -o /dev/null ./cmd/fast-file-deletion
	@echo "✓ Windows AMD64"
	@GOOS=linux GOARCH=amd64 go build -o /dev/null ./cmd/fast-file-deletion
	@echo "✓ Linux AMD64"
	@GOOS=darwin GOARCH=amd64 go build -o /dev/null ./cmd/fast-file-deletion
	@echo "✓ macOS AMD64"
	@GOOS=darwin GOARCH=arm64 go build -o /dev/null ./cmd/fast-file-deletion
	@echo "✓ macOS ARM64"
	@echo "All platforms compile successfully!"

# Install locally
.PHONY: install
install:
	@echo "Installing $(BINARY_NAME)..."
	@go install $(LDFLAGS) ./cmd/fast-file-deletion

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# GUI TARGETS (Wails v3)
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

# Install Wails CLI (required for GUI development)
.PHONY: gui-install-wails
gui-install-wails:
	@echo "Installing Wails v3 CLI..."
	@go install github.com/wailsapp/wails/v3/cmd/wails3@latest
	@echo "✓ Wails v3 installed at ~/go/bin/wails3"

# Install frontend dependencies
.PHONY: gui-deps
gui-deps:
	@echo "Installing frontend dependencies..."
	@cd frontend && npm install
	@echo "✓ Frontend dependencies installed"

# Generate Wails bindings (TypeScript interfaces for Go backend)
.PHONY: gui-bindings
gui-bindings:
	@echo "Generating Wails bindings..."
	@cd cmd/ffd-gui && ~/go/bin/wails3 generate bindings
	@cp -r cmd/ffd-gui/frontend/bindings frontend/
	@echo "✓ Bindings generated and copied to frontend/"

# Development mode with hot reload
.PHONY: gui-dev
gui-dev:
	@echo "Starting GUI in development mode..."
	@cd cmd/ffd-gui && ~/go/bin/wails3 dev

# Build frontend only
.PHONY: gui-build-frontend
gui-build-frontend:
	@echo "Building frontend..."
	@cd frontend && npm run build
	@echo "✓ Frontend built to frontend/dist/"

# Build GUI for current platform (NOTE: Must run on target platform)
.PHONY: gui-build
gui-build: gui-build-frontend
	@echo "Building GUI for current platform..."
	@echo "NOTE: GUI apps with native WebView cannot be cross-compiled."
	@echo "      Run this on Windows to build ffd-gui.exe"
	@echo ""
	@mkdir -p $(BUILD_DIR)
	@rm -rf cmd/ffd-gui/build
	@cd cmd/ffd-gui && ~/go/bin/wails3 build
	@if [ -f cmd/ffd-gui/build/bin/ffd-gui.exe ]; then \
		cp cmd/ffd-gui/build/bin/ffd-gui.exe $(BUILD_DIR)/ffd-gui.exe; \
		echo "✓ GUI binary built: $(BUILD_DIR)/ffd-gui.exe"; \
		ls -lh $(BUILD_DIR)/ffd-gui.exe; \
	elif [ -f cmd/ffd-gui/build/bin/ffd-gui ]; then \
		cp cmd/ffd-gui/build/bin/ffd-gui $(BUILD_DIR)/ffd-gui; \
		echo "✓ GUI binary built: $(BUILD_DIR)/ffd-gui"; \
		ls -lh $(BUILD_DIR)/ffd-gui; \
	else \
		echo "ERROR: Build output not found"; \
		exit 1; \
	fi

# Clean GUI build artifacts
.PHONY: gui-clean
gui-clean:
	@echo "Cleaning GUI build artifacts..."
	@rm -rf frontend/dist
	@rm -rf frontend/bindings
	@rm -rf cmd/ffd-gui/frontend/bindings
	@rm -rf cmd/ffd-gui/build
	@rm -f $(BUILD_DIR)/ffd-gui.exe
	@rm -f $(BUILD_DIR)/ffd-gui
	@echo "✓ GUI artifacts cleaned"

# Full GUI setup (first-time setup)
.PHONY: gui-setup
gui-setup: gui-install-wails gui-deps gui-bindings
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo "✓ GUI setup complete!"
	@echo ""
	@echo "Next steps:"
	@echo "  make gui-dev        # Start development server"
	@echo "  make gui-build      # Build production binary for Windows"
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Show help
.PHONY: help
help:
	@echo "╔════════════════════════════════════════════════════════════════╗"
	@echo "║         Fast File Deletion - Build & Test Targets             ║"
	@echo "╚════════════════════════════════════════════════════════════════╝"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo "BUILD TARGETS"
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo ""
	@echo "  all                  Clean and build Windows binaries (default)"
	@echo "  clean                Remove build artifacts from bin/ directory"
	@echo ""
	@echo "  Windows Builds:"
	@echo "    build-windows        Build all Windows binaries (AMD64 + ARM64)"
	@echo "    build-windows-amd64  Build Windows AMD64 binary only"
	@echo "    build-windows-arm64  Build Windows ARM64 binary only"
	@echo ""
	@echo "  Linux Builds (for testing):"
	@echo "    build-linux          Build all Linux binaries (AMD64 + ARM64)"
	@echo "    build-linux-amd64    Build Linux AMD64 binary only"
	@echo "    build-linux-arm64    Build Linux ARM64 binary only"
	@echo ""
	@echo "  macOS Builds (for testing):"
	@echo "    build-darwin         Build all macOS binaries (AMD64 + ARM64)"
	@echo "    build-darwin-amd64   Build macOS AMD64 binary only"
	@echo "    build-darwin-arm64   Build macOS ARM64 binary (Apple Silicon)"
	@echo ""
	@echo "  Cross-Platform:"
	@echo "    build-all            Build for all platforms (Windows, Linux, macOS)"
	@echo ""
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo "TEST TARGETS"
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo ""
	@echo "  Development Workflows (Fast):"
	@echo "    test                 Run fast tests in quick mode (default, ~30s)"
	@echo "                         → Use during active development for rapid feedback"
	@echo "                         → 10-20 iterations per property test"
	@echo "                         → Small data sets (100 files, depth 3)"
	@echo ""
	@echo "    test-quick           Explicitly run tests in quick mode (same as 'test')"
	@echo "                         → Useful when TEST_INTENSITY is set elsewhere"
	@echo ""
	@echo "  CI/CD Workflows (Comprehensive):"
	@echo "    test-thorough        Run comprehensive tests in thorough mode (~5min)"
	@echo "                         → Use in CI/CD pipelines before merging"
	@echo "                         → 100-200 iterations per property test"
	@echo "                         → Larger data sets (1000 files, depth 5)"
	@echo ""
	@echo "    test-coverage        Generate HTML coverage report (coverage.html)"
	@echo "                         → Creates coverage.out and coverage.html files"
	@echo "                         → Use to validate test coverage before releases"
	@echo ""
	@echo "  Performance Testing (Long-Running):"
	@echo "    test-stress          Run stress tests only (10+ minutes)"
	@echo "                         → Tests with large data sets (10,000+ files)"
	@echo "                         → Deep directory structures (depth 10+)"
	@echo "                         → Use before major releases"
	@echo ""
	@echo "    test-all             Run all tests: thorough + stress (10+ minutes)"
	@echo "                         → Most comprehensive validation"
	@echo "                         → Use before creating releases"
	@echo ""
	@echo "  Debugging & Analysis:"
	@echo "    test-race            Run tests with race detector"
	@echo "                         → Detects data races in concurrent code"
	@echo "                         → Use when debugging concurrency issues"
	@echo ""
	@echo "    benchmark            Run Go benchmarks (internal/backend)"
	@echo "                         → Measures performance of deletion methods"
	@echo "                         → Use for performance regression testing"
	@echo ""
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo "BUILD VERIFICATION TARGETS"
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo ""
	@echo "  verify-windows       Verify Windows-specific code compiles (AMD64 + ARM64)"
	@echo "                       → Ensures Windows build tags are correct"
	@echo "                       → Use before committing Windows-specific changes"
	@echo ""
	@echo "  verify-all           Verify code compiles on all platforms"
	@echo "                       → Tests Windows, Linux, macOS (AMD64 + ARM64)"
	@echo "                       → Use before creating releases"
	@echo ""
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo "GUI TARGETS (Wails v3)"
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo ""
	@echo "  First-Time Setup:"
	@echo "    gui-setup            Complete GUI setup (install Wails, deps, bindings)"
	@echo "    gui-install-wails    Install Wails v3 CLI tool"
	@echo "    gui-deps             Install frontend dependencies (npm install)"
	@echo "    gui-bindings         Generate TypeScript bindings for Go backend"
	@echo ""
	@echo "  Development:"
	@echo "    gui-dev              Start development server with hot reload"
	@echo "    gui-build-frontend   Build frontend only (dist/)"
	@echo ""
	@echo "  Production Build:"
	@echo "    gui-build            Build GUI for current platform (run on Windows for .exe)"
	@echo "                         Note: GUI apps cannot be cross-compiled"
	@echo ""
	@echo "  Maintenance:"
	@echo "    gui-clean            Remove GUI build artifacts"
	@echo ""
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo "OTHER TARGETS"
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo ""
	@echo "  install              Install CLI binary locally (go install)"
	@echo "  help                 Show this help message"
	@echo ""
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo "TEST MODES EXPLAINED"
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo ""
	@echo "  Quick Mode (default):"
	@echo "    • Execution time: ~30 seconds"
	@echo "    • Iterations: 10-20 per property test"
	@echo "    • Data sets: 100 files, depth 3"
	@echo "    • Use case: Local development, rapid iteration"
	@echo ""
	@echo "  Thorough Mode:"
	@echo "    • Execution time: ~5 minutes"
	@echo "    • Iterations: 100-200 per property test"
	@echo "    • Data sets: 1,000 files, depth 5"
	@echo "    • Use case: CI/CD pipelines, pre-release validation"
	@echo ""
	@echo "  Stress Tests:"
	@echo "    • Execution time: 10+ minutes"
	@echo "    • Iterations: 100-200 per property test"
	@echo "    • Data sets: 10,000+ files, depth 10+"
	@echo "    • Use case: Performance testing, release validation"
	@echo ""
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo "ENVIRONMENT VARIABLES"
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo ""
	@echo "  TEST_INTENSITY=quick|thorough"
	@echo "    Control test thoroughness (default: quick)"
	@echo "    Examples:"
	@echo "      TEST_INTENSITY=quick go test ./..."
	@echo "      TEST_INTENSITY=thorough go test ./..."
	@echo ""
	@echo "  TEST_QUICK=1"
	@echo "    Force quick mode regardless of TEST_INTENSITY"
	@echo "    Example:"
	@echo "      TEST_QUICK=1 go test ./..."
	@echo ""
	@echo "  VERBOSE_TESTS=1"
	@echo "    Enable verbose test output (use sparingly)"
	@echo "    Example:"
	@echo "      VERBOSE_TESTS=1 go test ./internal/logger"
	@echo ""
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo "RECOMMENDED WORKFLOWS"
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo ""
	@echo "  During development:"
	@echo "    make test                    # Quick validation (~30s)"
	@echo ""
	@echo "  Before committing:"
	@echo "    make test                    # Quick validation"
	@echo ""
	@echo "  Before creating PR:"
	@echo "    make test-thorough           # Comprehensive validation (~5min)"
	@echo "    make test-coverage           # Check coverage"
	@echo ""
	@echo "  Before release:"
	@echo "    make test-all                # All tests including stress (10+ min)"
	@echo ""
	@echo "  Debugging specific test:"
	@echo "    VERBOSE_TESTS=1 go test ./internal/engine -run TestName"
	@echo "    make test-race               # Check for race conditions"
	@echo ""
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo ""
	@echo "For more details, see README.md or run: go test ./... -h"
	@echo ""
