# Implementation Plan: Test Suite Optimization

## Overview

This implementation plan optimizes the fast-file-deletion test suite by introducing configurable test intensity levels, reducing resource consumption, implementing execution timeouts, and separating long-running stress tests. The work is organized into phases that build incrementally, with each phase adding new capabilities while maintaining existing test coverage.

**Current Status**: No testutil package exists yet. All existing tests use hardcoded values and generate massive output. The test suite needs complete optimization from scratch.

## Tasks

- [x] 1. Create test utilities package structure
  - Create `internal/testutil/` directory
  - Create `config.go` for test configuration
  - Create `fixtures.go` for test data generation
  - Create `rapid.go` for rapid framework integration
  - Create `timeout.go` for timeout mechanism
  - Create `cleanup.go` for cleanup utilities
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5_

- [ ] 2. Implement test configuration system
  - [x] 2.1 Create TestConfig struct and TestIntensity enum in config.go
    - Define TestConfig with all fields (Intensity, IterationCount, MaxFiles, MaxFileSize, MaxDepth, Timeout, VerboseOutput)
    - Define TestIntensity enum (IntensityQuick, IntensityThorough)
    - _Requirements: 1.1, 1.2, 1.3_

  - [x] 2.2 Implement GetTestConfig() function in config.go
    - Read TEST_INTENSITY environment variable
    - Read TEST_QUICK environment variable for override
    - Read VERBOSE_TESTS environment variable
    - Apply defaults (quick mode if not set)
    - Return populated TestConfig
    - _Requirements: 1.1, 1.2, 1.3, 1.4_

  - [x] 2.3 Add configuration logging in config.go
    - Log active test intensity mode at package init
    - Log configuration values when VERBOSE_TESTS is enabled
    - _Requirements: 1.5_

  - [x] 2.4 Write unit tests in config_test.go
    - Test "quick" mode parsing
    - Test "thorough" mode parsing
    - Test default behavior (no env vars)
    - Test TEST_QUICK override behavior
    - Test invalid values fall back to defaults
    - _Requirements: 1.1, 1.2, 1.3, 1.4_

  - [x] 2.5 Write property test for configuration parsing in config_test.go
    - **Property 4: Configuration Parsing Idempotence**
    - **Validates: Requirements 1.1, 1.2, 1.3, 1.4**

- [ ] 3. Implement rapid framework integration
  - [x] 3.1 Create GetRapidOptions() function in rapid.go
    - Return rapid.Check options with MinCount based on test intensity
    - Quick mode: MinCount(10)
    - Thorough mode: MinCount(100)
    - _Requirements: 2.1, 2.2, 2.5_

  - [x] 3.2 Create RapidCheck() wrapper function in rapid.go
    - Wrap rapid.Check with configured options
    - Add iteration count reporting (only in verbose mode)
    - Integrate with timeout mechanism
    - _Requirements: 2.1, 2.2, 2.3, 2.5_

  - [x] 3.3 Write unit tests in rapid_test.go
    - Test MinCount option is set correctly
    - Test iteration count reporting
    - Test integration with timeout
    - _Requirements: 2.1, 2.2, 2.3, 2.5_

- [ ] 4. Implement timeout mechanism
  - [x] 4.1 Create GetTestTimeout() function in timeout.go
    - Return 30 seconds for quick mode
    - Return 5 minutes for thorough mode
    - _Requirements: 4.1, 4.2_

  - [x] 4.2 Create WithTimeout() function in timeout.go
    - Use context.WithTimeout to enforce time limits
    - Execute test function within timeout context
    - Return timeout error if exceeded
    - _Requirements: 4.1, 4.2, 4.3_

  - [x] 4.3 Write unit tests in timeout_test.go
    - Test timeout triggers for slow tests
    - Test timeout error reporting
    - Test tests complete within timeout
    - _Requirements: 4.1, 4.3_

  - [x] 4.4 Write property test for timeout enforcement in timeout_test.go
    - **Property 5: Timeout Enforcement**
    - **Validates: Requirements 4.1, 4.3**

- [ ] 5. Implement test fixture generators
  - [x] 5.1 Create CreateTestDirectory() function in fixtures.go
    - Use t.TempDir() for automatic cleanup
    - Generate files based on TestConfig
    - Return directory path
    - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5, 3.6_

  - [x] 5.2 Create GenerateTestFiles() function in fixtures.go
    - Create files with random content up to MaxFileSize
    - Respect MaxFiles limit
    - Use buffered I/O for efficiency
    - _Requirements: 3.1, 3.2, 3.3, 3.4, 8.1, 8.4_

  - [x] 5.3 Create GenerateTestTree() function in fixtures.go
    - Create nested directory structures
    - Respect MaxDepth limit
    - Distribute files across directories
    - _Requirements: 3.5, 3.6, 8.3_

  - [x] 5.4 Create rapid generators in fixtures.go
    - RapidFileCountGenerator respects MaxFiles
    - RapidFileSizeGenerator respects MaxFileSize
    - RapidDepthGenerator respects MaxDepth
    - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5, 3.6_

  - [x] 5.5 Write property test for file count limits in fixtures_test.go
    - **Property 1: File Count Limit Enforcement**
    - **Validates: Requirements 3.1, 3.2**

  - [x] 5.6 Write property test for file size ranges in fixtures_test.go
    - **Property 2: File Size Range Compliance**
    - **Validates: Requirements 3.3, 3.4, 8.4**

  - [x] 5.7 Write property test for directory depth limits in fixtures_test.go
    - **Property 3: Directory Depth Limit Enforcement**
    - **Validates: Requirements 3.5, 3.6**

- [ ] 6. Implement cleanup utilities
  - [x] 6.1 Create CleanupTestDir() function in cleanup.go
    - Use project's optimized deletion engine
    - Handle cleanup failures gracefully (log but don't fail)
    - _Requirements: 8.5_

  - [x] 6.2 Create RegisterCleanup() function in cleanup.go
    - Integrate with t.Cleanup()
    - Use CleanupTestDir for efficient deletion
    - _Requirements: 8.5_

  - [x] 6.3 Add cleanup after timeout in timeout.go
    - Integrate cleanup with timeout mechanism
    - Ensure files are removed when tests timeout
    - _Requirements: 4.4_

  - [x] 6.4 Write property test for cleanup after timeout in cleanup_test.go
    - **Property 6: Cleanup After Timeout**
    - **Validates: Requirements 4.4**

- [x] 7. Checkpoint - Ensure test utilities work correctly
  - Run all testutil package tests: `go test ./internal/testutil`
  - Verify configuration parsing works
  - Verify fixture generation respects limits
  - Verify timeout mechanism works
  - Ask the user if questions arise

- [ ] 8. Refactor existing engine tests
  - [x] 8.1 Update TestCompleteDirectoryRemoval in engine_test.go
    - Replace hardcoded file creation with testutil.CreateTestDirectory()
    - Use testutil.GetRapidOptions() for iteration count
    - Reduce file counts: 50 files (quick), 500 files (thorough)
    - Remove verbose logging (keep only on failure)
    - _Requirements: 2.1, 2.2, 3.1, 3.2, 7.1, 7.2_

  - [x] 8.2 Update TestDeletionIsolation in engine_test.go
    - Use testutil.CreateTestDirectory() for test data
    - Reduce to 2 directories with 20 files each (quick mode)
    - Use testutil.GetRapidOptions() for iteration count
    - Remove verbose logging
    - _Requirements: 2.1, 2.2, 3.1, 3.2, 7.1, 7.2_

  - [x] 8.3 Update TestErrorResilience in engine_test.go
    - Use testutil.CreateTestDirectory() for test data
    - Reduce to 30 files with 5 error injections (quick mode)
    - Use 1-byte file sizes
    - Remove verbose logging
    - _Requirements: 2.1, 2.2, 3.1, 3.3, 7.1, 7.2_

  - [x] 8.4 Update TestErrorTrackingAccuracy in engine_test.go
    - Use testutil.CreateTestDirectory() for test data
    - Reduce file counts for quick mode (30 files max)
    - Remove verbose logging
    - _Requirements: 2.1, 2.2, 3.1, 7.1, 7.2_

  - [x] 8.5 Update TestDryRunPreservation in engine_test.go
    - Use testutil.CreateTestDirectory() for test data
    - Reduce file counts for quick mode (50 files max)
    - Remove verbose logging
    - _Requirements: 2.1, 2.2, 3.1, 7.1, 7.2_

  - [x] 8.6 Update TestBufferSizeCalculation in engine_test.go
    - Use testutil.GetRapidOptions() for iteration count
    - Reduce max file count to 10,000 (from 50,000) for quick mode
    - Remove verbose logging
    - _Requirements: 2.1, 2.2, 7.1, 7.2_

  - [x] 8.7 Update TestAdaptiveWorkerAdjustment in engine_test.go
    - Use testutil.GetRapidOptions() for iteration count
    - Reduce file counts to 500-1000 (from 1000-5000)
    - Remove verbose logging
    - _Requirements: 2.1, 2.2, 7.1, 7.2_

- [ ] 9. Refactor scanner tests
  - [x] 9.1 Update TestAgeBasedFilteringProperty in scanner_test.go
    - Use testutil.GetRapidOptions() for iteration count
    - Reduce max files to 20 (from unlimited)
    - Remove verbose logging
    - _Requirements: 2.1, 2.2, 3.1, 7.1, 7.2_

  - [x] 9.2 Update TestModificationTimestampUsageProperty in scanner_test.go
    - Use testutil.GetRapidOptions() for iteration count
    - Reduce max files to 10 (from unlimited)
    - Remove verbose logging
    - _Requirements: 2.1, 2.2, 3.1, 7.1, 7.2_

  - [x] 9.3 Update TestRetentionCountAccuracyProperty in scanner_test.go
    - Use testutil.GetRapidOptions() for iteration count
    - Reduce max files to 20 (from unlimited)
    - Remove verbose logging
    - _Requirements: 2.1, 2.2, 3.1, 7.1, 7.2_

- [x] 10. Fix or remove TestBatchMemoryRelease
  - [x] 10.1 Investigate infinite loop issue
    - Review test code for loop termination conditions
    - Identify why test loops indefinitely
    - _Requirements: 5.1_

  - [x] 10.2 Implement fix or removal decision
    - If fixable: Add timeout, reduce batch size to 1000 files, fix loop condition
    - If not fixable: Remove test and document decision in commit message
    - Verify batch memory behavior is covered by other tests
    - _Requirements: 5.1, 5.2, 5.4, 5.5_

- [x] 11. Checkpoint - Ensure refactored tests pass
  - Run all engine tests with quick mode: `go test ./internal/engine`
  - Run all scanner tests with quick mode: `go test ./internal/scanner`
  - Verify no tests are skipped (except TestBatchMemoryRelease if not fixed)
  - Measure execution time improvement
  - Ask the user if questions arise

- [x] 12. Separate stress tests
  - [x] 12.1 Create engine_stress_test.go file
    - Add `//go:build stress` build tag at top of file
    - Add package documentation explaining stress tests
    - _Requirements: 6.1, 6.2, 6.3_

  - [x] 12.2 Move TestBatchMemoryRelease to stress file (if fixed)
    - Move test to engine_stress_test.go
    - Keep original test logic but with stress-appropriate data sizes
    - _Requirements: 6.1, 6.2, 6.3_

  - [x] 12.3 Create stress versions of property tests
    - Create stress version of TestCompleteDirectoryRemoval (10,000 files)
    - Create stress version of TestBufferSizeCalculation (50,000 files)
    - _Requirements: 6.1, 6.2, 6.3_

  - [x] 12.4 Update Makefile with stress test targets
    - Update `test` target to remove `-v` flag (too verbose)
    - Add `test-quick` target: `TEST_INTENSITY=quick go test ./...`
    - Add `test-thorough` target: `TEST_INTENSITY=thorough go test ./...`
    - Add `test-stress` target: `go test -tags=stress ./...`
    - Add `test-all` target: `TEST_INTENSITY=thorough go test -tags=stress ./...`
    - _Requirements: 6.2, 6.3_

  - [x] 12.5 Verify stress test separation
    - Run tests without build tag, verify stress tests excluded
    - Run tests with stress tag, verify stress tests included
    - Verify non-stress tests maintain >80% code coverage
    - _Requirements: 6.2, 6.3, 6.5_

- [x] 13. Update documentation
  - [x] 13.1 Add test configuration section to README
    - Document TEST_INTENSITY environment variable
    - Document TEST_QUICK environment variable
    - Document VERBOSE_TESTS environment variable
    - _Requirements: 9.1_

  - [x] 13.2 Add test execution examples to README
    - Provide examples for local development (quick mode)
    - Provide examples for CI environments (thorough mode)
    - Provide examples for running stress tests
    - Document expected execution times
    - _Requirements: 9.2, 9.3, 9.4, 9.5_

  - [x] 13.3 Update Makefile help documentation
    - Document all test-related make targets
    - Explain when to use each target
    - _Requirements: 9.2, 9.3, 9.5_

- [x] 14. Performance validation
  - [x] 14.1 Measure baseline performance
    - Time current test suite execution (all tests)
    - Document current execution time
    - Document current output size

  - [x] 14.2 Measure optimized performance
    - Time quick mode test suite execution
    - Time thorough mode test suite execution
    - Document execution time improvements
    - Verify 50% reduction in quick mode execution time
    - Verify output is minimal (no verbose logging)

- [x] 15. Final checkpoint - Verify all requirements met
  - Run complete test suite in quick mode (should complete in <30 seconds)
  - Run complete test suite in thorough mode (should complete in <5 minutes)
  - Run stress tests separately
  - Verify zero skipped tests (except intentionally removed ones)
  - Verify all property tests pass
  - Verify documentation is complete
  - Ask the user if questions arise

## Notes

- **Current State**: No testutil package exists. All tests use hardcoded values and generate excessive output.
- **Priority**: Create testutil package first (tasks 1-6), then refactor existing tests (tasks 8-9).
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation
- Property tests validate universal correctness properties
- Unit tests validate specific examples and edge cases
- The implementation uses Go's testing framework and the rapid property-based testing library
- Test utilities are designed to be reusable across all test files in the project
- **Critical**: Remove `-v` flag from test commands to prevent output crashes
