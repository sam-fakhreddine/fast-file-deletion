// Package logger provides structured logging functionality with configurable
// log levels and output destinations. It supports both console and file logging
// with timestamps and severity levels.
package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"time"
)

// LogLevel represents the severity level of a log message.
type LogLevel int

const (
	// DEBUG level for detailed diagnostic information (verbose mode only)
	DEBUG LogLevel = iota
	// INFO level for general informational messages
	INFO
	// WARNING level for potentially problematic situations
	WARNING
	// ERROR level for error events that might still allow the application to continue
	ERROR
)

// Logger manages application logging with configurable levels and output destinations.
// It supports writing to both stderr and a log file simultaneously, and filters
// messages based on the configured log level.
type Logger struct {
	level      LogLevel
	fileWriter io.WriteCloser
	logger     *log.Logger
}

var (
	// globalLogger is the singleton logger instance used throughout the application
	globalLogger *Logger
)

// SetupLogging initializes the global logger with the specified configuration.
//
// Parameters:
//   - verbose: If true, enables DEBUG level logging (shows all messages)
//   - logFile: If non-empty, writes logs to the specified file path in addition to stderr
//
// The logger writes to stderr by default. If a log file is specified, it writes to both
// stderr and the file using io.MultiWriter. The log file is opened in append mode,
// creating it if it doesn't exist.
//
// Returns an error if the log file cannot be created or opened.
func SetupLogging(verbose bool, logFile string) error {
	level := INFO
	if verbose {
		level = DEBUG
	}

	var fileWriter io.WriteCloser
	var output io.Writer = os.Stderr

	// Set up file logging if specified
	if logFile != "" {
		f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("failed to open log file %s: %w", logFile, err)
		}
		fileWriter = f
		// Write to both stderr and file
		output = io.MultiWriter(os.Stderr, f)
	}

	// Create logger with timestamp and no prefix (we'll add our own)
	logger := log.New(output, "", 0)

	globalLogger = &Logger{
		level:      level,
		fileWriter: fileWriter,
		logger:     logger,
	}

	return nil
}

// Close closes the log file if one was opened.
// This should be called before application exit to ensure all log data is flushed.
// It's safe to call this even if no log file was opened (returns nil).
// It's also safe to call multiple times (idempotent).
func Close() error {
	if globalLogger != nil && globalLogger.fileWriter != nil {
		err := globalLogger.fileWriter.Close()
		// Set fileWriter to nil after closing to make this idempotent
		globalLogger.fileWriter = nil
		return err
	}
	return nil
}

// Debug logs a debug-level message (only shown in verbose mode).
// Debug messages provide detailed diagnostic information useful for troubleshooting.
// These messages are filtered out unless verbose logging is enabled.
func Debug(format string, args ...interface{}) {
	logMessage(DEBUG, format, args...)
}

// Info logs an informational message.
// Info messages provide general information about application progress and state.
func Info(format string, args ...interface{}) {
	logMessage(INFO, format, args...)
}

// Warning logs a warning message.
// Warning messages indicate potentially problematic situations that don't prevent
// the application from continuing (e.g., skipped files, non-critical errors).
func Warning(format string, args ...interface{}) {
	logMessage(WARNING, format, args...)
}

// Error logs an error message.
// Error messages indicate serious problems that occurred during execution
// but may still allow the application to continue (e.g., file deletion failures).
func Error(format string, args ...interface{}) {
	logMessage(ERROR, format, args...)
}

// LogFileError logs a file-specific error with structured formatting.
// This is used for tracking individual file deletion failures with detailed context.
// The output includes the timestamp, error level, file path, and error reason.
//
// Example output:
//
//	2026-01-14 10:23:45 [ERROR] Failed to delete file
//	  Path: C:\path\to\file.txt
//	  Reason: Permission denied (Access is denied)
func LogFileError(path string, err error) {
	if globalLogger == nil {
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	globalLogger.logger.Printf("%s [ERROR] Failed to delete file\n  Path: %s\n  Reason: %v\n",
		timestamp, path, err)
}

// LogFileWarning logs a file-specific warning with structured formatting.
// This is used for tracking skipped files or non-critical issues.
// The output includes the timestamp, warning level, file path, and reason.
//
// Example output:
//
//	2026-01-14 10:23:45 [WARNING] Skipped file
//	  Path: C:\path\to\file.txt
//	  Reason: File is locked by another process
func LogFileWarning(path string, reason string) {
	if globalLogger == nil {
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	globalLogger.logger.Printf("%s [WARNING] Skipped file\n  Path: %s\n  Reason: %s\n",
		timestamp, path, reason)
}

// logMessage is the internal function that handles all logging.
// It checks the log level and formats messages with timestamps and severity levels.
// Messages below the configured log level are filtered out.
// If the logger is not initialized, it falls back to standard log.Printf.
func logMessage(level LogLevel, format string, args ...interface{}) {
	if globalLogger == nil {
		// Logger not initialized, use default stderr logging
		log.Printf(format, args...)
		return
	}

	// Check if this message should be logged based on current level
	if level < globalLogger.level {
		return
	}

	// Format the message
	message := fmt.Sprintf(format, args...)

	// Add timestamp and level prefix
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	levelStr := levelToString(level)

	globalLogger.logger.Printf("%s [%s] %s", timestamp, levelStr, message)
}

// levelToString converts a LogLevel to its string representation.
// This is used for formatting log messages with severity level prefixes.
func levelToString(level LogLevel) string {
	switch level {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARNING:
		return "WARNING"
	case ERROR:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}
