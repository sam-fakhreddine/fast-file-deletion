package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"testing"

	"pgregory.net/rapid"
)

// Feature: windows-performance-optimization, Property 19: CLI consistency across versions
// **Validates: Requirements 7.3**
//
// Property 19: CLI consistency across versions
// For any Windows version, the same command-line arguments should produce equivalent
// behavior (even if internal implementation differs).
//
// This property ensures backward compatibility by verifying that:
// 1. The same CLI flags are accepted on all platforms
// 2. The same configuration is produced from identical arguments
// 3. Invalid arguments are rejected consistently
// 4. Flag parsing behavior is deterministic and platform-independent
func TestCLIConsistencyAcrossVersions(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Create a temporary directory for testing
		tmpDir := t.TempDir()

		// Generate a random but valid configuration
		// This simulates the same command being run on different Windows versions
		
		// Generate boolean flags
		force := rapid.Bool().Draw(rt, "force")
		dryRun := rapid.Bool().Draw(rt, "dryRun")
		verbose := rapid.Bool().Draw(rt, "verbose")
		
		// Generate valid numeric parameters
		keepDaysValue := rapid.IntRange(-1, 365).Draw(rt, "keepDays")
		workersValue := rapid.IntRange(0, 100).Draw(rt, "workers")
		bufferSizeValue := rapid.IntRange(0, 10000).Draw(rt, "bufferSize")
		
		// Generate valid deletion method
		methods := []string{"auto", "fileinfo", "deleteonclose", "ntapi", "deleteapi"}
		methodIdx := rapid.IntRange(0, len(methods)-1).Draw(rt, "methodIdx")
		deletionMethod := methods[methodIdx]
		
		// Generate optional log file
		hasLogFile := rapid.Bool().Draw(rt, "hasLogFile")
		var logFile string
		if hasLogFile {
			logFile = "/tmp/test.log"
		}
		
		// Build command-line arguments
		args := []string{"fast-file-deletion", "-td", tmpDir}
		
		if force {
			args = append(args, "--force")
		}
		if dryRun {
			args = append(args, "--dry-run")
		}
		if verbose {
			args = append(args, "--verbose")
		}
		if keepDaysValue >= 0 {
			args = append(args, "--keep-days", fmt.Sprintf("%d", keepDaysValue))
		}
		if workersValue > 0 {
			args = append(args, "--workers", fmt.Sprintf("%d", workersValue))
		}
		if bufferSizeValue > 0 {
			args = append(args, "--buffer-size", fmt.Sprintf("%d", bufferSizeValue))
		}
		args = append(args, "--deletion-method", deletionMethod)
		if hasLogFile {
			args = append(args, "--log-file", logFile)
		}
		
		// Property 1: Parse the arguments multiple times - should produce identical results
		// This simulates running the same command on different occasions or versions
		
		// First parse
		config1 := parseArgumentsWithArgs(rt, args)
		
		// Second parse (simulating a different execution or Windows version)
		config2 := parseArgumentsWithArgs(rt, args)
		
		// Property: Parsing should be deterministic - same args produce same config
		if !configsEqual(config1, config2) {
			rt.Fatalf("CLI parsing is non-deterministic:\nFirst:  %+v\nSecond: %+v\nArgs: %v",
				config1, config2, args)
		}
		
		// Property 2: Verify all flags were parsed correctly
		// This ensures the CLI interface is consistent
		if config1.TargetDir != tmpDir {
			rt.Fatalf("TargetDir not parsed correctly: expected %s, got %s", tmpDir, config1.TargetDir)
		}
		
		if config1.Force != force {
			rt.Fatalf("Force flag not parsed correctly: expected %v, got %v", force, config1.Force)
		}
		
		if config1.DryRun != dryRun {
			rt.Fatalf("DryRun flag not parsed correctly: expected %v, got %v", dryRun, config1.DryRun)
		}
		
		if config1.Verbose != verbose {
			rt.Fatalf("Verbose flag not parsed correctly: expected %v, got %v", verbose, config1.Verbose)
		}
		
		if keepDaysValue >= 0 {
			if config1.KeepDays == nil {
				rt.Fatalf("KeepDays should not be nil when specified")
			}
			if *config1.KeepDays != keepDaysValue {
				rt.Fatalf("KeepDays not parsed correctly: expected %d, got %d", keepDaysValue, *config1.KeepDays)
			}
		} else {
			if config1.KeepDays != nil {
				rt.Fatalf("KeepDays should be nil when not specified, got %d", *config1.KeepDays)
			}
		}
		
		if workersValue > 0 {
			if config1.Workers != workersValue {
				rt.Fatalf("Workers not parsed correctly: expected %d, got %d", workersValue, config1.Workers)
			}
		}
		
		if bufferSizeValue > 0 {
			if config1.BufferSize != bufferSizeValue {
				rt.Fatalf("BufferSize not parsed correctly: expected %d, got %d", bufferSizeValue, config1.BufferSize)
			}
		}
		
		if config1.DeletionMethod != deletionMethod {
			rt.Fatalf("DeletionMethod not parsed correctly: expected %s, got %s", deletionMethod, config1.DeletionMethod)
		}
		
		if hasLogFile {
			if config1.LogFile != logFile {
				rt.Fatalf("LogFile not parsed correctly: expected %s, got %s", logFile, config1.LogFile)
			}
		}
		
		// Property 3: Flag order should not matter
		// Shuffle the flags and verify we get the same configuration
		if len(args) > 3 { // Only test if we have flags beyond the required ones
			shuffledArgs := shuffleArgs(rt, args)
			config3 := parseArgumentsWithArgs(rt, shuffledArgs)
			
			if !configsEqual(config1, config3) {
				rt.Fatalf("CLI parsing is order-dependent:\nOriginal: %+v\nShuffled: %+v\nOriginal args: %v\nShuffled args: %v",
					config1, config3, args, shuffledArgs)
			}
		}
		
		// Property 4: Platform detection should not affect CLI parsing
		// The CLI should accept the same arguments on all platforms
		// (even if some features are only available on Windows)
		currentPlatform := runtime.GOOS
		
		// On non-Windows platforms, the CLI should still parse the arguments
		// (validation of platform-specific features happens later in validateConfig)
		if currentPlatform != "windows" {
			// The config should still be valid from a parsing perspective
			if config1 == nil {
				rt.Fatalf("CLI parsing failed on non-Windows platform, but should succeed")
			}
		}
	})
}

// TestCLIConsistencyInvalidArguments tests that invalid arguments are rejected consistently
// **Validates: Requirements 7.3**
func TestCLIConsistencyInvalidArguments(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate various types of invalid argument combinations
		invalidScenario := rapid.IntRange(0, 5).Draw(rt, "invalidScenario")
		
		// Save and restore original args
		oldArgs := os.Args
		defer func() { os.Args = oldArgs }()
		
		// Reset flag package state
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
		
		var args []string
		var scenarioDescription string
		
		switch invalidScenario {
		case 0:
			// Invalid keep-days value (negative, but not -1)
			negativeValue := rapid.IntRange(-1000, -2).Draw(rt, "negativeKeepDays")
			args = []string{"fast-file-deletion", "-td", "/tmp/test", "--keep-days", fmt.Sprintf("%d", negativeValue)}
			scenarioDescription = "negative keep-days value"
			
		case 1:
			// Invalid workers value (negative)
			negativeWorkers := rapid.IntRange(-1000, -1).Draw(rt, "negativeWorkers")
			args = []string{"fast-file-deletion", "-td", "/tmp/test", "--workers", fmt.Sprintf("%d", negativeWorkers)}
			scenarioDescription = "negative workers value"
			
		case 2:
			// Invalid deletion method
			invalidMethod := rapid.StringMatching(`[a-z]+`).Draw(rt, "invalidMethod")
			// Ensure it's not a valid method
			validMethods := map[string]bool{
				"auto": true, "fileinfo": true, "deleteonclose": true,
				"ntapi": true, "deleteapi": true,
			}
			if validMethods[invalidMethod] {
				invalidMethod = "invalidmethod123"
			}
			args = []string{"fast-file-deletion", "-td", "/tmp/test", "--deletion-method", invalidMethod}
			scenarioDescription = "invalid deletion method"
			
		case 3:
			// Positional argument instead of flag
			randomPath := rapid.StringMatching(`[a-zA-Z0-9/_]+`).Draw(rt, "randomPath")
			if randomPath == "" || randomPath[0] == '-' {
				randomPath = "/tmp/test"
			}
			args = []string{"fast-file-deletion", randomPath}
			scenarioDescription = "positional argument instead of flag"
			
		case 4:
			// Extra positional arguments
			args = []string{"fast-file-deletion", "-td", "/tmp/test", "extra", "args"}
			scenarioDescription = "extra positional arguments"
			
		case 5:
			// Invalid buffer size (negative)
			negativeBuffer := rapid.IntRange(-1000, -1).Draw(rt, "negativeBuffer")
			args = []string{"fast-file-deletion", "-td", "/tmp/test", "--buffer-size", fmt.Sprintf("%d", negativeBuffer)}
			scenarioDescription = "negative buffer size"
		}
		
		// Set test arguments
		os.Args = args
		
		// Parse arguments - should fail consistently
		config, err := parseArguments()
		
		// Property: Invalid arguments should always produce an error
		if err == nil {
			rt.Fatalf("Scenario '%s': Expected error for invalid arguments, but got none. Config: %+v\nArgs: %v",
				scenarioDescription, config, args)
		}
		
		// Property: Error message should be non-empty and informative
		if err.Error() == "" {
			rt.Fatalf("Scenario '%s': Error message is empty", scenarioDescription)
		}
		
		// Property: Config should be nil when there's an error
		if config != nil {
			rt.Fatalf("Scenario '%s': Expected nil config with error, got: %+v",
				scenarioDescription, config)
		}
		
		// Property: Parse the same invalid args again - should produce the same error
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
		os.Args = args
		
		config2, err2 := parseArguments()
		
		if err2 == nil {
			rt.Fatalf("Scenario '%s': Second parse did not produce error (non-deterministic)", scenarioDescription)
		}
		
		if config2 != nil {
			rt.Fatalf("Scenario '%s': Second parse produced non-nil config (non-deterministic)", scenarioDescription)
		}
		
		// Error messages should be consistent
		if err.Error() != err2.Error() {
			rt.Fatalf("Scenario '%s': Error messages differ:\nFirst:  %s\nSecond: %s",
				scenarioDescription, err.Error(), err2.Error())
		}
	})
}

// TestCLIConsistencyFlagAliases tests that short and long flag forms are equivalent
// **Validates: Requirements 7.3**
func TestCLIConsistencyFlagAliases(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tmpDir := t.TempDir()
		
		// Test that -td and --target-directory are equivalent
		useShortForm := rapid.Bool().Draw(rt, "useShortForm")
		
		var args []string
		if useShortForm {
			args = []string{"fast-file-deletion", "-td", tmpDir}
		} else {
			args = []string{"fast-file-deletion", "--target-directory", tmpDir}
		}
		
		// Add some additional flags
		if rapid.Bool().Draw(rt, "addForce") {
			args = append(args, "--force")
		}
		if rapid.Bool().Draw(rt, "addDryRun") {
			args = append(args, "--dry-run")
		}
		
		// Parse with the chosen form
		config := parseArgumentsWithArgs(rt, args)
		
		// Property: Both forms should produce valid config
		if config == nil {
			rt.Fatalf("Failed to parse valid arguments: %v", args)
		}
		
		// Property: Target directory should be parsed correctly regardless of flag form
		if config.TargetDir != tmpDir {
			rt.Fatalf("TargetDir not parsed correctly: expected %s, got %s", tmpDir, config.TargetDir)
		}
		
		// Property: Parse with the opposite form - should produce identical config
		var args2 []string
		if useShortForm {
			args2 = []string{"fast-file-deletion", "--target-directory", tmpDir}
		} else {
			args2 = []string{"fast-file-deletion", "-td", tmpDir}
		}
		
		// Copy the additional flags
		if len(args) > 3 {
			args2 = append(args2, args[3:]...)
		}
		
		config2 := parseArgumentsWithArgs(rt, args2)
		
		// Property: Short and long forms should produce identical configurations
		if !configsEqual(config, config2) {
			rt.Fatalf("Short and long flag forms produce different configs:\nShort form: %+v\nLong form:  %+v",
				config, config2)
		}
	})
}

// TestCLIConsistencyDefaultValues tests that default values are consistent
// **Validates: Requirements 7.3**
func TestCLIConsistencyDefaultValues(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tmpDir := t.TempDir()
		
		// Parse with minimal arguments (only required flags)
		args := []string{"fast-file-deletion", "-td", tmpDir}
		
		config := parseArgumentsWithArgs(rt, args)
		
		// Property: Minimal arguments should produce valid config
		if config == nil {
			rt.Fatalf("Failed to parse minimal valid arguments: %v", args)
		}
		
		// Property: Default values should be consistent
		if config.Force != false {
			rt.Fatalf("Default Force should be false, got %v", config.Force)
		}
		
		if config.DryRun != false {
			rt.Fatalf("Default DryRun should be false, got %v", config.DryRun)
		}
		
		if config.Verbose != false {
			rt.Fatalf("Default Verbose should be false, got %v", config.Verbose)
		}
		
		if config.LogFile != "" {
			rt.Fatalf("Default LogFile should be empty, got %s", config.LogFile)
		}
		
		if config.KeepDays != nil {
			rt.Fatalf("Default KeepDays should be nil, got %d", *config.KeepDays)
		}
		
		if config.Workers != 0 {
			rt.Fatalf("Default Workers should be 0 (auto), got %d", config.Workers)
		}
		
		if config.BufferSize != 0 {
			rt.Fatalf("Default BufferSize should be 0 (auto), got %d", config.BufferSize)
		}
		
		if config.DeletionMethod != "auto" {
			rt.Fatalf("Default DeletionMethod should be 'auto', got %s", config.DeletionMethod)
		}
		
		if config.Benchmark != false {
			rt.Fatalf("Default Benchmark should be false, got %v", config.Benchmark)
		}
		
		// Property: Parse again - should produce identical defaults
		config2 := parseArgumentsWithArgs(rt, args)
		
		if !configsEqual(config, config2) {
			rt.Fatalf("Default values are non-deterministic:\nFirst:  %+v\nSecond: %+v",
				config, config2)
		}
	})
}

// Helper function to parse arguments with custom args array
func parseArgumentsWithArgs(rt *rapid.T, args []string) *Config {
	// Save and restore original args
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	
	// Reset flag package state
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	
	// Set test arguments
	os.Args = args
	
	// Parse arguments
	config, err := parseArguments()
	if err != nil {
		rt.Fatalf("Failed to parse valid arguments: %v\nArgs: %v", err, args)
	}
	
	if config == nil {
		rt.Fatalf("parseArguments returned nil config for valid arguments: %v", args)
	}
	
	return config
}

// Helper function to compare two configs for equality
func configsEqual(c1, c2 *Config) bool {
	if c1 == nil && c2 == nil {
		return true
	}
	if c1 == nil || c2 == nil {
		return false
	}
	
	// Compare all fields
	if c1.TargetDir != c2.TargetDir {
		return false
	}
	if c1.Force != c2.Force {
		return false
	}
	if c1.DryRun != c2.DryRun {
		return false
	}
	if c1.Verbose != c2.Verbose {
		return false
	}
	if c1.LogFile != c2.LogFile {
		return false
	}
	if !intPtrEqual(c1.KeepDays, c2.KeepDays) {
		return false
	}
	if c1.Workers != c2.Workers {
		return false
	}
	if c1.BufferSize != c2.BufferSize {
		return false
	}
	if c1.DeletionMethod != c2.DeletionMethod {
		return false
	}
	if c1.Benchmark != c2.Benchmark {
		return false
	}
	
	return true
}

// Helper function to shuffle arguments while preserving flag-value pairs
func shuffleArgs(rt *rapid.T, args []string) []string {
	// Keep the program name (args[0]) and target directory flag at the start
	// Shuffle the remaining flags
	
	if len(args) <= 3 {
		return args
	}
	
	// Extract program name and target directory
	result := []string{args[0], args[1], args[2]} // "program", "-td", "/path"
	
	// Extract remaining flags as pairs or singles
	remaining := args[3:]
	var flags [][]string
	
	for i := 0; i < len(remaining); i++ {
		if remaining[i][0] == '-' {
			// This is a flag
			if i+1 < len(remaining) && remaining[i+1][0] != '-' {
				// Flag with value
				flags = append(flags, []string{remaining[i], remaining[i+1]})
				i++ // Skip the value
			} else {
				// Boolean flag
				flags = append(flags, []string{remaining[i]})
			}
		}
	}
	
	// Shuffle the flags
	if len(flags) > 1 {
		for i := len(flags) - 1; i > 0; i-- {
			j := rapid.IntRange(0, i).Draw(rt, "shuffleIdx")
			flags[i], flags[j] = flags[j], flags[i]
		}
	}
	
	// Reconstruct the args
	for _, flag := range flags {
		result = append(result, flag...)
	}
	
	return result
}
