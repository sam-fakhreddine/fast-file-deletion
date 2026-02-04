# Design Document: Test Suite Optimization

## Overview

This design optimizes the fast-file-deletion test suite to address performance issues while maintaining comprehensive correctness validation. The solution introduces configurable test intensity levels, reduces resource consumption through smaller test data sets, implements execution timeouts, and separates long-running stress tests from fast unit tests.

The key insight is that property-based testing doesn't require massive data sets to find bugs - most issues are discovered in the first 10-20 iterations. By making iteration counts and data sizes configurable, we enable fast feedback during development while preserving thorough validation in CI environments.

## Architecture

### Test Configuration System

The test suite will use a centralized configuration system that reads environment variables and provides test parameters to all test functions:

```go
// internal/testutil/config.go
package testutil

type TestConfig struct {
    Intensity        TestIntensity
    IterationCount   int
    MaxFiles         int
    MaxFileSize      int64
    MaxDepth         int
    Timeout          time.Duration
    VerboseOutput    bool
}

type TestIntensity int

const (
    IntensityQuick TestIntensity = iota
    IntensityThorough
)

func GetTestConfig() TestConfig {
    // Read environment variables and return configuration
}
```

### Test Organization

Tests will be organized into three categories:

1. **Fast Unit Tests** (`*_test.go`): Specific examples and edge cases, always run
2. **Fast Property Tests** (`*_test.go`): Property-based tests with quick mode defaults, always run
3. **Stress Tests** (`*_stress_test.go`): Long-running property tests with large data sets, run only with `stress` build tag

### File Structure

```
internal/
├── testutil/
│   ├── config.go           # Test configuration system
│   ├── fixtures.go         # Optimized test data generation
│   └── cleanup.go          # Test cleanup utilities
├── engine/
│   ├── engine_test.go      # Fast unit and property tests
│   └── engine_stress_test.go  # Long-running stress tests (new)
└── backend/
    ├── backend_test.go
    └── benchmark_test.go
```

## Components and Interfaces

### 1. Test Configuration Component

**Purpose**: Centralize test configuration and provide consistent parameters across all tests.

**Interface**:
```go
package testutil

// GetTestConfig returns the current test configuration based on environment variables
func GetTestConfig() TestConfig

// GetRapidOptions returns rapid.Check options configured for current test intensity
func GetRapidOptions(t *testing.T) []rapid.Option

// WithTimeout wraps a test function with a timeout based on current configuration
func WithTimeout(t *testing.T, fn func()) error
```

**Implementation Details**:
- Reads `TEST_INTENSITY` environment variable ("quick" or "thorough")
- Reads `TEST_QUICK` environment variable ("1" or "true" forces quick mode)
- Reads `VERBOSE_TESTS` environment variable to enable detailed logging
- Defaults to quick mode if no environment variables are set
- Logs the active configuration at package initialization

### 2. Test Fixture Generator

**Purpose**: Generate optimized test data (files, directories) based on current test intensity.

**Interface**:
```go
package testutil

// CreateTestDirectory creates a temporary directory with generated files
// Returns the directory path and cleanup function
func CreateTestDirectory(t *testing.T, config TestConfig) (string, func())

// GenerateTestFiles creates files in the given directory according to config
func GenerateTestFiles(dir string, config TestConfig) error

// GenerateTestTree creates a nested directory structure with files
func GenerateTestTree(dir string, depth int, filesPerDir int, config TestConfig) error
```

**Implementation Details**:
- Uses `t.TempDir()` for automatic cleanup
- Creates files with minimal content (random bytes up to MaxFileSize)
- Uses buffered I/O for efficient file creation
- Respects MaxFiles, MaxFileSize, and MaxDepth from configuration
- Provides rapid generators that respect configuration limits

### 3. Rapid Integration

**Purpose**: Configure rapid framework to use appropriate iteration counts and timeouts.

**Interface**:
```go
package testutil

// RapidCheck wraps rapid.Check with configured options
func RapidCheck(t *testing.T, fn interface{})

// RapidGenerator returns a rapid generator for test directories
func RapidDirectoryGenerator(config TestConfig) *rapid.Generator[string]

// RapidFileCountGenerator returns a generator for file counts within config limits
func RapidFileCountGenerator(config TestConfig) *rapid.Generator[int]
```

**Implementation Details**:
- Uses `rapid.MinCount()` to set iteration counts based on test intensity
- Quick mode: 10-20 iterations
- Thorough mode: 100-200 iterations
- Integrates with timeout mechanism
- Provides custom generators that respect configuration limits

### 4. Timeout Mechanism

**Purpose**: Prevent tests from running indefinitely by enforcing execution time limits.

**Interface**:
```go
package testutil

// WithTimeout executes a test function with a timeout
func WithTimeout(t *testing.T, timeout time.Duration, fn func()) error

// GetTestTimeout returns the appropriate timeout for current test intensity
func GetTestTimeout(config TestConfig) time.Duration
```

**Implementation Details**:
- Uses `context.WithTimeout` to enforce time limits
- Quick mode: 30 second timeout per test
- Thorough mode: 5 minute timeout per test
- Cleans up test resources on timeout
- Reports timeout failures with clear error messages

### 5. Test Cleanup Utilities

**Purpose**: Efficiently clean up test files and directories after test execution.

**Interface**:
```go
package testutil

// CleanupTestDir removes a test directory using the optimized deletion engine
func CleanupTestDir(dir string) error

// RegisterCleanup registers a cleanup function that uses optimized deletion
func RegisterCleanup(t *testing.T, dir string)
```

**Implementation Details**:
- Uses the project's optimized deletion engine for fast cleanup
- Handles cleanup failures gracefully (logs but doesn't fail tests)
- Integrates with `t.Cleanup()` for automatic execution

## Data Models

### TestConfig Structure

```go
type TestConfig struct {
    // Intensity level (quick or thorough)
    Intensity TestIntensity
    
    // Number of iterations for property tests
    IterationCount int
    
    // Maximum number of files to create in test cases
    MaxFiles int
    
    // Maximum size of individual test files (bytes)
    MaxFileSize int64
    
    // Maximum directory depth for nested structures
    MaxDepth int
    
    // Timeout duration for individual tests
    Timeout time.Duration
    
    // Enable verbose test output
    VerboseOutput bool
}
```

### Configuration Defaults

**Quick Mode** (default):
- IterationCount: 10
- MaxFiles: 100
- MaxFileSize: 1024 (1KB)
- MaxDepth: 3
- Timeout: 30 seconds
- VerboseOutput: false

**Thorough Mode**:
- IterationCount: 100
- MaxFiles: 1000
- MaxFileSize: 10240 (10KB)
- MaxDepth: 5
- Timeout: 5 minutes
- VerboseOutput: false

## Migration Strategy

### Phase 1: Create Test Utilities

1. Create `internal/testutil` package
2. Implement configuration system
3. Implement fixture generators
4. Implement rapid integration helpers
5. Implement timeout mechanism

### Phase 2: Refactor Existing Tests

1. Update `internal/engine/engine_test.go`:
   - Replace hardcoded iteration counts with `testutil.GetRapidOptions()`
   - Replace manual file creation with `testutil.CreateTestDirectory()`
   - Add timeouts to all property tests
   - Reduce file counts in test data generation

2. Fix or remove `TestBatchMemoryRelease`:
   - Investigate the infinite loop issue
   - If fixable: add timeout and reduce test data size
   - If not fixable: remove and document why

### Phase 3: Separate Stress Tests

1. Create `internal/engine/engine_stress_test.go`
2. Move long-running tests to stress file:
   - Tests that create >1000 files
   - Tests that run >100 iterations in quick mode
   - Tests that take >10 seconds in quick mode
3. Add `//go:build stress` tag to stress test file
4. Update Makefile with stress test targets

### Phase 4: Documentation

1. Update README with test configuration section
2. Document environment variables
3. Provide examples for local development
4. Provide examples for CI configuration
5. Document stress test execution

## Specific Test Optimizations

### TestCompleteDirectoryRemoval

**Current**: Creates large directory structures with many files
**Optimized**: 
- Quick mode: 50 files, depth 2
- Thorough mode: 500 files, depth 4
- Use `testutil.CreateTestDirectory()` for consistent generation

### TestDeletionIsolation

**Current**: Creates multiple directory structures
**Optimized**:
- Quick mode: 2 directories with 20 files each
- Thorough mode: 5 directories with 100 files each
- Reduce iteration count to 10 (quick) / 100 (thorough)

### TestErrorResilience

**Current**: Creates files and simulates errors
**Optimized**:
- Quick mode: 30 files with 5 error injections
- Thorough mode: 200 files with 20 error injections
- Use smaller file sizes (1 byte)

### TestBatchMemoryRelease

**Current**: Skipped due to infinite loop
**Options**:
1. **Fix**: Add timeout, reduce batch size to 1000 files, add loop termination condition
2. **Remove**: Delete test and rely on other memory-related tests
3. **Move to Stress**: Move to `engine_stress_test.go` with proper timeout

**Recommendation**: Fix with timeout and move to stress tests. The test validates important memory behavior but doesn't need to run on every test execution.

## Error Handling

### Timeout Handling

When a test times out:
1. Cancel the test context
2. Clean up any created test files/directories
3. Report timeout failure with elapsed time
4. Log the test configuration that was active

### Configuration Errors

When environment variables are invalid:
1. Log a warning with the invalid value
2. Fall back to default (quick mode)
3. Continue test execution

### Cleanup Failures

When test cleanup fails:
1. Log the error (don't fail the test)
2. Attempt to use system deletion as fallback
3. Report cleanup failures in test output

## Testing Strategy

### Unit Tests for Test Utilities

Create `internal/testutil/config_test.go`:
- Test environment variable parsing
- Test configuration defaults
- Test timeout calculation
- Test rapid options generation

### Validation Tests

Create `internal/testutil/fixtures_test.go`:
- Verify generated files respect MaxFiles limit
- Verify generated files respect MaxFileSize limit
- Verify generated directories respect MaxDepth limit
- Verify cleanup functions work correctly

### Integration Tests

Update existing property tests to use new utilities:
- Verify tests complete within timeout limits
- Verify tests respect iteration counts
- Verify tests generate appropriate data sizes
- Measure and document execution time improvements

### Performance Benchmarks

Create benchmarks to measure improvement:
- Benchmark test suite execution time (quick mode)
- Benchmark test suite execution time (thorough mode)
- Compare before/after optimization
- Target: 50% reduction in quick mode execution time


## Correctness Properties

*A property is a characteristic or behavior that should hold true across all valid executions of a system—essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees.*

### Property 1: File Count Limit Enforcement

*For any* test configuration (quick or thorough mode), when generating test directories, the number of files created shall never exceed the configured MaxFiles limit.

**Validates: Requirements 3.1, 3.2**

### Property 2: File Size Range Compliance

*For any* test configuration and any generated test file, the file size shall be within the configured range [1 byte, MaxFileSize].

**Validates: Requirements 3.3, 3.4, 8.4**

### Property 3: Directory Depth Limit Enforcement

*For any* test configuration and any generated directory structure, the maximum depth of nested directories shall never exceed the configured MaxDepth limit.

**Validates: Requirements 3.5, 3.6**

### Property 4: Configuration Parsing Idempotence

*For any* valid environment variable configuration, parsing the configuration multiple times shall produce identical TestConfig values.

**Validates: Requirements 1.1, 1.2, 1.3, 1.4**

### Property 5: Timeout Enforcement

*For any* test that runs longer than the configured timeout duration, the test execution shall be terminated and a timeout failure shall be reported.

**Validates: Requirements 4.1, 4.3**

### Property 6: Cleanup After Timeout

*For any* test that times out, all test files and directories created by that test shall be removed during cleanup.

**Validates: Requirements 4.4**

## Testing Strategy

This feature uses a dual testing approach combining unit tests and property-based tests to ensure comprehensive validation.

### Unit Tests

Unit tests will validate specific examples, edge cases, and integration points:

**Configuration Tests** (`internal/testutil/config_test.go`):
- Test environment variable parsing with specific values ("quick", "thorough", "1", "true")
- Test default behavior when no environment variables are set
- Test override behavior (TEST_QUICK overrides TEST_INTENSITY)
- Test invalid environment variable values fall back to defaults
- Test configuration logging output format

**Fixture Tests** (`internal/testutil/fixtures_test.go`):
- Test file creation with specific counts (0 files, 1 file, 100 files)
- Test directory creation with specific depths (0 depth, 1 depth, 5 depth)
- Test cleanup function removes all created files
- Test cleanup handles missing directories gracefully

**Timeout Tests** (`internal/testutil/timeout_test.go`):
- Test timeout triggers for deliberately slow tests
- Test timeout cleanup removes test files
- Test timeout error messages include elapsed time
- Test tests complete successfully within timeout limits

**Integration Tests** (`internal/engine/engine_test.go`):
- Test rapid integration with MinCount option
- Test property tests respect iteration counts
- Test property tests complete within timeout limits
- Test no tests are skipped after optimization

**Build Tag Tests**:
- Test stress tests are excluded without build tag
- Test stress tests are included with `stress` build tag
- Test non-stress tests maintain >80% code coverage

### Property-Based Tests

Property tests will validate universal correctness properties using the rapid framework:

**Property Test Configuration**:
- Minimum 100 iterations per property test (in thorough mode)
- Each test tagged with: `Feature: test-suite-optimization, Property N: [property text]`
- Tests use `testutil.RapidCheck()` wrapper for consistent configuration

**Property Tests** (`internal/testutil/fixtures_property_test.go`):

1. **Property 1: File Count Limit Enforcement**
   - Generate random test configurations
   - Generate test directories using those configurations
   - Verify file count ≤ MaxFiles for all generated directories
   - Tag: `Feature: test-suite-optimization, Property 1: File count limit enforcement`

2. **Property 2: File Size Range Compliance**
   - Generate random test configurations
   - Generate test files using those configurations
   - Verify all file sizes are in range [1, MaxFileSize]
   - Tag: `Feature: test-suite-optimization, Property 2: File size range compliance`

3. **Property 3: Directory Depth Limit Enforcement**
   - Generate random test configurations
   - Generate directory structures using those configurations
   - Verify maximum depth ≤ MaxDepth for all structures
   - Tag: `Feature: test-suite-optimization, Property 3: Directory depth limit enforcement`

4. **Property 4: Configuration Parsing Idempotence**
   - Generate random environment variable combinations
   - Parse configuration twice with same environment
   - Verify both parsed configs are identical
   - Tag: `Feature: test-suite-optimization, Property 4: Configuration parsing idempotence`

5. **Property 5: Timeout Enforcement**
   - Generate random timeout durations
   - Create tests that exceed those durations
   - Verify tests are terminated and report timeout failures
   - Tag: `Feature: test-suite-optimization, Property 5: Timeout enforcement`

6. **Property 6: Cleanup After Timeout**
   - Generate random test directories
   - Trigger timeouts during test execution
   - Verify all test files are removed after timeout
   - Tag: `Feature: test-suite-optimization, Property 6: Cleanup after timeout`

### Performance Validation

**Benchmark Tests** (`internal/testutil/benchmark_test.go`):
- Benchmark test suite execution time in quick mode
- Benchmark test suite execution time in thorough mode
- Benchmark file generation performance
- Benchmark cleanup performance

**Success Criteria**:
- Quick mode test suite completes in <30 seconds
- Thorough mode test suite completes in <5 minutes
- 50% reduction in quick mode execution time vs. current implementation
- No increase in test failure rate

### Test Execution Examples

**Local Development (Quick Mode)**:
```bash
# Run all fast tests (default)
go test ./...

# Run with explicit quick mode
TEST_INTENSITY=quick go test ./...

# Run with verbose output for debugging
VERBOSE_TESTS=1 go test ./... -v
```

**CI Environment (Thorough Mode)**:
```bash
# Run thorough tests
TEST_INTENSITY=thorough go test ./...

# Run with coverage
TEST_INTENSITY=thorough go test ./... -cover
```

**Stress Tests**:
```bash
# Run stress tests separately
go test -tags=stress ./...

# Run stress tests in thorough mode
TEST_INTENSITY=thorough go test -tags=stress ./...
```

### Makefile Integration

Update Makefile with new test targets:

```makefile
# Fast tests for local development (default)
test:
	go test ./...

# Thorough tests for CI
test-thorough:
	TEST_INTENSITY=thorough go test ./...

# Stress tests
test-stress:
	go test -tags=stress ./...

# All tests (fast + stress)
test-all:
	TEST_INTENSITY=thorough go test -tags=stress ./...

# Tests with coverage
test-coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
```
