package logger

import (
	"bytes"
	"io"
	"log"
	"os"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// Feature: fast-file-deletion, Property 12: Verbose Logging
// For any deletion operation, when verbose mode is enabled, the log output
// should contain more detailed information than when verbose mode is disabled.
// Validates: Requirements 6.5
func TestVerboseLogging(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate random log messages
		numDebugMessages := rapid.IntRange(1, 10).Draw(rt, "numDebugMessages")
		numInfoMessages := rapid.IntRange(1, 10).Draw(rt, "numInfoMessages")
		numWarningMessages := rapid.IntRange(0, 5).Draw(rt, "numWarningMessages")
		numErrorMessages := rapid.IntRange(0, 5).Draw(rt, "numErrorMessages")

		// Test 1: Non-verbose mode (INFO level) - should NOT show DEBUG messages
		var nonVerboseOutput bytes.Buffer
		testNonVerboseLogging(t, &nonVerboseOutput, numDebugMessages, numInfoMessages, numWarningMessages, numErrorMessages)

		// Test 2: Verbose mode (DEBUG level) - should show ALL messages including DEBUG
		var verboseOutput bytes.Buffer
		testVerboseLogging(t, &verboseOutput, numDebugMessages, numInfoMessages, numWarningMessages, numErrorMessages)

		// Property: Verbose output should contain more information than non-verbose output
		verboseLen := verboseOutput.Len()
		nonVerboseLen := nonVerboseOutput.Len()

		if verboseLen <= nonVerboseLen {
			rt.Fatalf("Verbose output (%d bytes) should be longer than non-verbose output (%d bytes)",
				verboseLen, nonVerboseLen)
		}

		// Property: Verbose output should contain DEBUG messages
		verboseStr := verboseOutput.String()
		if !strings.Contains(verboseStr, "[DEBUG]") {
			rt.Fatalf("Verbose output should contain [DEBUG] messages, but none found")
		}

		// Property: Non-verbose output should NOT contain DEBUG messages
		nonVerboseStr := nonVerboseOutput.String()
		if strings.Contains(nonVerboseStr, "[DEBUG]") {
			rt.Fatalf("Non-verbose output should NOT contain [DEBUG] messages, but found some")
		}

		// Property: Both outputs should contain INFO, WARNING, and ERROR messages
		// (if any were generated)
		if numInfoMessages > 0 {
			if !strings.Contains(verboseStr, "[INFO]") {
				rt.Fatalf("Verbose output should contain [INFO] messages")
			}
			if !strings.Contains(nonVerboseStr, "[INFO]") {
				rt.Fatalf("Non-verbose output should contain [INFO] messages")
			}
		}

		if numWarningMessages > 0 {
			if !strings.Contains(verboseStr, "[WARNING]") {
				rt.Fatalf("Verbose output should contain [WARNING] messages")
			}
			if !strings.Contains(nonVerboseStr, "[WARNING]") {
				rt.Fatalf("Non-verbose output should contain [WARNING] messages")
			}
		}

		if numErrorMessages > 0 {
			if !strings.Contains(verboseStr, "[ERROR]") {
				rt.Fatalf("Verbose output should contain [ERROR] messages")
			}
			if !strings.Contains(nonVerboseStr, "[ERROR]") {
				rt.Fatalf("Non-verbose output should contain [ERROR] messages")
			}
		}

		// Property: The number of DEBUG messages in verbose output should match
		// the number we generated
		debugCount := strings.Count(verboseStr, "[DEBUG]")
		if debugCount != numDebugMessages {
			rt.Fatalf("Expected %d DEBUG messages in verbose output, got %d",
				numDebugMessages, debugCount)
		}

		// Property: Non-verbose output should have zero DEBUG messages
		debugCountNonVerbose := strings.Count(nonVerboseStr, "[DEBUG]")
		if debugCountNonVerbose != 0 {
			rt.Fatalf("Expected 0 DEBUG messages in non-verbose output, got %d",
				debugCountNonVerbose)
		}
	})
}

// testNonVerboseLogging sets up a logger in non-verbose mode and logs messages
func testNonVerboseLogging(t *testing.
T, output io.Writer, numDebug, numInfo, numWarning, numError int) {
	// Save the current global logger
	oldLogger := globalLogger
	defer func() {
		globalLogger = oldLogger
	}()

	// Set up non-verbose logging (INFO level)
	globalLogger = &Logger{
		level:      INFO,
		fileWriter: nil,
		logger:     newTestLogger(output),
	}

	// Log messages at different levels
	for i := 0; i < numDebug; i++ {
		Debug("Debug message %d", i)
	}
	for i := 0; i < numInfo; i++ {
		Info("Info message %d", i)
	}
	for i := 0; i < numWarning; i++ {
		Warning("Warning message %d", i)
	}
	for i := 0; i < numError; i++ {
		Error("Error message %d", i)
	}
}

// testVerboseLogging sets up a logger in verbose mode and logs messages
func testVerboseLogging(t *testing.T, output io.Writer, numDebug, numInfo, numWarning, numError int) {
	// Save the current global logger
	oldLogger := globalLogger
	defer func() {
		globalLogger = oldLogger
	}()

	// Set up verbose logging (DEBUG level)
	globalLogger = &Logger{
		level:      DEBUG,
		fileWriter: nil,
		logger:     newTestLogger(output),
	}

	// Log messages at different levels
	for i := 0; i < numDebug; i++ {
		Debug("Debug message %d", i)
	}
	for i := 0; i < numInfo; i++ {
		Info("Info message %d", i)
	}
	for i := 0; i < numWarning; i++ {
		Warning("Warning message %d", i)
	}
	for i := 0; i < numError; i++ {
		Error("Error message %d", i)
	}
}

// newTestLogger creates a logger that writes to the given writer
func newTestLogger(output io.Writer) *log.Logger {
	return log.New(output, "", 0)
}

// Unit test to verify basic verbose vs non-verbose behavior
func TestVerboseVsNonVerbose(t *testing.T) {
	// Test non-verbose mode
	var nonVerboseOutput bytes.Buffer
	oldLogger := globalLogger
	defer func() {
		globalLogger = oldLogger
	}()

	globalLogger = &Logger{
		level:      INFO,
		fileWriter: nil,
		logger:     newTestLogger(&nonVerboseOutput),
	}

	Debug("This should not appear")
	Info("This should appear")

	nonVerboseStr := nonVerboseOutput.String()
	if strings.Contains(nonVerboseStr, "This should not appear") {
		t.Errorf("Non-verbose mode should not show DEBUG messages")
	}
	if !strings.Contains(nonVerboseStr, "This should appear") {
		t.Errorf("Non-verbose mode should show INFO messages")
	}

	// Test verbose mode
	var verboseOutput bytes.Buffer
	globalLogger = &Logger{
		level:      DEBUG,
		fileWriter: nil,
		logger:     newTestLogger(&verboseOutput),
	}

	Debug("This should appear in verbose")
	Info("This should also appear")

	verboseStr := verboseOutput.String()
	if !strings.Contains(verboseStr, "This should appear in verbose") {
		t.Errorf("Verbose mode should show DEBUG messages")
	}
	if !strings.Contains(verboseStr, "This should also appear") {
		t.Errorf("Verbose mode should show INFO messages")
	}
}

// Unit test to verify SetupLogging correctly configures verbose mode
func TestSetupLoggingVerboseFlag(t *testing.T) {
	// Clean up after test
	defer func() {
		if globalLogger != nil && globalLogger.fileWriter != nil {
			globalLogger.fileWriter.Close()
		}
		globalLogger = nil
	}()

	// Test verbose=false (should set INFO level)
	err := SetupLogging(false, "")
	if err != nil {
		t.Fatalf("SetupLogging failed: %v", err)
	}
	if globalLogger.level != INFO {
		t.Errorf("Expected INFO level with verbose=false, got %v", globalLogger.level)
	}

	// Test verbose=true (should set DEBUG level)
	err = SetupLogging(true, "")
	if err != nil {
		t.Fatalf("SetupLogging failed: %v", err)
	}
	if globalLogger.level != DEBUG {
		t.Errorf("Expected DEBUG level with verbose=true, got %v", globalLogger.level)
	}
}

// Feature: fast-file-deletion, Property 13: Log File Creation
// For any deletion operation with a log-file path specified, a log file should
// be created at that path containing operation details and any errors encountered.
// Validates: Requirements 6.6
func TestLogFileCreation(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate random log file path in temp directory
		tmpDir := t.TempDir()
		logFileName := rapid.StringMatching(`[a-zA-Z0-9_-]+\.log`).Draw(rt, "logFileName")
		logFilePath := tmpDir + "/" + logFileName

		// Generate random operation details and errors
		numInfoMessages := rapid.IntRange(1, 10).Draw(rt, "numInfoMessages")
		numErrorMessages := rapid.IntRange(0, 10).Draw(rt, "numErrorMessages")
		numWarningMessages := rapid.IntRange(0, 10).Draw(rt, "numWarningMessages")
		numFileErrors := rapid.IntRange(0, 5).Draw(rt, "numFileErrors")

		// Clean up after test
		defer func() {
			if globalLogger != nil && globalLogger.fileWriter != nil {
				globalLogger.fileWriter.Close()
			}
			globalLogger = nil
			os.Remove(logFilePath)
		}()

		// Property: SetupLogging with a log file path should succeed
		err := SetupLogging(false, logFilePath)
		if err != nil {
			rt.Fatalf("SetupLogging should succeed with valid log file path, got error: %v", err)
		}

		// Property: After SetupLogging, the log file should exist
		if _, err := os.Stat(logFilePath); os.IsNotExist(err) {
			rt.Fatalf("Log file should be created at %s after SetupLogging", logFilePath)
		}

		// Log various operation details
		for i := 0; i < numInfoMessages; i++ {
			Info("Operation detail %d: Processing files", i)
		}

		for i := 0; i < numErrorMessages; i++ {
			Error("Operation error %d: Failed to process", i)
		}

		for i := 0; i < numWarningMessages; i++ {
			Warning("Operation warning %d: Skipped item", i)
		}

		// Log file-specific errors (simulating deletion errors)
		for i := 0; i < numFileErrors; i++ {
			filePath := rapid.StringMatching(`C:\\[a-zA-Z0-9_\\]+\.txt`).Draw(rt, "filePath")
			LogFileError(filePath, os.ErrPermission)
		}

		// Close the logger to flush all data to file
		err = Close()
		if err != nil {
			rt.Fatalf("Close should succeed, got error: %v", err)
		}

		// Property: The log file should exist after logging
		if _, err := os.Stat(logFilePath); os.IsNotExist(err) {
			rt.Fatalf("Log file should exist at %s after logging operations", logFilePath)
		}

		// Property: The log file should contain operation details
		content, err := os.ReadFile(logFilePath)
		if err != nil {
			rt.Fatalf("Should be able to read log file, got error: %v", err)
		}

		contentStr := string(content)

		// Property: Log file should not be empty if we logged messages
		totalMessages := numInfoMessages + numErrorMessages + numWarningMessages + numFileErrors
		if totalMessages > 0 && len(contentStr) == 0 {
			rt.Fatalf("Log file should not be empty when %d messages were logged", totalMessages)
		}

		// Property: Log file should contain INFO messages if any were logged
		if numInfoMessages > 0 {
			if !strings.Contains(contentStr, "[INFO]") {
				rt.Fatalf("Log file should contain [INFO] messages when %d info messages were logged", numInfoMessages)
			}
			if !strings.Contains(contentStr, "Operation detail") {
				rt.Fatalf("Log file should contain operation details")
			}
		}

		// Property: Log file should contain ERROR messages if any were logged
		if numErrorMessages > 0 {
			if !strings.Contains(contentStr, "[ERROR]") {
				rt.Fatalf("Log file should contain [ERROR] messages when %d error messages were logged", numErrorMessages)
			}
		}

		// Property: Log file should contain WARNING messages if any were logged
		if numWarningMessages > 0 {
			if !strings.Contains(contentStr, "[WARNING]") {
				rt.Fatalf("Log file should contain [WARNING] messages when %d warning messages were logged", numWarningMessages)
			}
		}

		// Property: Log file should contain file-specific errors if any were logged
		if numFileErrors > 0 {
			if !strings.Contains(contentStr, "Failed to delete file") {
				rt.Fatalf("Log file should contain file deletion errors when %d file errors were logged", numFileErrors)
			}
			if !strings.Contains(contentStr, "Path:") {
				rt.Fatalf("Log file should contain file paths in error messages")
			}
			if !strings.Contains(contentStr, "Reason:") {
				rt.Fatalf("Log file should contain error reasons in error messages")
			}
		}

		// Property: Log file should contain timestamps
		if !strings.Contains(contentStr, "202") { // Year prefix in timestamp
			rt.Fatalf("Log file should contain timestamps in format YYYY-MM-DD HH:MM:SS")
		}

		// Property: The number of [INFO] occurrences should match the number logged
		infoCount := strings.Count(contentStr, "[INFO]")
		if infoCount != numInfoMessages {
			rt.Fatalf("Expected %d [INFO] messages in log file, got %d", numInfoMessages, infoCount)
		}

		// Property: The number of [ERROR] occurrences should match the number logged
		// (including both Error() calls and LogFileError() calls)
		errorCount := strings.Count(contentStr, "[ERROR]")
		expectedErrors := numErrorMessages + numFileErrors
		if errorCount != expectedErrors {
			rt.Fatalf("Expected %d [ERROR] messages in log file, got %d", expectedErrors, errorCount)
		}

		// Property: The number of [WARNING] occurrences should match the number logged
		warningCount := strings.Count(contentStr, "[WARNING]")
		if warningCount != numWarningMessages {
			rt.Fatalf("Expected %d [WARNING] messages in log file, got %d", numWarningMessages, warningCount)
		}
	})
}

// Unit test to verify basic log file creation
func TestBasicLogFileCreation(t *testing.T) {
	// Create a temporary log file
	tmpFile := t.TempDir() + "/test.log"

	// Clean up after test
	defer func() {
		if globalLogger != nil && globalLogger.fileWriter != nil {
			globalLogger.fileWriter.Close()
		}
		globalLogger = nil
		os.Remove(tmpFile)
	}()

	// Set up logging with a log file
	err := SetupLogging(false, tmpFile)
	if err != nil {
		t.Fatalf("SetupLogging failed: %v", err)
	}

	// Log a message
	Info("Test message")

	// Close the logger to flush the file
	err = Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Verify the log file was created and contains the message
	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), "Test message") {
		t.Errorf("Log file should contain 'Test message', got: %s", string(content))
	}
	if !strings.Contains(string(content), "[INFO]") {
		t.Errorf("Log file should contain [INFO] level, got: %s", string(content))
	}
}

// Unit test to verify log file creation with invalid path
func TestLogFileCreationInvalidPath(t *testing.T) {
	// Clean up after test
	defer func() {
		if globalLogger != nil && globalLogger.fileWriter != nil {
			globalLogger.fileWriter.Close()
		}
		globalLogger = nil
	}()

	// Try to create a log file in a non-existent directory
	invalidPath := "/nonexistent/directory/test.log"
	err := SetupLogging(false, invalidPath)
	if err == nil {
		t.Errorf("SetupLogging should fail with invalid path, but succeeded")
	}
	if !strings.Contains(err.Error(), "failed to open log file") {
		t.Errorf("Error message should mention 'failed to open log file', got: %v", err)
	}
}

// Unit test to verify log file append mode
func TestLogFileAppendMode(t *testing.T) {
	tmpFile := t.TempDir() + "/append_test.log"

	// Clean up after test
	defer func() {
		os.Remove(tmpFile)
	}()

	// First logging session
	err := SetupLogging(false, tmpFile)
	if err != nil {
		t.Fatalf("SetupLogging failed: %v", err)
	}
	Info("First message")
	Close()
	globalLogger = nil

	// Second logging session - should append, not overwrite
	err = SetupLogging(false, tmpFile)
	if err != nil {
		t.Fatalf("SetupLogging failed on second call: %v", err)
	}
	Info("Second message")
	Close()
	globalLogger = nil

	// Verify both messages are in the file
	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "First message") {
		t.Errorf("Log file should contain 'First message' from first session")
	}
	if !strings.Contains(contentStr, "Second message") {
		t.Errorf("Log file should contain 'Second message' from second session")
	}

	// Verify both messages are present (not overwritten)
	firstCount := strings.Count(contentStr, "First message")
	secondCount := strings.Count(contentStr, "Second message")
	if firstCount != 1 {
		t.Errorf("Expected 1 occurrence of 'First message', got %d", firstCount)
	}
	if secondCount != 1 {
		t.Errorf("Expected 1 occurrence of 'Second message', got %d", secondCount)
	}
}

// Unit test to verify log message formatting with timestamps
func TestLogMessageFormatting(t *testing.T) {
	var output bytes.Buffer
	oldLogger := globalLogger
	defer func() {
		globalLogger = oldLogger
	}()

	globalLogger = &Logger{
		level:      INFO,
		fileWriter: nil,
		logger:     newTestLogger(&output),
	}

	// Log messages at different levels
	Info("Info message")
	Warning("Warning message")
	Error("Error message")

	outputStr := output.String()

	// Verify timestamp format (YYYY-MM-DD HH:MM:SS)
	if !strings.Contains(outputStr, "202") {
		t.Errorf("Log output should contain timestamp with year, got: %s", outputStr)
	}

	// Verify level prefixes
	if !strings.Contains(outputStr, "[INFO]") {
		t.Errorf("Log output should contain [INFO] level")
	}
	if !strings.Contains(outputStr, "[WARNING]") {
		t.Errorf("Log output should contain [WARNING] level")
	}
	if !strings.Contains(outputStr, "[ERROR]") {
		t.Errorf("Log output should contain [ERROR] level")
	}

	// Verify message content
	if !strings.Contains(outputStr, "Info message") {
		t.Errorf("Log output should contain 'Info message'")
	}
	if !strings.Contains(outputStr, "Warning message") {
		t.Errorf("Log output should contain 'Warning message'")
	}
	if !strings.Contains(outputStr, "Error message") {
		t.Errorf("Log output should contain 'Error message'")
	}

	// Verify format: timestamp [LEVEL] message
	lines := strings.Split(strings.TrimSpace(outputStr), "\n")
	for _, line := range lines {
		// Each line should have format: YYYY-MM-DD HH:MM:SS [LEVEL] message
		if !strings.Contains(line, "[INFO]") && !strings.Contains(line, "[WARNING]") && !strings.Contains(line, "[ERROR]") {
			t.Errorf("Line should contain a log level: %s", line)
		}
	}
}

// Unit test to verify LogFileError formatting
func TestLogFileErrorFormatting(t *testing.T) {
	var output bytes.Buffer
	oldLogger := globalLogger
	defer func() {
		globalLogger = oldLogger
	}()

	globalLogger = &Logger{
		level:      INFO,
		fileWriter: nil,
		logger:     newTestLogger(&output),
	}

	// Log a file error
	testPath := "C:\\test\\file.txt"
	testErr := os.ErrPermission
	LogFileError(testPath, testErr)

	outputStr := output.String()

	// Verify structured format
	if !strings.Contains(outputStr, "[ERROR]") {
		t.Errorf("LogFileError output should contain [ERROR] level")
	}
	if !strings.Contains(outputStr, "Failed to delete file") {
		t.Errorf("LogFileError output should contain 'Failed to delete file'")
	}
	if !strings.Contains(outputStr, "Path:") {
		t.Errorf("LogFileError output should contain 'Path:' label")
	}
	if !strings.Contains(outputStr, testPath) {
		t.Errorf("LogFileError output should contain the file path: %s", testPath)
	}
	if !strings.Contains(outputStr, "Reason:") {
		t.Errorf("LogFileError output should contain 'Reason:' label")
	}
	if !strings.Contains(outputStr, "permission denied") {
		t.Errorf("LogFileError output should contain the error reason")
	}

	// Verify multi-line format
	lines := strings.Split(strings.TrimSpace(outputStr), "\n")
	if len(lines) < 3 {
		t.Errorf("LogFileError should produce at least 3 lines (header, path, reason), got %d", len(lines))
	}
}

// Unit test to verify LogFileWarning formatting
func TestLogFileWarningFormatting(t *testing.T) {
	var output bytes.Buffer
	oldLogger := globalLogger
	defer func() {
		globalLogger = oldLogger
	}()

	globalLogger = &Logger{
		level:      INFO,
		fileWriter: nil,
		logger:     newTestLogger(&output),
	}

	// Log a file warning
	testPath := "C:\\test\\locked.txt"
	testReason := "File is locked by another process"
	LogFileWarning(testPath, testReason)

	outputStr := output.String()

	// Verify structured format
	if !strings.Contains(outputStr, "[WARNING]") {
		t.Errorf("LogFileWarning output should contain [WARNING] level")
	}
	if !strings.Contains(outputStr, "Skipped file") {
		t.Errorf("LogFileWarning output should contain 'Skipped file'")
	}
	if !strings.Contains(outputStr, "Path:") {
		t.Errorf("LogFileWarning output should contain 'Path:' label")
	}
	if !strings.Contains(outputStr, testPath) {
		t.Errorf("LogFileWarning output should contain the file path: %s", testPath)
	}
	if !strings.Contains(outputStr, "Reason:") {
		t.Errorf("LogFileWarning output should contain 'Reason:' label")
	}
	if !strings.Contains(outputStr, testReason) {
		t.Errorf("LogFileWarning output should contain the reason: %s", testReason)
	}

	// Verify multi-line format
	lines := strings.Split(strings.TrimSpace(outputStr), "\n")
	if len(lines) < 3 {
		t.Errorf("LogFileWarning should produce at least 3 lines (header, path, reason), got %d", len(lines))
	}
}

// Unit test to verify Close is safe to call multiple times
func TestCloseIdempotent(t *testing.T) {
	tmpFile := t.TempDir() + "/close_test.log"

	// Clean up after test
	defer func() {
		globalLogger = nil
		os.Remove(tmpFile)
	}()

	// Set up logging with a log file
	err := SetupLogging(false, tmpFile)
	if err != nil {
		t.Fatalf("SetupLogging failed: %v", err)
	}

	// Close once
	err = Close()
	if err != nil {
		t.Errorf("First Close() should succeed, got error: %v", err)
	}

	// After first close, fileWriter should be nil
	// Close again - should be safe (returns nil when fileWriter is nil)
	err = Close()
	if err != nil {
		t.Errorf("Second Close() should succeed (idempotent), got error: %v", err)
	}

	// Third close should also be safe
	err = Close()
	if err != nil {
		t.Errorf("Third Close() should succeed (idempotent), got error: %v", err)
	}
}

// Unit test to verify Close is safe when no log file was opened
func TestCloseWithoutLogFile(t *testing.T) {
	// Clean up after test
	defer func() {
		globalLogger = nil
	}()

	// Set up logging without a log file
	err := SetupLogging(false, "")
	if err != nil {
		t.Fatalf("SetupLogging failed: %v", err)
	}

	// Close should succeed even though no file was opened
	err = Close()
	if err != nil {
		t.Errorf("Close() should succeed when no log file was opened, got error: %v", err)
	}
}

// Unit test to verify logging works when logger is not initialized
func TestLoggingWithoutInitialization(t *testing.T) {
	// Save and clear the global logger
	oldLogger := globalLogger
	globalLogger = nil
	defer func() {
		globalLogger = oldLogger
	}()

	// These should not panic, but fall back to default logging
	// We can't easily capture the output, but we can verify no panic occurs
	Debug("Debug without init")
	Info("Info without init")
	Warning("Warning without init")
	Error("Error without init")

	// If we get here without panic, the test passes
}

// Unit test to verify all log levels are correctly filtered
func TestLogLevelFiltering(t *testing.T) {
	tests := []struct {
		name          string
		level         LogLevel
		shouldShowDEBUG bool
		shouldShowINFO  bool
		shouldShowWARNING bool
		shouldShowERROR bool
	}{
		{
			name:          "DEBUG level shows all",
			level:         DEBUG,
			shouldShowDEBUG: true,
			shouldShowINFO:  true,
			shouldShowWARNING: true,
			shouldShowERROR: true,
		},
		{
			name:          "INFO level hides DEBUG",
			level:         INFO,
			shouldShowDEBUG: false,
			shouldShowINFO:  true,
			shouldShowWARNING: true,
			shouldShowERROR: true,
		},
		{
			name:          "WARNING level hides DEBUG and INFO",
			level:         WARNING,
			shouldShowDEBUG: false,
			shouldShowINFO:  false,
			shouldShowWARNING: true,
			shouldShowERROR: true,
		},
		{
			name:          "ERROR level shows only ERROR",
			level:         ERROR,
			shouldShowDEBUG: false,
			shouldShowINFO:  false,
			shouldShowWARNING: false,
			shouldShowERROR: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer
			oldLogger := globalLogger
			defer func() {
				globalLogger = oldLogger
			}()

			globalLogger = &Logger{
				level:      tt.level,
				fileWriter: nil,
				logger:     newTestLogger(&output),
			}

			Debug("DEBUG message")
			Info("INFO message")
			Warning("WARNING message")
			Error("ERROR message")

			outputStr := output.String()

			// Check DEBUG
			if tt.shouldShowDEBUG && !strings.Contains(outputStr, "DEBUG message") {
				t.Errorf("Should show DEBUG message at level %v", tt.level)
			}
			if !tt.shouldShowDEBUG && strings.Contains(outputStr, "DEBUG message") {
				t.Errorf("Should NOT show DEBUG message at level %v", tt.level)
			}

			// Check INFO
			if tt.shouldShowINFO && !strings.Contains(outputStr, "INFO message") {
				t.Errorf("Should show INFO message at level %v", tt.level)
			}
			if !tt.shouldShowINFO && strings.Contains(outputStr, "INFO message") {
				t.Errorf("Should NOT show INFO message at level %v", tt.level)
			}

			// Check WARNING
			if tt.shouldShowWARNING && !strings.Contains(outputStr, "WARNING message") {
				t.Errorf("Should show WARNING message at level %v", tt.level)
			}
			if !tt.shouldShowWARNING && strings.Contains(outputStr, "WARNING message") {
				t.Errorf("Should NOT show WARNING message at level %v", tt.level)
			}

			// Check ERROR
			if tt.shouldShowERROR && !strings.Contains(outputStr, "ERROR message") {
				t.Errorf("Should show ERROR message at level %v", tt.level)
			}
			if !tt.shouldShowERROR && strings.Contains(outputStr, "ERROR message") {
				t.Errorf("Should NOT show ERROR message at level %v", tt.level)
			}
		})
	}
}
