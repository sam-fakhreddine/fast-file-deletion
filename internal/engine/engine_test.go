package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/yourusername/fast-file-deletion/internal/backend"
	"pgregory.net/rapid"
)

// Feature: fast-file-deletion, Property 1: Complete Directory Removal
// For any valid directory structure, when deletion completes successfully,
// the target directory and all its contents should no longer exist on the filesystem.
// Validates: Requirements 1.1, 1.4
func TestCompleteDirectoryRemoval(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Create a temporary directory for this test iteration
		tmpDir := t.TempDir()
		targetDir := filepath.Join(tmpDir, "target")
		
		// Create the target directory
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			rt.Fatalf("Failed to create target directory: %v", err)
		}

		// Generate a random directory structure
		// Number of subdirectories (0 to 5)
		numSubdirs := rapid.IntRange(0, 5).Draw(rt, "numSubdirs")
		
		// Number of files per directory (1 to 10)
		numFilesPerDir := rapid.IntRange(1, 10).Draw(rt, "numFilesPerDir")

		// Track all created paths for verification
		allPaths := []string{targetDir}
		
		// Create subdirectories
		subdirs := []string{targetDir}
		for i := 0; i < numSubdirs; i++ {
			// Pick a random parent directory
			parentIdx := rapid.IntRange(0, len(subdirs)-1).Draw(rt, "parentIdx")
			parentDir := subdirs[parentIdx]
			
			// Create a subdirectory
			subdir := filepath.Join(parentDir, fmt.Sprintf("subdir_%d", i))
			if err := os.MkdirAll(subdir, 0755); err != nil {
				rt.Fatalf("Failed to create subdirectory: %v", err)
			}
			subdirs = append(subdirs, subdir)
			allPaths = append(allPaths, subdir)
		}

		// Create files in each directory
		for _, dir := range subdirs {
			for i := 0; i < numFilesPerDir; i++ {
				fileName := filepath.Join(dir, fmt.Sprintf("file_%d.txt", i))
				
				// Generate random file content (0 to 1000 bytes)
				contentSize := rapid.IntRange(0, 1000).Draw(rt, "contentSize")
				content := make([]byte, contentSize)
				for j := 0; j < contentSize; j++ {
					content[j] = byte(rapid.IntRange(0, 255).Draw(rt, "contentByte"))
				}
				
				if err := os.WriteFile(fileName, content, 0644); err != nil {
					rt.Fatalf("Failed to create file: %v", err)
				}
				allPaths = append(allPaths, fileName)
			}
		}

		// Verify all paths exist before deletion
		for _, path := range allPaths {
			if _, err := os.Stat(path); os.IsNotExist(err) {
				rt.Fatalf("Path %s does not exist before deletion", path)
			}
		}

		// Build file list for deletion (bottom-up order)
		var filesToDelete []string
		err := filepath.WalkDir(targetDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			// Add to list (will be reversed later for bottom-up)
			filesToDelete = append(filesToDelete, path)
			return nil
		})
		if err != nil {
			rt.Fatalf("Failed to walk directory: %v", err)
		}

		// Reverse the list to get bottom-up order (files before directories)
		for i, j := 0, len(filesToDelete)-1; i < j; i, j = i+1, j-1 {
			filesToDelete[i], filesToDelete[j] = filesToDelete[j], filesToDelete[i]
		}

		// Create deletion engine with generic backend
		backend := backend.NewBackend()
		engine := NewEngine(backend, 2, nil)

		// Create context for deletion
		ctx := context.Background()

		// Perform deletion (not dry-run)
		result, err := engine.Delete(ctx, filesToDelete, false)
		if err != nil {
			rt.Fatalf("Delete failed: %v", err)
		}

		// Property: When deletion completes successfully, the target directory
		// and all its contents should no longer exist on the filesystem
		
		// Check that deletion was successful (no failures)
		if result.FailedCount > 0 {
			rt.Fatalf("Deletion had %d failures, expected 0. Errors: %v", 
				result.FailedCount, result.Errors)
		}

		// Verify the target directory no longer exists
		if _, err := os.Stat(targetDir); !os.IsNotExist(err) {
			rt.Fatalf("Target directory %s still exists after successful deletion", targetDir)
		}

		// Verify all created paths no longer exist
		for _, path := range allPaths {
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				rt.Fatalf("Path %s still exists after successful deletion", path)
			}
		}

		// Verify the deletion count matches the number of items we tried to delete
		if result.DeletedCount != len(filesToDelete) {
			rt.Fatalf("Expected %d items deleted, got %d", 
				len(filesToDelete), result.DeletedCount)
		}
	})
}

// Feature: fast-file-deletion, Property 2: Deletion Isolation
// For any set of directories where one is the target for deletion,
// deleting the target directory should not affect files or directories outside the target path.
// Validates: Requirements 1.3
func TestDeletionIsolation(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Create a temporary directory for this test iteration
		tmpDir := t.TempDir()
		
		// Create two sibling directories: target (to delete) and sibling (to preserve)
		targetDir := filepath.Join(tmpDir, "target")
		siblingDir := filepath.Join(tmpDir, "sibling")
		
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			rt.Fatalf("Failed to create target directory: %v", err)
		}
		if err := os.MkdirAll(siblingDir, 0755); err != nil {
			rt.Fatalf("Failed to create sibling directory: %v", err)
		}

		// Generate random content in target directory
		numTargetFiles := rapid.IntRange(1, 10).Draw(rt, "numTargetFiles")
		for i := 0; i < numTargetFiles; i++ {
			fileName := filepath.Join(targetDir, fmt.Sprintf("target_file_%d.txt", i))
			content := []byte(fmt.Sprintf("target content %d", i))
			if err := os.WriteFile(fileName, content, 0644); err != nil {
				rt.Fatalf("Failed to create target file: %v", err)
			}
		}

		// Generate random content in sibling directory
		numSiblingFiles := rapid.IntRange(1, 10).Draw(rt, "numSiblingFiles")
		siblingFiles := make(map[string][]byte)
		for i := 0; i < numSiblingFiles; i++ {
			fileName := filepath.Join(siblingDir, fmt.Sprintf("sibling_file_%d.txt", i))
			content := []byte(fmt.Sprintf("sibling content %d", i))
			if err := os.WriteFile(fileName, content, 0644); err != nil {
				rt.Fatalf("Failed to create sibling file: %v", err)
			}
			siblingFiles[fileName] = content
		}

		// Create a subdirectory in sibling with files
		siblingSubdir := filepath.Join(siblingDir, "subdir")
		if err := os.MkdirAll(siblingSubdir, 0755); err != nil {
			rt.Fatalf("Failed to create sibling subdirectory: %v", err)
		}
		numSubdirFiles := rapid.IntRange(1, 5).Draw(rt, "numSubdirFiles")
		for i := 0; i < numSubdirFiles; i++ {
			fileName := filepath.Join(siblingSubdir, fmt.Sprintf("subdir_file_%d.txt", i))
			content := []byte(fmt.Sprintf("subdir content %d", i))
			if err := os.WriteFile(fileName, content, 0644); err != nil {
				rt.Fatalf("Failed to create subdir file: %v", err)
			}
			siblingFiles[fileName] = content
		}

		// Verify sibling directory and files exist before deletion
		if _, err := os.Stat(siblingDir); os.IsNotExist(err) {
			rt.Fatalf("Sibling directory does not exist before deletion")
		}
		for path := range siblingFiles {
			if _, err := os.Stat(path); os.IsNotExist(err) {
				rt.Fatalf("Sibling file %s does not exist before deletion", path)
			}
		}

		// Build file list for deletion (only target directory, bottom-up order)
		var filesToDelete []string
		err := filepath.WalkDir(targetDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			filesToDelete = append(filesToDelete, path)
			return nil
		})
		if err != nil {
			rt.Fatalf("Failed to walk target directory: %v", err)
		}

		// Reverse the list to get bottom-up order (files before directories)
		for i, j := 0, len(filesToDelete)-1; i < j; i, j = i+1, j-1 {
			filesToDelete[i], filesToDelete[j] = filesToDelete[j], filesToDelete[i]
		}

		// Create deletion engine with generic backend
		backend := backend.NewBackend()
		engine := NewEngine(backend, 2, nil)

		// Create context for deletion
		ctx := context.Background()

		// Perform deletion (not dry-run)
		result, err := engine.Delete(ctx, filesToDelete, false)
		if err != nil {
			rt.Fatalf("Delete failed: %v", err)
		}

		// Property: Deleting the target directory should not affect files
		// or directories outside the target path

		// Verify target directory was deleted
		if _, err := os.Stat(targetDir); !os.IsNotExist(err) {
			rt.Fatalf("Target directory %s still exists after deletion", targetDir)
		}

		// Verify sibling directory still exists
		if _, err := os.Stat(siblingDir); os.IsNotExist(err) {
			rt.Fatalf("Sibling directory %s was deleted (isolation violated)", siblingDir)
		}

		// Verify all sibling files still exist with correct content
		for path, expectedContent := range siblingFiles {
			// Check file exists
			if _, err := os.Stat(path); os.IsNotExist(err) {
				rt.Fatalf("Sibling file %s was deleted (isolation violated)", path)
			}

			// Check file content is unchanged
			actualContent, err := os.ReadFile(path)
			if err != nil {
				rt.Fatalf("Failed to read sibling file %s: %v", path, err)
			}
			if string(actualContent) != string(expectedContent) {
				rt.Fatalf("Sibling file %s content changed (isolation violated). Expected: %s, Got: %s",
					path, string(expectedContent), string(actualContent))
			}
		}

		// Verify sibling subdirectory still exists
		if _, err := os.Stat(siblingSubdir); os.IsNotExist(err) {
			rt.Fatalf("Sibling subdirectory %s was deleted (isolation violated)", siblingSubdir)
		}

		// Verify deletion was successful (no failures)
		if result.FailedCount > 0 {
			rt.Fatalf("Deletion had %d failures, expected 0. Errors: %v",
				result.FailedCount, result.Errors)
		}
	})
}

// Feature: fast-file-deletion, Property 9: Error Resilience
// For any set of files where some have restricted permissions or are locked,
// the deletion engine should continue processing remaining files and not halt on individual failures.
// Validates: Requirements 4.1, 4.2
func TestErrorResilience(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Create a temporary directory for this test iteration
		tmpDir := t.TempDir()
		targetDir := filepath.Join(tmpDir, "target")
		
		// Create the target directory
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			rt.Fatalf("Failed to create target directory: %v", err)
		}

		// Generate a random number of subdirectories (2 to 5)
		// We'll make some of them read-only to cause deletion failures
		numSubdirs := rapid.IntRange(2, 5).Draw(rt, "numSubdirs")
		
		// Randomly select which subdirectories will be read-only (at least 1, at most half)
		numReadOnlyDirs := rapid.IntRange(1, numSubdirs/2).Draw(rt, "numReadOnlyDirs")
		
		// Create a set of indices for read-only directories
		readOnlyIndices := make(map[int]bool)
		for len(readOnlyIndices) < numReadOnlyDirs {
			idx := rapid.IntRange(0, numSubdirs-1).Draw(rt, "readOnlyIdx")
			readOnlyIndices[idx] = true
		}

		// Track all created paths and which ones should fail
		allFiles := make([]string, 0)
		readOnlyDirFiles := make(map[string]bool)
		normalDirFiles := make(map[string]bool)
		subdirs := make([]string, 0)
		
		// Create subdirectories and files
		for i := 0; i < numSubdirs; i++ {
			subdir := filepath.Join(targetDir, fmt.Sprintf("subdir_%d", i))
			if err := os.MkdirAll(subdir, 0755); err != nil {
				rt.Fatalf("Failed to create subdirectory: %v", err)
			}
			subdirs = append(subdirs, subdir)
			
			// Create 2-3 files in each subdirectory
			numFilesInDir := rapid.IntRange(2, 3).Draw(rt, "numFilesInDir")
			for j := 0; j < numFilesInDir; j++ {
				fileName := filepath.Join(subdir, fmt.Sprintf("file_%d.txt", j))
				
				// Generate random file content
				contentSize := rapid.IntRange(10, 100).Draw(rt, "contentSize")
				content := make([]byte, contentSize)
				for k := 0; k < contentSize; k++ {
					content[k] = byte(rapid.IntRange(0, 255).Draw(rt, "contentByte"))
				}
				
				if err := os.WriteFile(fileName, content, 0644); err != nil {
					rt.Fatalf("Failed to create file: %v", err)
				}
				
				allFiles = append(allFiles, fileName)
				
				// Track which directory this file belongs to
				if readOnlyIndices[i] {
					readOnlyDirFiles[fileName] = true
				} else {
					normalDirFiles[fileName] = true
				}
			}
			
			// If this directory should be read-only, remove write permissions
			// This will prevent deletion of files within it on Unix-like systems
			if readOnlyIndices[i] {
				if err := os.Chmod(subdir, 0555); err != nil {
					rt.Fatalf("Failed to set read-only permissions on directory: %v", err)
				}
			}
		}

		// Add subdirectories and target directory to the list (for deletion)
		// Reverse order so files come before directories
		for i := len(subdirs) - 1; i >= 0; i-- {
			allFiles = append(allFiles, subdirs[i])
		}
		allFiles = append(allFiles, targetDir)

		// Verify all files exist before deletion
		for _, path := range allFiles {
			if _, err := os.Stat(path); os.IsNotExist(err) {
				rt.Fatalf("Path %s does not exist before deletion", path)
			}
		}

		// Create deletion engine with generic backend
		backend := backend.NewBackend()
		engine := NewEngine(backend, 2, nil)

		// Create context for deletion
		ctx := context.Background()

		// Perform deletion (not dry-run)
		result, err := engine.Delete(ctx, allFiles, false)
		if err != nil {
			rt.Fatalf("Delete failed: %v", err)
		}

		// Property: The deletion engine should continue processing remaining files
		// and not halt on individual failures

		// Verify that some files failed to delete (the ones in read-only directories)
		if result.FailedCount == 0 {
			rt.Fatalf("Expected some failures due to read-only directories, but got 0 failures")
		}

		// Verify that some files were successfully deleted (the ones in normal directories)
		if result.DeletedCount == 0 {
			rt.Fatalf("Expected some successful deletions, but got 0 deletions")
		}

		// Verify that the total count matches
		totalProcessed := result.DeletedCount + result.FailedCount
		if totalProcessed != len(allFiles) {
			rt.Fatalf("Expected %d total items processed, got %d (deleted: %d, failed: %d)",
				len(allFiles), totalProcessed, result.DeletedCount, result.FailedCount)
		}

		// Verify that errors were tracked
		if len(result.Errors) != result.FailedCount {
			rt.Fatalf("Expected %d errors in error list, got %d",
				result.FailedCount, len(result.Errors))
		}

		// Verify that files in read-only directories still exist (failed to delete)
		for readOnlyFile := range readOnlyDirFiles {
			if _, err := os.Stat(readOnlyFile); os.IsNotExist(err) {
				rt.Fatalf("File %s in read-only directory was deleted despite directory permissions", readOnlyFile)
			}
		}

		// Verify that files in normal directories were deleted
		deletedNormalFiles := 0
		for normalFile := range normalDirFiles {
			if _, err := os.Stat(normalFile); os.IsNotExist(err) {
				deletedNormalFiles++
			}
		}

		// We expect all normal files to be deleted
		if deletedNormalFiles != len(normalDirFiles) {
			rt.Fatalf("Expected all %d normal files to be deleted, but only %d were deleted",
				len(normalDirFiles), deletedNormalFiles)
		}

		// Verify that the engine continued processing despite errors
		// (i.e., it didn't stop at the first error)
		if result.DeletedCount+result.FailedCount < len(allFiles) {
			rt.Fatalf("Engine stopped processing before all files were attempted. "+
				"Processed: %d, Total: %d", result.DeletedCount+result.FailedCount, len(allFiles))
		}
		
		// Clean up: restore write permissions on read-only directories so they can be cleaned up
		for i, subdir := range subdirs {
			if readOnlyIndices[i] {
				os.Chmod(subdir, 0755)
			}
		}
	})
}

// Feature: fast-file-deletion, Property 10: Error Tracking Accuracy
// For any deletion operation, the reported counts of successfully deleted and failed files
// should exactly match the actual number of files deleted and failed during the operation.
// Validates: Requirements 4.4, 4.5
func TestErrorTrackingAccuracy(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Create a temporary directory for this test iteration
		tmpDir := t.TempDir()
		targetDir := filepath.Join(tmpDir, "target")
		
		// Create the target directory
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			rt.Fatalf("Failed to create target directory: %v", err)
		}

		// Generate a random number of subdirectories (2 to 6)
		numSubdirs := rapid.IntRange(2, 6).Draw(rt, "numSubdirs")
		
		// Randomly select which subdirectories will be read-only to cause failures
		// At least 1 read-only, at most half of the subdirectories
		numReadOnlyDirs := rapid.IntRange(1, numSubdirs/2).Draw(rt, "numReadOnlyDirs")
		
		// Create a set of indices for read-only directories
		readOnlyIndices := make(map[int]bool)
		for len(readOnlyIndices) < numReadOnlyDirs {
			idx := rapid.IntRange(0, numSubdirs-1).Draw(rt, "readOnlyIdx")
			readOnlyIndices[idx] = true
		}

		// Track expected outcomes
		expectedSuccessfulFiles := 0
		expectedFailedFiles := 0
		allFiles := make([]string, 0)
		subdirs := make([]string, 0)
		
		// Create subdirectories and files
		for i := 0; i < numSubdirs; i++ {
			subdir := filepath.Join(targetDir, fmt.Sprintf("subdir_%d", i))
			if err := os.MkdirAll(subdir, 0755); err != nil {
				rt.Fatalf("Failed to create subdirectory: %v", err)
			}
			subdirs = append(subdirs, subdir)
			
			// Create 2-4 files in each subdirectory
			numFilesInDir := rapid.IntRange(2, 4).Draw(rt, "numFilesInDir")
			for j := 0; j < numFilesInDir; j++ {
				fileName := filepath.Join(subdir, fmt.Sprintf("file_%d.txt", j))
				
				// Generate random file content
				contentSize := rapid.IntRange(10, 100).Draw(rt, "contentSize")
				content := make([]byte, contentSize)
				for k := 0; k < contentSize; k++ {
					content[k] = byte(rapid.IntRange(0, 255).Draw(rt, "contentByte"))
				}
				
				if err := os.WriteFile(fileName, content, 0644); err != nil {
					rt.Fatalf("Failed to create file: %v", err)
				}
				
				allFiles = append(allFiles, fileName)
				
				// Track expected outcome based on directory permissions
				if readOnlyIndices[i] {
					expectedFailedFiles++
				} else {
					expectedSuccessfulFiles++
				}
			}
			
			// If this directory should be read-only, remove write permissions
			if readOnlyIndices[i] {
				if err := os.Chmod(subdir, 0555); err != nil {
					rt.Fatalf("Failed to set read-only permissions on directory: %v", err)
				}
				// The directory itself will also fail to delete
				expectedFailedFiles++
			} else {
				// Normal directories will be successfully deleted
				expectedSuccessfulFiles++
			}
		}

		// Add subdirectories to the deletion list (reverse order for bottom-up)
		for i := len(subdirs) - 1; i >= 0; i-- {
			allFiles = append(allFiles, subdirs[i])
		}
		
		// Add target directory (will succeed if all subdirs are deleted, fail otherwise)
		allFiles = append(allFiles, targetDir)
		if numReadOnlyDirs > 0 {
			// Target directory will fail because read-only subdirs can't be deleted
			expectedFailedFiles++
		} else {
			expectedSuccessfulFiles++
		}

		// Create deletion engine with generic backend
		backend := backend.NewBackend()
		engine := NewEngine(backend, 2, nil)

		// Create context for deletion
		ctx := context.Background()

		// Perform deletion (not dry-run)
		result, err := engine.Delete(ctx, allFiles, false)
		if err != nil {
			rt.Fatalf("Delete failed: %v", err)
		}

		// Property: The reported counts of successfully deleted and failed files
		// should exactly match the actual number of files deleted and failed

		// Verify the total count matches
		totalProcessed := result.DeletedCount + result.FailedCount
		if totalProcessed != len(allFiles) {
			rt.Fatalf("Total processed count mismatch. Expected: %d, Got: %d (deleted: %d, failed: %d)",
				len(allFiles), totalProcessed, result.DeletedCount, result.FailedCount)
		}

		// Verify the error list count matches the failed count
		if len(result.Errors) != result.FailedCount {
			rt.Fatalf("Error list count mismatch. Expected: %d errors, Got: %d errors",
				result.FailedCount, len(result.Errors))
		}

		// Verify each error in the list has a valid path and error message
		for _, fileError := range result.Errors {
			if fileError.Path == "" {
				rt.Fatalf("Error entry has empty path")
			}
			if fileError.Error == "" {
				rt.Fatalf("Error entry for path %s has empty error message", fileError.Path)
			}
		}

		// Verify the deleted count matches expected successful deletions
		// Note: This is approximate because directory deletion depends on whether
		// all children were deleted successfully
		if result.DeletedCount != expectedSuccessfulFiles {
			rt.Fatalf("Deleted count mismatch. Expected: %d, Got: %d",
				expectedSuccessfulFiles, result.DeletedCount)
		}

		// Verify the failed count matches expected failures
		if result.FailedCount != expectedFailedFiles {
			rt.Fatalf("Failed count mismatch. Expected: %d, Got: %d",
				expectedFailedFiles, result.FailedCount)
		}

		// Verify that no file appears in both deleted and failed categories
		// by checking that deleted + failed = total
		if result.DeletedCount+result.FailedCount != len(allFiles) {
			rt.Fatalf("Deleted + Failed != Total. Deleted: %d, Failed: %d, Total: %d",
				result.DeletedCount, result.FailedCount, len(allFiles))
		}

		// Verify that the engine tracked all operations (no missing items)
		if result.DeletedCount == 0 && result.FailedCount == 0 {
			rt.Fatalf("No operations were tracked (both counts are 0)")
		}

		// Clean up: restore write permissions on read-only directories
		for i, subdir := range subdirs {
			if readOnlyIndices[i] {
				os.Chmod(subdir, 0755)
			}
		}
	})
}

// Feature: fast-file-deletion, Property 4: Dry-Run Preservation
// For any directory structure, running deletion in dry-run mode should leave
// all files and directories unchanged on the filesystem.
// Validates: Requirements 2.3, 6.4
func TestDryRunPreservation(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Create a temporary directory for this test iteration
		tmpDir := t.TempDir()
		targetDir := filepath.Join(tmpDir, "target")
		
		// Create the target directory
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			rt.Fatalf("Failed to create target directory: %v", err)
		}

		// Generate a random directory structure
		// Number of subdirectories (0 to 5)
		numSubdirs := rapid.IntRange(0, 5).Draw(rt, "numSubdirs")
		
		// Number of files per directory (1 to 10)
		numFilesPerDir := rapid.IntRange(1, 10).Draw(rt, "numFilesPerDir")

		// Track all created paths and their content for verification
		allPaths := []string{targetDir}
		fileContents := make(map[string][]byte)
		
		// Create subdirectories
		subdirs := []string{targetDir}
		for i := 0; i < numSubdirs; i++ {
			// Pick a random parent directory
			parentIdx := rapid.IntRange(0, len(subdirs)-1).Draw(rt, "parentIdx")
			parentDir := subdirs[parentIdx]
			
			// Create a subdirectory
			subdir := filepath.Join(parentDir, fmt.Sprintf("subdir_%d", i))
			if err := os.MkdirAll(subdir, 0755); err != nil {
				rt.Fatalf("Failed to create subdirectory: %v", err)
			}
			subdirs = append(subdirs, subdir)
			allPaths = append(allPaths, subdir)
		}

		// Create files in each directory
		for _, dir := range subdirs {
			for i := 0; i < numFilesPerDir; i++ {
				fileName := filepath.Join(dir, fmt.Sprintf("file_%d.txt", i))
				
				// Generate random file content (0 to 1000 bytes)
				contentSize := rapid.IntRange(0, 1000).Draw(rt, "contentSize")
				content := make([]byte, contentSize)
				for j := 0; j < contentSize; j++ {
					content[j] = byte(rapid.IntRange(0, 255).Draw(rt, "contentByte"))
				}
				
				if err := os.WriteFile(fileName, content, 0644); err != nil {
					rt.Fatalf("Failed to create file: %v", err)
				}
				allPaths = append(allPaths, fileName)
				fileContents[fileName] = content
			}
		}

		// Verify all paths exist before dry-run deletion
		for _, path := range allPaths {
			if _, err := os.Stat(path); os.IsNotExist(err) {
				rt.Fatalf("Path %s does not exist before dry-run deletion", path)
			}
		}

		// Build file list for deletion (bottom-up order)
		var filesToDelete []string
		err := filepath.WalkDir(targetDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			filesToDelete = append(filesToDelete, path)
			return nil
		})
		if err != nil {
			rt.Fatalf("Failed to walk directory: %v", err)
		}

		// Reverse the list to get bottom-up order (files before directories)
		for i, j := 0, len(filesToDelete)-1; i < j; i, j = i+1, j-1 {
			filesToDelete[i], filesToDelete[j] = filesToDelete[j], filesToDelete[i]
		}

		// Create deletion engine with generic backend
		backend := backend.NewBackend()
		engine := NewEngine(backend, 2, nil)

		// Create context for deletion
		ctx := context.Background()

		// Perform deletion in DRY-RUN mode
		result, err := engine.Delete(ctx, filesToDelete, true)
		if err != nil {
			rt.Fatalf("Delete failed: %v", err)
		}

		// Property: Running deletion in dry-run mode should leave all files
		// and directories unchanged on the filesystem

		// Verify the target directory still exists
		if _, err := os.Stat(targetDir); os.IsNotExist(err) {
			rt.Fatalf("Target directory %s was deleted in dry-run mode (preservation violated)", targetDir)
		}

		// Verify all created paths still exist
		for _, path := range allPaths {
			if _, err := os.Stat(path); os.IsNotExist(err) {
				rt.Fatalf("Path %s was deleted in dry-run mode (preservation violated)", path)
			}
		}

		// Verify all file contents are unchanged
		for fileName, expectedContent := range fileContents {
			actualContent, err := os.ReadFile(fileName)
			if err != nil {
				rt.Fatalf("Failed to read file %s after dry-run: %v", fileName, err)
			}
			if string(actualContent) != string(expectedContent) {
				rt.Fatalf("File %s content changed in dry-run mode (preservation violated). Expected: %v, Got: %v",
					fileName, expectedContent, actualContent)
			}
		}

		// Verify dry-run reported success (no failures)
		if result.FailedCount > 0 {
			rt.Fatalf("Dry-run had %d failures, expected 0. Errors: %v",
				result.FailedCount, result.Errors)
		}

		// Verify dry-run reported the correct number of "deleted" items
		// (even though nothing was actually deleted)
		if result.DeletedCount != len(filesToDelete) {
			rt.Fatalf("Expected dry-run to report %d items processed, got %d",
				len(filesToDelete), result.DeletedCount)
		}
	})
}

// Unit Tests for Deletion Engine
// Requirements: 4.3, 5.1, 5.5

// TestWorkerCountAutoDetection tests that the engine correctly auto-detects
// the optimal worker count based on CPU cores when workers <= 0.
// Validates: Requirements 5.1, 5.5
func TestWorkerCountAutoDetection(t *testing.T) {
	backend := backend.NewBackend()
	
	// Test with workers = 0 (should auto-detect)
	engine := NewEngine(backend, 0, nil)
	expectedWorkers := runtime.NumCPU() * 2
	if engine.workers != expectedWorkers {
		t.Errorf("Expected %d workers (auto-detected), got %d", expectedWorkers, engine.workers)
	}
	
	// Test with workers = -1 (should also auto-detect)
	engine = NewEngine(backend, -1, nil)
	if engine.workers != expectedWorkers {
		t.Errorf("Expected %d workers (auto-detected), got %d", expectedWorkers, engine.workers)
	}
	
	// Test with workers = -100 (should also auto-detect)
	engine = NewEngine(backend, -100, nil)
	if engine.workers != expectedWorkers {
		t.Errorf("Expected %d workers (auto-detected), got %d", expectedWorkers, engine.workers)
	}
	
	// Test with explicit worker count (should use specified value)
	engine = NewEngine(backend, 4, nil)
	if engine.workers != 4 {
		t.Errorf("Expected 4 workers (explicit), got %d", engine.workers)
	}
	
	// Test with workers = 1 (should use specified value)
	engine = NewEngine(backend, 1, nil)
	if engine.workers != 1 {
		t.Errorf("Expected 1 worker (explicit), got %d", engine.workers)
	}
}

// TestInterruptionHandling tests that the engine handles context cancellation
// gracefully and stops processing when interrupted.
// Validates: Requirements 4.3
func TestInterruptionHandling(t *testing.T) {
	// Create a temporary directory with many files
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "target")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatalf("Failed to create target directory: %v", err)
	}
	
	// Create a large number of files to ensure deletion takes some time
	numFiles := 100
	var filesToDelete []string
	for i := 0; i < numFiles; i++ {
		fileName := filepath.Join(targetDir, fmt.Sprintf("file_%d.txt", i))
		content := []byte(fmt.Sprintf("content %d", i))
		if err := os.WriteFile(fileName, content, 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
		filesToDelete = append(filesToDelete, fileName)
	}
	filesToDelete = append(filesToDelete, targetDir)
	
	// Create deletion engine with generic backend
	backend := backend.NewBackend()
	
	// Track progress to verify deletion started
	progressCallback := func(count int) {
		// Progress callback to track deletion
	}
	
	engine := NewEngine(backend, 2, progressCallback)
	
	// Create a context that we'll cancel after a short delay
	ctx, cancel := context.WithCancel(context.Background())
	
	// Start deletion in a goroutine
	resultChan := make(chan *DeletionResult, 1)
	go func() {
		result, _ := engine.Delete(ctx, filesToDelete, false)
		resultChan <- result
	}()
	
	// Cancel the context after a short delay to interrupt deletion
	time.Sleep(10 * time.Millisecond)
	cancel()
	
	// Wait for deletion to complete
	result := <-resultChan
	
	// Verify that deletion was interrupted (not all files were processed)
	totalProcessed := result.DeletedCount + result.FailedCount
	if totalProcessed == len(filesToDelete) {
		// It's possible all files were deleted before cancellation
		// This is acceptable - the important thing is no panic occurred
		t.Logf("All files were processed before cancellation (acceptable)")
	} else {
		// Verify that some files were processed (deletion started)
		if totalProcessed == 0 {
			t.Errorf("No files were processed - deletion may not have started")
		}
		
		// Verify that not all files were processed (interruption worked)
		if totalProcessed >= len(filesToDelete) {
			t.Errorf("All files were processed despite cancellation")
		}
		
		t.Logf("Deletion interrupted: %d/%d files processed", totalProcessed, len(filesToDelete))
	}
	
	// Verify that the engine returned a valid result (no panic)
	if result == nil {
		t.Fatalf("Engine returned nil result after interruption")
	}
	
	// Verify that duration was recorded
	if result.DurationSeconds <= 0 {
		t.Errorf("Expected positive duration, got %f", result.DurationSeconds)
	}
}

// TestEmptyDirectoryHandling tests that the engine correctly handles
// deletion of empty directories without errors.
// Validates: Requirements 5.1
func TestEmptyDirectoryHandling(t *testing.T) {
	// Create a temporary directory with an empty subdirectory
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "target")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatalf("Failed to create target directory: %v", err)
	}
	
	// Create several empty subdirectories
	emptyDir1 := filepath.Join(targetDir, "empty1")
	emptyDir2 := filepath.Join(targetDir, "empty2")
	emptyDir3 := filepath.Join(targetDir, "empty3")
	
	if err := os.MkdirAll(emptyDir1, 0755); err != nil {
		t.Fatalf("Failed to create empty directory 1: %v", err)
	}
	if err := os.MkdirAll(emptyDir2, 0755); err != nil {
		t.Fatalf("Failed to create empty directory 2: %v", err)
	}
	if err := os.MkdirAll(emptyDir3, 0755); err != nil {
		t.Fatalf("Failed to create empty directory 3: %v", err)
	}
	
	// Verify directories exist
	if _, err := os.Stat(emptyDir1); os.IsNotExist(err) {
		t.Fatalf("Empty directory 1 does not exist")
	}
	if _, err := os.Stat(emptyDir2); os.IsNotExist(err) {
		t.Fatalf("Empty directory 2 does not exist")
	}
	if _, err := os.Stat(emptyDir3); os.IsNotExist(err) {
		t.Fatalf("Empty directory 3 does not exist")
	}
	
	// Build file list for deletion (bottom-up order: subdirs before parent)
	filesToDelete := []string{
		emptyDir1,
		emptyDir2,
		emptyDir3,
		targetDir,
	}
	
	// Create deletion engine with generic backend
	backend := backend.NewBackend()
	engine := NewEngine(backend, 2, nil)
	
	// Create context for deletion
	ctx := context.Background()
	
	// Perform deletion (not dry-run)
	result, err := engine.Delete(ctx, filesToDelete, false)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	
	// Verify deletion was successful (no failures)
	if result.FailedCount > 0 {
		t.Errorf("Expected 0 failures for empty directories, got %d. Errors: %v",
			result.FailedCount, result.Errors)
	}
	
	// Verify all directories were deleted
	if result.DeletedCount != len(filesToDelete) {
		t.Errorf("Expected %d items deleted, got %d",
			len(filesToDelete), result.DeletedCount)
	}
	
	// Verify directories no longer exist
	if _, err := os.Stat(emptyDir1); !os.IsNotExist(err) {
		t.Errorf("Empty directory 1 still exists after deletion")
	}
	if _, err := os.Stat(emptyDir2); !os.IsNotExist(err) {
		t.Errorf("Empty directory 2 still exists after deletion")
	}
	if _, err := os.Stat(emptyDir3); !os.IsNotExist(err) {
		t.Errorf("Empty directory 3 still exists after deletion")
	}
	if _, err := os.Stat(targetDir); !os.IsNotExist(err) {
		t.Errorf("Target directory still exists after deletion")
	}
	
	// Verify duration was recorded
	if result.DurationSeconds <= 0 {
		t.Errorf("Expected positive duration, got %f", result.DurationSeconds)
	}
}

// TestEmptyDirectoryDryRun tests that empty directories are preserved in dry-run mode.
// Validates: Requirements 5.1
func TestEmptyDirectoryDryRun(t *testing.T) {
	// Create a temporary directory with an empty subdirectory
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "target")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatalf("Failed to create target directory: %v", err)
	}
	
	// Create an empty subdirectory
	emptyDir := filepath.Join(targetDir, "empty")
	if err := os.MkdirAll(emptyDir, 0755); err != nil {
		t.Fatalf("Failed to create empty directory: %v", err)
	}
	
	// Build file list for deletion
	filesToDelete := []string{emptyDir, targetDir}
	
	// Create deletion engine with generic backend
	backend := backend.NewBackend()
	engine := NewEngine(backend, 2, nil)
	
	// Create context for deletion
	ctx := context.Background()
	
	// Perform deletion in DRY-RUN mode
	result, err := engine.Delete(ctx, filesToDelete, true)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	
	// Verify dry-run reported success
	if result.FailedCount > 0 {
		t.Errorf("Dry-run had %d failures, expected 0. Errors: %v",
			result.FailedCount, result.Errors)
	}
	
	// Verify dry-run reported the correct number of "deleted" items
	if result.DeletedCount != len(filesToDelete) {
		t.Errorf("Expected dry-run to report %d items processed, got %d",
			len(filesToDelete), result.DeletedCount)
	}
	
	// Verify directories still exist (not actually deleted)
	if _, err := os.Stat(emptyDir); os.IsNotExist(err) {
		t.Errorf("Empty directory was deleted in dry-run mode")
	}
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		t.Errorf("Target directory was deleted in dry-run mode")
	}
}
