# Requirements Document

## Introduction

The fast-file-deletion project uses property-based testing with the `rapid` framework to ensure correctness across a wide range of inputs. However, the current test suite has significant performance issues that impact developer productivity and CI/CD pipeline efficiency. Tests generate massive output that crashes the IDE, take too long to execute (creating 100k+ files), and consume excessive resources. This feature will optimize the test suite to be faster, more configurable, and more maintainable while preserving comprehensive correctness validation.

## Glossary

- **Test_Suite**: The collection of all automated tests in the project
- **Property_Test**: A test that validates universal properties across many generated inputs using the rapid framework
- **Unit_Test**: A test that validates specific examples and edge cases
- **Test_Intensity**: The thoroughness level of test execution (quick vs thorough)
- **Iteration_Count**: The number of random inputs generated for each property test
- **Test_Timeout**: Maximum execution time allowed for a test before it is terminated
- **CI_Environment**: Continuous Integration environment where tests run automatically
- **Local_Environment**: Developer's local machine where tests run during development

## Requirements

### Requirement 1: Configurable Test Intensity

**User Story:** As a developer, I want to control test execution intensity through environment variables, so that I can run quick tests during development and thorough tests in CI.

#### Acceptance Criteria

1. WHEN the TEST_INTENSITY environment variable is set to "quick", THE Test_Suite SHALL use reduced iteration counts (10-20 iterations per property test)
2. WHEN the TEST_INTENSITY environment variable is set to "thorough", THE Test_Suite SHALL use comprehensive iteration counts (100+ iterations per property test)
3. WHEN the TEST_INTENSITY environment variable is not set, THE Test_Suite SHALL default to "quick" mode
4. WHEN the TEST_QUICK environment variable is set to "1" or "true", THE Test_Suite SHALL use quick mode regardless of TEST_INTENSITY
5. THE Test_Suite SHALL log the active test intensity mode at the start of property test execution

### Requirement 2: Reduced Property Test Iteration Counts

**User Story:** As a developer, I want property tests to run faster by default, so that I can iterate quickly during development without sacrificing correctness validation.

#### Acceptance Criteria

1. WHEN running in quick mode, THE Property_Test SHALL execute with 10-20 iterations maximum
2. WHEN running in thorough mode, THE Property_Test SHALL execute with 100-200 iterations
3. WHEN a property test completes, THE Test_Suite SHALL report the number of iterations executed
4. THE Test_Suite SHALL maintain the same correctness properties regardless of iteration count
5. WHEN running property tests, THE Test_Suite SHALL use rapid's MinCount option to control iteration counts

### Requirement 3: Optimized Test Data Sizes

**User Story:** As a developer, I want property tests to use smaller test data sets, so that tests complete faster while still validating correctness properties.

#### Acceptance Criteria

1. WHEN generating test directories in quick mode, THE Property_Test SHALL create maximum 100 files per test case
2. WHEN generating test directories in thorough mode, THE Property_Test SHALL create maximum 1000 files per test case
3. WHEN generating test file sizes, THE Property_Test SHALL use sizes between 1 byte and 1KB in quick mode
4. WHEN generating test file sizes, THE Property_Test SHALL use sizes between 1 byte and 10KB in thorough mode
5. WHEN generating directory depths, THE Property_Test SHALL use maximum depth of 3 levels in quick mode
6. WHEN generating directory depths, THE Property_Test SHALL use maximum depth of 5 levels in thorough mode

### Requirement 4: Test Execution Timeouts

**User Story:** As a developer, I want tests to have execution time limits, so that hanging or infinite-loop tests don't block the test suite indefinitely.

#### Acceptance Criteria

1. WHEN running in quick mode, THE Property_Test SHALL timeout after 30 seconds
2. WHEN running in thorough mode, THE Property_Test SHALL timeout after 5 minutes
3. WHEN a test exceeds its timeout, THE Test_Suite SHALL terminate the test and report a timeout failure
4. WHEN a test times out, THE Test_Suite SHALL clean up any created test files and directories
5. THE Test_Suite SHALL use Go's testing.T.Deadline() to implement timeouts

### Requirement 5: Fix or Remove Skipped Tests

**User Story:** As a developer, I want all tests to be either functional or removed, so that the test suite accurately reflects the project's test coverage.

#### Acceptance Criteria

1. WHEN TestBatchMemoryRelease is identified as looping indefinitely, THE Test_Suite SHALL either fix the infinite loop or remove the test
2. IF TestBatchMemoryRelease is fixed, THEN THE Test_Suite SHALL execute it with appropriate timeouts
3. IF TestBatchMemoryRelease cannot be fixed, THEN THE Test_Suite SHALL remove it and document the decision
4. WHEN all tests are executed, THE Test_Suite SHALL have zero skipped tests
5. THE Test_Suite SHALL validate that batch memory release behavior is covered by other tests if TestBatchMemoryRelease is removed

### Requirement 6: Separate Long-Running Tests

**User Story:** As a developer, I want long-running stress tests separated from fast unit tests, so that I can run quick tests frequently and thorough tests less often.

#### Acceptance Criteria

1. WHEN property tests are organized, THE Test_Suite SHALL place stress tests in files with "_stress_test.go" suffix
2. WHEN running tests without build tags, THE Test_Suite SHALL exclude stress tests by default
3. WHEN the "stress" build tag is provided, THE Test_Suite SHALL include stress tests
4. THE Test_Suite SHALL document how to run stress tests separately in the README
5. WHEN stress tests are separated, THE Test_Suite SHALL maintain at least 80% code coverage with non-stress tests

### Requirement 7: Reduced Test Output Verbosity

**User Story:** As a developer, I want property tests to generate minimal output, so that test results are readable and don't crash the IDE.

#### Acceptance Criteria

1. WHEN property tests execute, THE Test_Suite SHALL not log individual iteration details by default
2. WHEN a property test fails, THE Test_Suite SHALL log only the failing input and error message
3. WHEN property tests complete successfully, THE Test_Suite SHALL log only a summary line
4. THE Test_Suite SHALL provide a VERBOSE_TESTS environment variable to enable detailed logging when needed
5. WHEN VERBOSE_TESTS is enabled, THE Test_Suite SHALL log iteration progress every 10 iterations

### Requirement 8: Optimized File Creation Patterns

**User Story:** As a developer, I want test file creation to be efficient, so that test setup time is minimized.

#### Acceptance Criteria

1. WHEN creating test files, THE Test_Suite SHALL use buffered I/O operations
2. WHEN creating multiple test files, THE Test_Suite SHALL reuse file handles where possible
3. WHEN creating test directories, THE Test_Suite SHALL use os.MkdirAll for efficient nested directory creation
4. WHEN writing test file content, THE Test_Suite SHALL use minimal content sizes (1-100 bytes in quick mode)
5. WHEN cleaning up test files, THE Test_Suite SHALL use the project's optimized deletion engine

### Requirement 9: Test Configuration Documentation

**User Story:** As a developer, I want clear documentation on test configuration options, so that I can understand how to run tests effectively.

#### Acceptance Criteria

1. THE Test_Suite SHALL document all test-related environment variables in the README
2. THE Test_Suite SHALL provide examples of running quick tests for local development
3. THE Test_Suite SHALL provide examples of running thorough tests for CI environments
4. THE Test_Suite SHALL document the expected execution time for quick vs thorough modes
5. THE Test_Suite SHALL document how to run stress tests separately
