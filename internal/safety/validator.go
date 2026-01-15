// Package safety provides path validation and user confirmation functionality
// to prevent accidental deletion of system-critical directories.
package safety

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/yourusername/fast-file-deletion/internal/logger"
)

// ProtectedPaths contains system-critical paths that should never be deleted.
// These paths are checked during safety validation to prevent accidental
// deletion of operating system files and critical system directories.
// The list includes common Windows and Unix system directories.
var ProtectedPaths = []string{
	"C:\\Windows",
	"C:\\Program Files",
	"C:\\Program Files (x86)",
	"C:\\ProgramData",
	"C:\\Users",
	"C:\\System Volume Information",
	"/bin",
	"/sbin",
	"/usr",
	"/lib",
	"/lib64",
	"/etc",
	"/boot",
	"/sys",
	"/proc",
	"/dev",
}

// IsSafePath checks if a path is safe to delete by validating it against
// system-critical directories and checking for proper permissions.
// It performs the following checks:
//   - Resolves the path to an absolute path
//   - Verifies the path exists and is a directory
//   - Checks if the path is a drive root (requires special confirmation)
//   - Validates the path is not in the ProtectedPaths list
//   - Ensures the path is not a parent of any protected path
//   - Verifies write permissions on the parent directory
//
// Returns (isSafe, reason) where reason explains why the path is unsafe.
func IsSafePath(path string) (bool, string) {
	// Clean and normalize the path
	cleanPath := filepath.Clean(path)
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		logger.Warning("Cannot resolve absolute path for: %s (error: %v)", path, err)
		return false, fmt.Sprintf("cannot resolve absolute path: %v", err)
	}

	logger.Debug("Validating path safety: %s", absPath)

	// Check if path exists
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Warning("Path does not exist: %s", absPath)
			return false, "path does not exist"
		}
		logger.Warning("Cannot access path: %s (error: %v)", absPath, err)
		return false, fmt.Sprintf("cannot access path: %v", err)
	}

	// Ensure it's a directory
	if !info.IsDir() {
		logger.Warning("Path is not a directory: %s", absPath)
		return false, "path is not a directory"
	}

	// Check if path is a drive root
	if isDriveRoot(absPath) {
		logger.Warning("Path is a drive root: %s", absPath)
		return false, "cannot delete drive root (requires additional confirmation)"
	}

	// Check against protected paths
	for _, protected := range ProtectedPaths {
		protectedAbs, err := filepath.Abs(protected)
		if err != nil {
			continue
		}

		// Check if the path matches or is a subdirectory of a protected path
		if strings.EqualFold(absPath, protectedAbs) {
			logger.Warning("Path is protected system directory: %s", absPath)
			return false, fmt.Sprintf("path is protected system directory: %s", protected)
		}

		// Check if it's a parent of a protected path (even more dangerous)
		if isParentOf(absPath, protectedAbs) {
			logger.Warning("Path contains protected system directory: %s (contains %s)", absPath, protected)
			return false, fmt.Sprintf("path contains protected system directory: %s", protected)
		}
	}

	// Check write permissions on parent directory
	parentDir := filepath.Dir(absPath)
	if !hasWritePermission(parentDir) {
		logger.Warning("Insufficient permissions for: %s (parent not writable)", absPath)
		return false, "insufficient permissions to delete (parent directory not writable)"
	}

	logger.Debug("Path is safe to delete: %s", absPath)
	return true, ""
}

// isDriveRoot checks if a path is a drive root (e.g., C:\, D:\, /).
// Drive roots require special handling and additional confirmation
// because deleting them would remove all data on the drive.
// On Windows, it checks for patterns like "C:\", "D:\", or "C:".
// On Unix systems, it checks for the root directory "/".
func isDriveRoot(path string) bool {
	cleanPath := filepath.Clean(path)

	if runtime.GOOS == "windows" {
		// Windows drive root: C:\, D:\, etc.
		// After Clean(), drive roots look like "C:\" or "C:/"
		if len(cleanPath) == 3 && cleanPath[1] == ':' && (cleanPath[2] == '\\' || cleanPath[2] == '/') {
			return true
		}
		// Also check for just "C:" which Clean() might produce
		if len(cleanPath) == 2 && cleanPath[1] == ':' {
			return true
		}
	} else {
		// Unix root: /
		if cleanPath == "/" {
			return true
		}
	}

	return false
}

// isParentOf checks if parent is a parent directory of child.
// This is used to prevent deletion of directories that contain
// system-critical subdirectories. For example, deleting C:\ would
// be dangerous because it contains C:\Windows.
// The comparison is case-insensitive on Windows and case-sensitive on Unix.
func isParentOf(parent, child string) bool {
	// Normalize paths for comparison
	parent = filepath.Clean(parent)
	child = filepath.Clean(child)

	// Make sure both end with separator for accurate comparison
	if !strings.HasSuffix(parent, string(filepath.Separator)) {
		parent += string(filepath.Separator)
	}

	// Check if child starts with parent path
	if runtime.GOOS == "windows" {
		return strings.HasPrefix(strings.ToLower(child), strings.ToLower(parent))
	}
	return strings.HasPrefix(child, parent)
}

// hasWritePermission checks if the current user has write permission on a directory.
// This is done by attempting to create a temporary test file in the directory.
// If the file can be created and deleted, the user has write permission.
// This is necessary to ensure the deletion operation will succeed.
func hasWritePermission(path string) bool {
	// Try to create a temporary file in the directory
	testFile := filepath.Join(path, ".write_test_"+fmt.Sprintf("%d", os.Getpid()))
	f, err := os.Create(testFile)
	if err != nil {
		return false
	}
	f.Close()
	os.Remove(testFile)
	return true
}

// GetUserConfirmation prompts the user for deletion confirmation.
// It displays a formatted confirmation dialog showing:
//   - The absolute path to be deleted
//   - The number of files and directories to be deleted
//   - A warning that the action cannot be undone (unless in dry-run mode)
//   - A prompt requiring the user to type the exact path to confirm
//
// The function performs path matching validation to ensure the user
// typed the correct path, preventing typos from causing accidental deletions.
//
// Parameters:
//   - path: The directory path to delete
//   - fileCount: Number of files and directories to be deleted
//   - dryRun: If true, indicates this is a simulation (no actual deletion)
//   - force: If true, skips confirmation and returns true immediately
//
// Returns true if user confirms, false otherwise.
func GetUserConfirmation(path string, fileCount int, dryRun bool, force bool) bool {
	// Skip confirmation if force flag is enabled
	if force {
		logger.Info("Force flag enabled, skipping confirmation")
		return true
	}

	logger.Debug("Requesting user confirmation for: %s", path)

	// Display confirmation prompt
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	fmt.Println()
	fmt.Println("╔════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                    DELETION CONFIRMATION                       ║")
	fmt.Println("╚════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	if dryRun {
		fmt.Printf("DRY RUN MODE: Simulating deletion of:\n")
	} else {
		fmt.Printf("⚠️  WARNING: You are about to permanently delete:\n")
	}

	fmt.Printf("   Path: %s\n", absPath)
	if fileCount > 0 {
		fmt.Printf("   Files: %d files and directories\n", fileCount)
	}
	fmt.Println()

	// Special warning for drive roots
	if isDriveRoot(absPath) {
		fmt.Println("⚠️  CRITICAL WARNING: This is a drive root!")
		fmt.Println("   Deleting this will remove ALL data on the drive.")
		fmt.Println()
	}

	if !dryRun {
		fmt.Println("This action CANNOT be undone!")
		fmt.Println()
	}

	// Prompt for path confirmation
	fmt.Printf("To confirm, please type the full path exactly as shown above:\n")
	fmt.Print("> ")

	// Use bufio.Reader to read the entire line including spaces
	reader := bufio.NewReader(os.Stdin)
	userInput, err := reader.ReadString('\n')
	if err != nil {
		logger.Warning("Failed to read user input: %v", err)
		return false
	}

	// Trim whitespace and newline characters (handles both \n and \r\n)
	userInput = strings.TrimSpace(userInput)

	// Validate that the user typed the exact path
	userInputAbs, err := filepath.Abs(userInput)
	if err != nil {
		userInputAbs = userInput
	}

	// Compare paths (case-insensitive on Windows, case-sensitive on Unix)
	if !pathsMatch(absPath, userInputAbs) {
		fmt.Println()
		fmt.Println("❌ Path mismatch. Deletion cancelled.")
		logger.Info("User confirmation failed: path mismatch")
		return false
	}

	fmt.Println()
	if dryRun {
		fmt.Println("✓ Confirmed. Starting dry run...")
		logger.Info("User confirmed dry run for: %s", absPath)
	} else {
		fmt.Println("✓ Confirmed. Starting deletion...")
		logger.Info("User confirmed deletion for: %s", absPath)
	}

	return true
}

// pathsMatch compares two paths for equality, respecting OS conventions.
// On Windows, the comparison is case-insensitive (C:\Path == c:\path).
// On Unix systems, the comparison is case-sensitive (/Path != /path).
// Both paths are cleaned using filepath.Clean before comparison.
func pathsMatch(path1, path2 string) bool {
	clean1 := filepath.Clean(path1)
	clean2 := filepath.Clean(path2)

	if runtime.GOOS == "windows" {
		// Case-insensitive comparison on Windows
		return strings.EqualFold(clean1, clean2)
	}
	// Case-sensitive comparison on Unix
	return clean1 == clean2
}
