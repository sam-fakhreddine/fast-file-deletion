#!/bin/bash
# Fast File Deletion - Build Script
# Requirements: 9.2, 9.3

set -e

# Configuration
BINARY_NAME="fast-file-deletion"
BUILD_DIR="bin"
VERSION="${VERSION:-dev}"
LDFLAGS="-s -w -X main.version=${VERSION}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Helper functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Clean build artifacts
clean() {
    log_info "Cleaning build artifacts..."
    rm -rf "${BUILD_DIR}"
    mkdir -p "${BUILD_DIR}"
}

# Build for a specific platform
build_platform() {
    local os=$1
    local arch=$2
    local ext=$3
    
    local output="${BUILD_DIR}/${BINARY_NAME}-${os}-${arch}${ext}"
    
    log_info "Building for ${os}/${arch}..."
    GOOS="${os}" GOARCH="${arch}" go build -ldflags="${LDFLAGS}" -o "${output}" ./cmd/fast-file-deletion
    
    if [ $? -eq 0 ]; then
        local size=$(du -h "${output}" | cut -f1)
        log_info "✓ Built ${output} (${size})"
    else
        log_error "✗ Failed to build for ${os}/${arch}"
        return 1
    fi
}

# Build Windows binaries
build_windows() {
    log_info "Building Windows binaries..."
    build_platform "windows" "amd64" ".exe"
    build_platform "windows" "arm64" ".exe"
}

# Build Linux binaries
build_linux() {
    log_info "Building Linux binaries..."
    build_platform "linux" "amd64" ""
    build_platform "linux" "arm64" ""
}

# Build macOS binaries
build_darwin() {
    log_info "Building macOS binaries..."
    build_platform "darwin" "amd64" ""
    build_platform "darwin" "arm64" ""
}

# Build all platforms
build_all() {
    clean
    build_windows
    build_linux
    build_darwin
    log_info "All builds complete!"
    ls -lh "${BUILD_DIR}"
}

# Run tests
run_tests() {
    log_info "Running tests..."
    go test ./... -v
}

# Run tests with race detection
run_tests_race() {
    log_info "Running tests with race detection..."
    go test -race ./... -v
}

# Run tests with coverage
run_tests_coverage() {
    log_info "Running tests with coverage..."
    go test -cover ./... -coverprofile=coverage.out
    go tool cover -html=coverage.out -o coverage.html
    log_info "Coverage report generated: coverage.html"
}

# Install locally
install_local() {
    log_info "Installing ${BINARY_NAME}..."
    go install -ldflags="${LDFLAGS}" ./cmd/fast-file-deletion
    log_info "✓ Installed successfully"
}

# Show help
show_help() {
    cat << EOF
Fast File Deletion - Build Script

Usage: ./build.sh [command]

Commands:
  clean              - Remove build artifacts
  windows            - Build all Windows binaries
  linux              - Build all Linux binaries (for testing)
  darwin             - Build all macOS binaries (for testing)
  all                - Build for all platforms
  test               - Run all tests
  test-race          - Run tests with race detection
  test-coverage      - Run tests with coverage report
  install            - Install binary locally
  help               - Show this help message

Examples:
  ./build.sh windows           # Build Windows binaries
  ./build.sh all               # Build for all platforms
  VERSION=1.0.0 ./build.sh all # Build with version 1.0.0

EOF
}

# Main script logic
main() {
    local command="${1:-windows}"
    
    case "${command}" in
        clean)
            clean
            ;;
        windows)
            clean
            build_windows
            ;;
        linux)
            clean
            build_linux
            ;;
        darwin)
            clean
            build_darwin
            ;;
        all)
            build_all
            ;;
        test)
            run_tests
            ;;
        test-race)
            run_tests_race
            ;;
        test-coverage)
            run_tests_coverage
            ;;
        install)
            install_local
            ;;
        help|--help|-h)
            show_help
            ;;
        *)
            log_error "Unknown command: ${command}"
            show_help
            exit 1
            ;;
    esac
}

# Run main function
main "$@"
