# Test Utilities Package

This package provides utilities for configuring and running tests with different intensity levels and resource constraints.

## Overview

The `testutil` package centralizes test configuration and provides utilities for:
- Configurable test intensity (quick vs thorough mode)
- Test fixture generation with resource limits
- Rapid framework integration
- Test timeout mechanisms
- Efficient test cleanup

## Files

### config.go
Provides test configuration system that reads environment variables:
- `TestConfig` struct with intensity, iteration counts, file limits, timeouts
- `GetTestConfig()` - reads environment variables and returns configuration
- `TestIntensity` enum (Quick/Thorough)
- Environment variables: `TEST_INTENSITY`, `TEST_QUICK`, `VERBOSE_TESTS`

### fixtures.go
Generates test data (files and directories) respecting configuration limits:
- `CreateTestDirectory()` - creates temp directory with files
- `GenerateTestFiles()` - creates files with random content
- `GenerateTestTree()` - creates nested directory structures
- `CountFiles()` - counts files in a directory
- `GetMaxDepth()` - calculates maximum directory depth

### rapid.go
Integrates with the rapid property-based testing framework:
- `RapidCheck()` - wrapper for rapid.Check with configuration
- `GetRapidCheckConfig()` - configures rapid iteration counts
- `RapidFileCountGenerator()` - generates file counts within limits
- `RapidFileSizeGenerator()` - generates file sizes within limits
- `RapidDepthGenerator()` - generates directory depths within limits
- `RapidConfigGenerator()` - generates random test configurations

### timeout.go
Implements test timeout mechanisms:
- `GetTestTimeout()` - returns timeout for current intensity
- `WithTimeout()` - executes test function with timeout
- `WithTimeoutT()` - convenience wrapper that fails test on timeout
- `CheckDeadline()` - checks if test exceeded deadline
- `TimeoutContext()` - creates context with test timeout

### cleanup.go
Provides efficient test cleanup utilities:
- `CleanupTestDir()` - removes test directory
- `RegisterCleanup()` - registers cleanup with t.Cleanup()
- `CleanupWithTimeout()` - cleanup with timeout
- `EnsureCleanup()` - aggressive cleanup with retries
- `CountFilesRecursive()` - counts files recursively
- `VerifyCleanup()` - verifies directory was cleaned up

## Usage

### Basic Configuration

```go
import "github.com/yourusername/fast-file-deletion/internal/testutil"

func TestExample(t *testing.T) {
    config := testutil.GetTestConfig()
    // config.Intensity, config.MaxFiles, etc.
}
```

### Creating Test Fixtures

```go
func TestWithFixtures(t *testing.T) {
    config := testutil.GetTestConfig()
    dir := testutil.CreateTestDirectory(t, config)
    // dir contains files according to config limits
    // Automatically cleaned up by t.TempDir()
}
```

### Property-Based Testing

```go
func TestProperty(t *testing.T) {
    testutil.RapidCheck(t, func(rt *rapid.T) {
        // Property test logic
        // Iteration count controlled by TEST_INTENSITY
    })
}
```

### Test Timeouts

```go
func TestWithTimeout(t *testing.T) {
    testutil.WithTimeoutT(t, func(ctx context.Context) {
        // Test logic that respects context timeout
    })
}
```

## Environment Variables

- `TEST_INTENSITY=quick|thorough` - Sets test intensity level (default: quick)
- `TEST_QUICK=1|true` - Forces quick mode (overrides TEST_INTENSITY)
- `VERBOSE_TESTS=1|true` - Enables verbose test output

## Configuration Defaults

### Quick Mode (default)
- Iterations: 10
- Max Files: 100
- Max File Size: 1KB
- Max Depth: 3
- Timeout: 30 seconds

### Thorough Mode
- Iterations: 100
- Max Files: 1000
- Max File Size: 10KB
- Max Depth: 5
- Timeout: 5 minutes

## Testing

Run tests for this package:
```bash
go test ./internal/testutil
```

Run with verbose output:
```bash
VERBOSE_TESTS=1 go test ./internal/testutil -v
```
