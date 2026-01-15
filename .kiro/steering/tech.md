# Technology Stack

## Language & Runtime

- **Go 1.25.5**: Primary language for performance and cross-platform support
- **Build system**: Standard Go toolchain + Make + GoReleaser

## Key Dependencies

- `golang.org/x/sys/windows`: Direct Windows API access for optimized deletion
- `pgregory.net/rapid v1.2.0`: Property-based testing framework

## Build System

### Common Commands

```bash
# Build for Windows (primary target)
make build-windows          # AMD64 + ARM64
make build-windows-amd64    # Windows AMD64 only

# Build for all platforms
make build-all              # Windows, Linux, macOS (all architectures)

# Testing
make test                   # Run all tests
make test-race              # Run with race detection
make test-coverage          # Generate coverage report (coverage.html)

# Development
make install                # Install locally
make clean                  # Remove build artifacts

# Build output location
bin/                        # All compiled binaries
```

### Build Configuration

- **Binary name**: fast-file-deletion
- **Build flags**: `-ldflags="-s -w"` for smaller binaries
- **Cross-compilation**: Supports Windows, Linux, macOS (AMD64 + ARM64)

## Testing Strategy

### Dual Testing Approach

1. **Unit tests**: Specific scenarios and edge cases (`*_test.go` files)
2. **Property-based tests**: Universal correctness properties using Rapid framework
   - Test data stored in `testdata/rapid/` directories
   - Failure cases preserved as `.fail` files for debugging

### Test Execution

```bash
# Run all tests
go test ./...

# Run specific package tests
go test ./internal/engine -v
go test ./internal/scanner -v

# Run with race detection
go test -race ./...

# Generate coverage
go test -cover ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## Platform-Specific Code

Uses Go build tags for platform-specific implementations:

- `factory_windows.go`: Windows-specific backend (Win32 API)
- `factory_generic.go`: Generic backend (standard Go file operations)
- `backend_windows_test.go`: Windows-specific tests
- `backend_generic_test.go`: Generic platform tests

## Release Process

- **GoReleaser**: Automated multi-platform builds and releases
- **Configuration**: `.goreleaser.yml`
- **Artifacts**: Binaries, checksums, archives (tar.gz, zip)
