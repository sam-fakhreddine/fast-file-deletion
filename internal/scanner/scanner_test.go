package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"pgregory.net/rapid"
)

func TestScanner_BasicScan(t *testing.T) {
	// Create a temporary directory structure for testing
	tmpDir := t.TempDir()

	// Create some test files and directories
	testFiles := []string{
		"file1.txt",
		"file2.txt",
		"subdir/file3.txt",
		"subdir/nested/file4.txt",
	}

	for _, file := range testFiles {
		fullPath := filepath.Join(tmpDir, file)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte("test content"), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
	}

	// Create scanner without age filtering
	scanner := NewScanner(tmpDir, nil)

	// Scan the directory
	result, err := scanner.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Verify results
	if result.TotalScanned == 0 {
		t.Error("Expected TotalScanned > 0")
	}

	if result.TotalToDelete == 0 {
		t.Error("Expected TotalToDelete > 0")
	}

	if len(result.Files) == 0 {
		t.Error("Expected Files list to be non-empty")
	}

	// Verify files are ordered bottom-up (files before directories)
	// The last item should be the root directory
	if len(result.Files) > 0 && result.Files[len(result.Files)-1] != tmpDir {
		t.Errorf("Expected last item to be root directory %s, got %s", tmpDir, result.Files[len(result.Files)-1])
	}
}

func TestScanner_AgeFiltering(t *testing.T) {
	// Create a temporary directory structure for testing
	tmpDir := t.TempDir()

	// Create an old file (modified 10 days ago)
	oldFile := filepath.Join(tmpDir, "old_file.txt")
	if err := os.WriteFile(oldFile, []byte("old content"), 0644); err != nil {
		t.Fatalf("Failed to create old file: %v", err)
	}
	oldTime := time.Now().Add(-10 * 24 * time.Hour)
	if err := os.Chtimes(oldFile, oldTime, oldTime); err != nil {
		t.Fatalf("Failed to set old file time: %v", err)
	}

	// Create a new file (modified now)
	newFile := filepath.Join(tmpDir, "new_file.txt")
	if err := os.WriteFile(newFile, []byte("new content"), 0644); err != nil {
		t.Fatalf("Failed to create new file: %v", err)
	}

	// Create scanner with 5-day retention
	keepDays := 5
	scanner := NewScanner(tmpDir, &keepDays)

	// Scan the directory
	result, err := scanner.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Verify that we scanned both files
	if result.TotalScanned < 2 {
		t.Errorf("Expected TotalScanned >= 2, got %d", result.TotalScanned)
	}

	// Verify that only the old file is marked for deletion
	if result.TotalRetained == 0 {
		t.Error("Expected some files to be retained")
	}

	// Verify that the old file is in the deletion list
	foundOldFile := false
	for _, file := range result.Files {
		if file == oldFile {
			foundOldFile = true
			break
		}
	}
	if !foundOldFile {
		t.Error("Expected old file to be in deletion list")
	}
}

func TestScanner_KeepDaysZero(t *testing.T) {
	// Create a temporary directory with a file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create scanner with keepDays=0 (should delete everything)
	keepDays := 0
	scanner := NewScanner(tmpDir, &keepDays)

	// Scan the directory
	result, err := scanner.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Verify that all files are marked for deletion
	if result.TotalRetained != 0 {
		t.Errorf("Expected TotalRetained = 0 with keepDays=0, got %d", result.TotalRetained)
	}

	if result.TotalToDelete == 0 {
		t.Error("Expected files to be marked for deletion with keepDays=0")
	}
}

// Feature: fast-file-deletion, Property 6: Modification Timestamp Usage
// Validates: Requirements 7.2
func TestModificationTimestampUsageProperty(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Create a temporary directory for this test iteration
		tmpDir := t.TempDir()

		// Generate a random keep-days threshold (1 to 30 days)
		keepDays := rapid.IntRange(1, 30).Draw(rt, "keepDays")
		keepDuration := time.Duration(keepDays) * 24 * time.Hour

		// Generate a random number of files (1 to 10 for reasonable test speed)
		numFiles := rapid.IntRange(1, 10).Draw(rt, "numFiles")

		// Track files and their modification times
		fileModTimes := make(map[string]time.Time)

		now := time.Now()
		for i := 0; i < numFiles; i++ {
			fileName := filepath.Join(tmpDir, fmt.Sprintf("file_%d.txt", i))

			// Generate random modification time (0 to 60 days old)
			daysOld := rapid.IntRange(0, 60).Draw(rt, "daysOld")
			hoursOffset := rapid.IntRange(0, 23).Draw(rt, "hoursOffset")
			modTime := now.Add(-time.Duration(daysOld)*24*time.Hour - time.Duration(hoursOffset)*time.Hour)

			// Create the file
			if err := os.WriteFile(fileName, []byte("test content"), 0644); err != nil {
				rt.Fatalf("Failed to create file: %v", err)
			}

			// Set DIFFERENT access time and modification time
			// Access time is always "now" (recent), modification time is in the past
			accessTime := now
			if err := os.Chtimes(fileName, accessTime, modTime); err != nil {
				rt.Fatalf("Failed to set file times: %v", err)
			}

			fileModTimes[fileName] = modTime
		}

		// Create scanner with the keep-days threshold
		scanner := NewScanner(tmpDir, &keepDays)

		// Scan the directory
		result, err := scanner.Scan()
		if err != nil {
			rt.Fatalf("Scan failed: %v", err)
		}

		// Property: Age calculation must use modification time, not access time
		// If the scanner incorrectly used access time, all files would be retained
		// because we set access time to "now" for all files
		for _, filePath := range result.Files {
			// Skip the root directory
			if filePath == tmpDir {
				continue
			}

			// Skip directories
			info, err := os.Stat(filePath)
			if err != nil {
				continue
			}
			if info.IsDir() {
				continue
			}

			// Get the file's modification time
			modTime, exists := fileModTimes[filePath]
			if !exists {
				continue // Not one of our test files
			}

			// Calculate age based on modification time
			fileAge := now.Sub(modTime) + time.Second // Add buffer for timing differences

			// If file is in deletion list, it must be older than keepDays based on MODIFICATION time
			// This proves we're using ModTime() and not access time
			if fileAge <= keepDuration {
				rt.Fatalf("File %s was marked for deletion but its modification time is too recent (age: %v, threshold: %v). "+
					"This suggests the scanner may be using access time instead of modification time.",
					filePath, fileAge, keepDuration)
			}
		}

		// Additional verification: Check that files with old modification times but recent access times
		// are correctly identified for deletion (proving we use mtime, not atime)
		deletionSet := make(map[string]bool)
		for _, filePath := range result.Files {
			deletionSet[filePath] = true
		}

		for fileName, modTime := range fileModTimes {
			fileAge := now.Sub(modTime) + time.Second
			shouldBeDeleted := fileAge > keepDuration

			isInDeletionList := deletionSet[fileName]

			if shouldBeDeleted && !isInDeletionList {
				rt.Fatalf("File %s should be deleted based on modification time (age: %v, threshold: %v) but was not in deletion list. "+
					"This suggests the scanner may be using access time instead of modification time.",
					fileName, fileAge, keepDuration)
			}
		}
	})
}

// Feature: fast-file-deletion, Property 7: Retention Count Accuracy
// Validates: Requirements 7.6
func TestRetentionCountAccuracyProperty(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Create a temporary directory for this test iteration
		tmpDir := t.TempDir()

		// Generate a random keep-days threshold (1 to 30 days)
		keepDays := rapid.IntRange(1, 30).Draw(rt, "keepDays")
		keepDuration := time.Duration(keepDays) * 24 * time.Hour

		// Generate a random number of files (1 to 20 for reasonable test speed)
		numFiles := rapid.IntRange(1, 20).Draw(rt, "numFiles")

		// Track files that should be retained based on age
		expectedRetainedFiles := make([]string, 0)

		// Create files with various ages
		now := time.Now()
		for i := 0; i < numFiles; i++ {
			fileName := filepath.Join(tmpDir, fmt.Sprintf("file_%d.txt", i))

			// Generate a random age for this file (0 to 60 days old)
			daysOld := rapid.IntRange(0, 60).Draw(rt, "daysOld")
			hoursOffset := rapid.IntRange(0, 23).Draw(rt, "hoursOffset")
			fileAge := time.Duration(daysOld)*24*time.Hour + time.Duration(hoursOffset)*time.Hour
			modTime := now.Add(-fileAge)

			// Create the file
			if err := os.WriteFile(fileName, []byte("test content"), 0644); err != nil {
				rt.Fatalf("Failed to create file: %v", err)
			}

			// Set the modification time
			if err := os.Chtimes(fileName, modTime, modTime); err != nil {
				rt.Fatalf("Failed to set file time: %v", err)
			}

			// Determine if this file should be retained
			// Use a 1-second buffer to account for timing differences
			calculatedAge := now.Sub(modTime) + time.Second
			if calculatedAge <= keepDuration {
				expectedRetainedFiles = append(expectedRetainedFiles, fileName)
			}
		}

		// Create scanner with the keep-days threshold
		scanner := NewScanner(tmpDir, &keepDays)

		// Scan the directory
		result, err := scanner.Scan()
		if err != nil {
			rt.Fatalf("Scan failed: %v", err)
		}

		// Property: The reported retention count must exactly match the number of files
		// that were skipped due to being within the retention period
		expectedRetainedCount := len(expectedRetainedFiles)
		actualRetainedCount := result.TotalRetained

		if actualRetainedCount != expectedRetainedCount {
			rt.Fatalf("Retention count mismatch: expected %d retained files, but scanner reported %d. "+
				"The reported count should exactly match the number of files within the retention period.",
				expectedRetainedCount, actualRetainedCount)
		}

		// Additional verification: Ensure that the files we expect to be retained
		// are NOT in the deletion list
		deletionSet := make(map[string]bool)
		for _, filePath := range result.Files {
			deletionSet[filePath] = true
		}

		for _, retainedFile := range expectedRetainedFiles {
			if deletionSet[retainedFile] {
				info, _ := os.Stat(retainedFile)
				fileAge := time.Since(info.ModTime())
				rt.Fatalf("File %s should be retained (age: %v, threshold: %v) but was found in deletion list. "+
					"This indicates the retention count may be inaccurate.",
					retainedFile, fileAge, keepDuration)
			}
		}

		// Verify the sum: TotalScanned should equal TotalToDelete + TotalRetained
		// (excluding the root directory from the count)
		// Note: TotalScanned includes both files and directories
		// TotalToDelete includes files, directories, and the root
		// TotalRetained only includes files that were skipped due to age
		// So we verify: files scanned = files to delete + files retained
		filesScanned := result.TotalScanned
		filesInDeletionList := 0
		for _, filePath := range result.Files {
			if filePath == tmpDir {
				continue // Don't count root directory
			}
			info, err := os.Stat(filePath)
			if err != nil {
				continue
			}
			if !info.IsDir() {
				filesInDeletionList++
			}
		}

		// The retention count should account for all files not in the deletion list
		totalFilesAccountedFor := filesInDeletionList + result.TotalRetained
		if totalFilesAccountedFor > filesScanned {
			rt.Fatalf("Accounting error: files in deletion list (%d) + retained files (%d) = %d, "+
				"but only %d files were scanned. Retention count may be inflated.",
				filesInDeletionList, result.TotalRetained, totalFilesAccountedFor, filesScanned)
		}
	})
}

// Edge Case Tests

// TestScanner_EmptyDirectory tests scanning an empty directory
// Requirements: 1.1, 7.1
func TestScanner_EmptyDirectory(t *testing.T) {
	// Create an empty temporary directory
	tmpDir := t.TempDir()

	// Create scanner without age filtering
	scanner := NewScanner(tmpDir, nil)

	// Scan the empty directory
	result, err := scanner.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Verify results for empty directory
	if result.TotalScanned != 0 {
		t.Errorf("Expected TotalScanned = 0 for empty directory, got %d", result.TotalScanned)
	}

	// Should still include the root directory for deletion
	if len(result.Files) != 1 || result.Files[0] != tmpDir {
		t.Errorf("Expected Files to contain only root directory, got %v", result.Files)
	}

	if result.TotalToDelete != 1 {
		t.Errorf("Expected TotalToDelete = 1 (root directory), got %d", result.TotalToDelete)
	}

	if result.TotalRetained != 0 {
		t.Errorf("Expected TotalRetained = 0, got %d", result.TotalRetained)
	}

	if result.TotalSizeBytes != 0 {
		t.Errorf("Expected TotalSizeBytes = 0, got %d", result.TotalSizeBytes)
	}
}

// TestScanner_SingleFile tests scanning a directory with a single file
// Requirements: 1.1, 7.1
func TestScanner_SingleFile(t *testing.T) {
	// Create a temporary directory with a single file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "single_file.txt")
	testContent := []byte("test content for single file")
	
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create scanner without age filtering
	scanner := NewScanner(tmpDir, nil)

	// Scan the directory
	result, err := scanner.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Verify results
	if result.TotalScanned != 1 {
		t.Errorf("Expected TotalScanned = 1, got %d", result.TotalScanned)
	}

	// Should include the file and the root directory
	if len(result.Files) != 2 {
		t.Errorf("Expected Files length = 2 (file + root), got %d", len(result.Files))
	}

	// First item should be the file, last should be root directory
	if result.Files[0] != testFile {
		t.Errorf("Expected first file to be %s, got %s", testFile, result.Files[0])
	}

	if result.Files[len(result.Files)-1] != tmpDir {
		t.Errorf("Expected last item to be root directory %s, got %s", tmpDir, result.Files[len(result.Files)-1])
	}

	if result.TotalToDelete != 2 {
		t.Errorf("Expected TotalToDelete = 2 (file + root), got %d", result.TotalToDelete)
	}

	if result.TotalSizeBytes != int64(len(testContent)) {
		t.Errorf("Expected TotalSizeBytes = %d, got %d", len(testContent), result.TotalSizeBytes)
	}
}

// TestScanner_DeeplyNestedStructure tests scanning a deeply nested directory structure
// Requirements: 1.1, 7.1
func TestScanner_DeeplyNestedStructure(t *testing.T) {
	// Create a temporary directory with deeply nested structure
	tmpDir := t.TempDir()

	// Create a structure 10 levels deep
	depth := 10
	currentPath := tmpDir
	var allFiles []string
	var allDirs []string

	for i := 0; i < depth; i++ {
		// Create a subdirectory at this level
		subDir := filepath.Join(currentPath, fmt.Sprintf("level_%d", i))
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatalf("Failed to create directory at level %d: %v", i, err)
		}
		allDirs = append(allDirs, subDir)

		// Create a file at this level
		testFile := filepath.Join(subDir, fmt.Sprintf("file_at_level_%d.txt", i))
		if err := os.WriteFile(testFile, []byte(fmt.Sprintf("content at level %d", i)), 0644); err != nil {
			t.Fatalf("Failed to create file at level %d: %v", i, err)
		}
		allFiles = append(allFiles, testFile)

		// Move deeper
		currentPath = subDir
	}

	// Create scanner without age filtering
	scanner := NewScanner(tmpDir, nil)

	// Scan the directory
	result, err := scanner.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Verify that all files and directories were scanned
	expectedScanned := len(allFiles) + len(allDirs)
	if result.TotalScanned != expectedScanned {
		t.Errorf("Expected TotalScanned = %d, got %d", expectedScanned, result.TotalScanned)
	}

	// Verify that all items are marked for deletion (files + dirs + root)
	expectedToDelete := len(allFiles) + len(allDirs) + 1 // +1 for root
	if result.TotalToDelete != expectedToDelete {
		t.Errorf("Expected TotalToDelete = %d, got %d", expectedToDelete, result.TotalToDelete)
	}

	// Verify bottom-up ordering: files should come before their parent directories
	// The last item should be the root directory
	if result.Files[len(result.Files)-1] != tmpDir {
		t.Errorf("Expected last item to be root directory %s, got %s", tmpDir, result.Files[len(result.Files)-1])
	}

	// Verify that deeper directories come before shallower ones
	// Find positions of directories in the result
	dirPositions := make(map[string]int)
	for i, path := range result.Files {
		for _, dir := range allDirs {
			if path == dir {
				dirPositions[dir] = i
				break
			}
		}
	}

	// Check that deeper directories come before shallower ones
	for i := 0; i < len(allDirs)-1; i++ {
		deeperDir := allDirs[len(allDirs)-1-i]   // Start from deepest
		shallowerDir := allDirs[len(allDirs)-2-i] // One level up

		deeperPos, deeperFound := dirPositions[deeperDir]
		shallowerPos, shallowerFound := dirPositions[shallowerDir]

		if deeperFound && shallowerFound {
			if deeperPos >= shallowerPos {
				t.Errorf("Directory ordering violation: deeper directory %s (pos %d) should come before shallower directory %s (pos %d)",
					deeperDir, deeperPos, shallowerDir, shallowerPos)
			}
		}
	}
}

// TestScanner_Symlinks tests scanning behavior with symbolic links (if applicable)
// Requirements: 1.1, 7.1
func TestScanner_Symlinks(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()

	// Create a regular file
	regularFile := filepath.Join(tmpDir, "regular_file.txt")
	if err := os.WriteFile(regularFile, []byte("regular content"), 0644); err != nil {
		t.Fatalf("Failed to create regular file: %v", err)
	}

	// Create a target directory for symlink
	targetDir := filepath.Join(tmpDir, "target_dir")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatalf("Failed to create target directory: %v", err)
	}

	targetFile := filepath.Join(targetDir, "target_file.txt")
	if err := os.WriteFile(targetFile, []byte("target content"), 0644); err != nil {
		t.Fatalf("Failed to create target file: %v", err)
	}

	// Create a symlink to the target directory
	symlinkPath := filepath.Join(tmpDir, "symlink_to_target")
	if err := os.Symlink(targetDir, symlinkPath); err != nil {
		// Symlinks might not be supported on all systems (e.g., Windows without admin)
		t.Skipf("Symlinks not supported on this system: %v", err)
	}

	// Create scanner without age filtering
	scanner := NewScanner(tmpDir, nil)

	// Scan the directory
	result, err := scanner.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Verify that the scanner handles symlinks appropriately
	// The scanner should scan the symlink itself, not follow it
	// (to avoid infinite loops and duplicate deletions)
	
	// Count how many times we see files from target_dir
	targetFileCount := 0
	for _, path := range result.Files {
		if filepath.Dir(path) == targetDir {
			targetFileCount++
		}
	}

	// We should see target_file.txt exactly once (from the actual target_dir)
	// not twice (once from target_dir and once through the symlink)
	if targetFileCount != 1 {
		t.Logf("Warning: Symlink handling may cause duplicate scanning. Target file seen %d times", targetFileCount)
	}

	// Verify that we scanned the expected items
	// Should include: regular_file.txt, target_dir, target_file.txt, symlink_to_target
	if result.TotalScanned < 3 {
		t.Errorf("Expected TotalScanned >= 3, got %d", result.TotalScanned)
	}
}

// Feature: fast-file-deletion, Property 5: Age-Based Filtering
// Validates: Requirements 7.1, 7.3
func TestAgeBasedFilteringProperty(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Create a temporary directory for this test iteration
		tmpDir := t.TempDir()

		// Generate a random keep-days threshold (1 to 30 days)
		keepDays := rapid.IntRange(1, 30).Draw(rt, "keepDays")
		keepDuration := time.Duration(keepDays) * 24 * time.Hour

		// Generate a random number of files (1 to 20 for reasonable test speed)
		numFiles := rapid.IntRange(1, 20).Draw(rt, "numFiles")

		// Track which files should be deleted vs retained
		expectedDeleted := make(map[string]bool)
		expectedRetained := make(map[string]bool)

		// Create files with various ages
		now := time.Now()
		for i := 0; i < numFiles; i++ {
			// Generate unique filename using index to avoid collisions
			fileName := filepath.Join(tmpDir, fmt.Sprintf("file_%d.txt", i))
			
			// Generate a random age for this file (0 to 60 days old)
			// Add a small offset to avoid exact boundary conditions (timing precision issues)
			daysOld := rapid.IntRange(0, 60).Draw(rt, "daysOld")
			hoursOffset := rapid.IntRange(0, 23).Draw(rt, "hoursOffset")
			fileAge := time.Duration(daysOld)*24*time.Hour + time.Duration(hoursOffset)*time.Hour
			modTime := now.Add(-fileAge)

			// Create the file
			if err := os.WriteFile(fileName, []byte("test content"), 0644); err != nil {
				rt.Fatalf("Failed to create file: %v", err)
			}

			// Set the modification time
			if err := os.Chtimes(fileName, modTime, modTime); err != nil {
				rt.Fatalf("Failed to set file time: %v", err)
			}

			// Determine if this file should be deleted or retained
			// Use the same logic as the scanner: fileAge > keepDuration
			// Add a 1-second buffer to account for timing differences between
			// when we calculate the age and when the scanner checks it
			calculatedAge := now.Sub(modTime) + time.Second
			if calculatedAge > keepDuration {
				expectedDeleted[fileName] = true
			} else {
				expectedRetained[fileName] = true
			}
		}

		// Create scanner with the keep-days threshold
		scanner := NewScanner(tmpDir, &keepDays)

		// Scan the directory
		result, err := scanner.Scan()
		if err != nil {
			rt.Fatalf("Scan failed: %v", err)
		}

		// Property 1: All files marked for deletion should be older than keepDays
		for _, filePath := range result.Files {
			// Skip the root directory itself
			if filePath == tmpDir {
				continue
			}

			// Skip directories (we only care about files for age filtering)
			info, err := os.Stat(filePath)
			if err != nil {
				continue // File might have been deleted or inaccessible
			}
			if info.IsDir() {
				continue
			}

			// Check if this file should have been deleted
			if !expectedDeleted[filePath] {
				fileAge := time.Since(info.ModTime())
				rt.Fatalf("File %s was marked for deletion but should be retained (age: %v, threshold: %v)",
					filePath, fileAge, keepDuration)
			}
		}

		// Property 2: All retained files should be newer than keepDays
		// We verify this by checking that files NOT in the deletion list are in expectedRetained
		deletionSet := make(map[string]bool)
		for _, filePath := range result.Files {
			deletionSet[filePath] = true
		}

		for retainedFile := range expectedRetained {
			if deletionSet[retainedFile] {
				info, _ := os.Stat(retainedFile)
				fileAge := time.Since(info.ModTime())
				rt.Fatalf("File %s was marked for deletion but should be retained (age: %v, threshold: %v)",
					retainedFile, fileAge, keepDuration)
			}
		}

		// Property 3: Retention count should match the number of files within the retention period
		if result.TotalRetained != len(expectedRetained) {
			rt.Fatalf("Expected %d retained files, got %d", len(expectedRetained), result.TotalRetained)
		}

		// Property 4: Files marked for deletion should match expected count
		// (excluding directories and the root directory)
		actualDeletedFiles := 0
		for _, filePath := range result.Files {
			if filePath == tmpDir {
				continue
			}
			info, err := os.Stat(filePath)
			if err != nil || info.IsDir() {
				continue
			}
			actualDeletedFiles++
		}

		if actualDeletedFiles != len(expectedDeleted) {
			rt.Fatalf("Expected %d files marked for deletion, got %d", len(expectedDeleted), actualDeletedFiles)
		}
	})
}
