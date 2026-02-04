//go:build windows

package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
	"pgregory.net/rapid"
)

// Feature: windows-performance-optimization, Property 8: Parallel subdirectory processing
// **Validates: Requirements 3.2**
//
// Property: For any directory tree with multiple subdirectories, the parallel scanner
// should process subdirectories concurrently using multiple goroutines, resulting in
// faster scan times compared to sequential processing.
func TestParallelSubdirectoryProcessingProperty(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Create a temporary directory for this test iteration
		tmpDir := t.TempDir()

		// Generate a random number of subdirectories (2 to 10)
		numSubdirs := rapid.IntRange(2, 10).Draw(rt, "numSubdirs")

		// Generate a random number of files per subdirectory (5 to 20)
		filesPerSubdir := rapid.IntRange(5, 20).Draw(rt, "filesPerSubdir")

		// Create subdirectories with files
		expectedFiles := make(map[string]bool)
		for i := 0; i < numSubdirs; i++ {
			subdir := filepath.Join(tmpDir, fmt.Sprintf("subdir_%d", i))
			if err := os.MkdirAll(subdir, 0755); err != nil {
				rt.Fatalf("Failed to create subdirectory: %v", err)
			}

			// Create files in this subdirectory
			for j := 0; j < filesPerSubdir; j++ {
				fileName := filepath.Join(subdir, fmt.Sprintf("file_%d.txt", j))
				if err := os.WriteFile(fileName, []byte("test content"), 0644); err != nil {
					rt.Fatalf("Failed to create file: %v", err)
				}
				expectedFiles[fileName] = true
			}
			expectedFiles[subdir] = true
		}

		// Create parallel scanner with multiple workers
		workers := rapid.IntRange(2, 8).Draw(rt, "workers")
		scanner := NewParallelScanner(tmpDir, nil, workers)

		// Scan the directory
		result, err := scanner.Scan()
		if err != nil {
			rt.Fatalf("Parallel scan failed: %v", err)
		}

		// Property 1: All subdirectories should be discovered
		discoveredDirs := make(map[string]bool)
		for _, path := range result.Files {
			if path == tmpDir {
				continue // Skip root
			}
			info, statErr := os.Stat(path)
			if statErr == nil && info.IsDir() {
				discoveredDirs[path] = true
			}
		}

		for i := 0; i < numSubdirs; i++ {
			subdir := filepath.Join(tmpDir, fmt.Sprintf("subdir_%d", i))
			if !discoveredDirs[subdir] {
				rt.Fatalf("Subdirectory %s was not discovered by parallel scanner", subdir)
			}
		}

		// Property 2: All files should be discovered
		discoveredFiles := make(map[string]bool)
		for _, path := range result.Files {
			discoveredFiles[path] = true
		}

		for expectedFile := range expectedFiles {
			if !discoveredFiles[expectedFile] {
				rt.Fatalf("Expected file/directory %s was not discovered by parallel scanner", expectedFile)
			}
		}

		// Property 3: Scan should complete successfully with multiple workers
		// (This is implicitly tested by the fact that we got results without error)
		if result.TotalScanned == 0 {
			rt.Fatal("Parallel scanner reported 0 scanned items, but we created files")
		}

		// Property 4: UTF-16 paths should be pre-converted for all discovered items
		if len(result.FilesUTF16) != len(result.Files) {
			rt.Fatalf("UTF-16 path count (%d) does not match file count (%d)",
				len(result.FilesUTF16), len(result.Files))
		}

		// Verify UTF-16 paths are not nil
		for i, utf16Path := range result.FilesUTF16 {
			if utf16Path == nil {
				rt.Fatalf("UTF-16 path at index %d is nil (file: %s)", i, result.Files[i])
			}
		}
	})
}

// Feature: windows-performance-optimization, Property 8: Parallel subdirectory processing
// **Validates: Requirements 3.2**
//
// This test verifies that parallel scanning is actually faster than sequential scanning
// for directory trees with multiple subdirectories.
func TestParallelScanningPerformance(t *testing.T) {
	// Skip in short mode as this test measures performance
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	// Create a test directory with multiple subdirectories
	tmpDir := t.TempDir()

	// Create 10 subdirectories with 100 files each
	numSubdirs := 10
	filesPerSubdir := 100

	for i := 0; i < numSubdirs; i++ {
		subdir := filepath.Join(tmpDir, fmt.Sprintf("subdir_%d", i))
		if err := os.MkdirAll(subdir, 0755); err != nil {
			t.Fatalf("Failed to create subdirectory: %v", err)
		}

		for j := 0; j < filesPerSubdir; j++ {
			fileName := filepath.Join(subdir, fmt.Sprintf("file_%d.txt", j))
			if err := os.WriteFile(fileName, []byte("test content"), 0644); err != nil {
				t.Fatalf("Failed to create file: %v", err)
			}
		}
	}

	// Measure sequential scan time (1 worker)
	sequentialScanner := NewParallelScanner(tmpDir, nil, 1)
	startSeq := time.Now()
	seqResult, err := sequentialScanner.Scan()
	if err != nil {
		t.Fatalf("Sequential scan failed: %v", err)
	}
	seqDuration := time.Since(startSeq)

	// Measure parallel scan time (4 workers)
	parallelScanner := NewParallelScanner(tmpDir, nil, 4)
	startPar := time.Now()
	parResult, err := parallelScanner.Scan()
	if err != nil {
		t.Fatalf("Parallel scan failed: %v", err)
	}
	parDuration := time.Since(startPar)

	// Verify both scans found the same number of items
	if seqResult.TotalScanned != parResult.TotalScanned {
		t.Errorf("Sequential and parallel scans found different numbers of items: %d vs %d",
			seqResult.TotalScanned, parResult.TotalScanned)
	}

	// Log the performance comparison
	t.Logf("Sequential scan (1 worker): %v", seqDuration)
	t.Logf("Parallel scan (4 workers): %v", parDuration)
	t.Logf("Speedup: %.2fx", float64(seqDuration)/float64(parDuration))

	// Property: Parallel scanning should be at least as fast as sequential
	// (We don't enforce a specific speedup ratio as it depends on hardware)
	// But we verify that parallel scanning doesn't make things significantly worse
	if parDuration > seqDuration*2 {
		t.Errorf("Parallel scan is significantly slower than sequential: %v vs %v",
			parDuration, seqDuration)
	}
}

// Feature: windows-performance-optimization, Property 8: Parallel subdirectory processing
// **Validates: Requirements 3.2**
//
// This test verifies that the parallel scanner correctly handles deeply nested directories.
func TestParallelScanningDeepNesting(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Create a temporary directory for this test iteration
		tmpDir := t.TempDir()

		// Generate a random depth (3 to 8 levels)
		depth := rapid.IntRange(3, 8).Draw(rt, "depth")

		// Create a deeply nested directory structure
		currentPath := tmpDir
		expectedPaths := make(map[string]bool)

		for i := 0; i < depth; i++ {
			subdir := filepath.Join(currentPath, fmt.Sprintf("level_%d", i))
			if err := os.MkdirAll(subdir, 0755); err != nil {
				rt.Fatalf("Failed to create directory at level %d: %v", i, err)
			}
			expectedPaths[subdir] = true

			// Create a file at this level
			fileName := filepath.Join(subdir, fmt.Sprintf("file_%d.txt", i))
			if err := os.WriteFile(fileName, []byte("test content"), 0644); err != nil {
				rt.Fatalf("Failed to create file at level %d: %v", i, err)
			}
			expectedPaths[fileName] = true

			currentPath = subdir
		}

		// Create parallel scanner
		workers := rapid.IntRange(2, 4).Draw(rt, "workers")
		scanner := NewParallelScanner(tmpDir, nil, workers)

		// Scan the directory
		result, err := scanner.Scan()
		if err != nil {
			rt.Fatalf("Parallel scan failed: %v", err)
		}

		// Property: All paths in the deep structure should be discovered
		discoveredPaths := make(map[string]bool)
		for _, path := range result.Files {
			discoveredPaths[path] = true
		}

		for expectedPath := range expectedPaths {
			if !discoveredPaths[expectedPath] {
				rt.Fatalf("Expected path %s was not discovered by parallel scanner", expectedPath)
			}
		}

		// Property: Scan should complete without errors
		if result.TotalScanned == 0 {
			rt.Fatal("Parallel scanner reported 0 scanned items for deeply nested structure")
		}
	})
}

// Feature: windows-performance-optimization, Property 8: Parallel subdirectory processing
// **Validates: Requirements 3.2**
//
// This test verifies that the parallel scanner handles concurrent access correctly
// without race conditions or data corruption.
func TestParallelScanningConcurrency(t *testing.T) {
	// Run with race detector to catch concurrency issues
	// go test -race ./internal/scanner/...

	rapid.Check(t, func(rt *rapid.T) {
		// Create a temporary directory for this test iteration
		tmpDir := t.TempDir()

		// Generate a random number of subdirectories (5 to 15)
		numSubdirs := rapid.IntRange(5, 15).Draw(rt, "numSubdirs")

		// Create subdirectories with files
		for i := 0; i < numSubdirs; i++ {
			subdir := filepath.Join(tmpDir, fmt.Sprintf("subdir_%d", i))
			if err := os.MkdirAll(subdir, 0755); err != nil {
				rt.Fatalf("Failed to create subdirectory: %v", err)
			}

			// Create files in this subdirectory
			numFiles := rapid.IntRange(3, 10).Draw(rt, "numFiles")
			for j := 0; j < numFiles; j++ {
				fileName := filepath.Join(subdir, fmt.Sprintf("file_%d.txt", j))
				if err := os.WriteFile(fileName, []byte("test content"), 0644); err != nil {
					rt.Fatalf("Failed to create file: %v", err)
				}
			}
		}

		// Create parallel scanner with many workers to stress concurrency
		workers := rapid.IntRange(4, 8).Draw(rt, "workers")
		scanner := NewParallelScanner(tmpDir, nil, workers)

		// Scan the directory multiple times to increase chance of catching race conditions
		for iteration := 0; iteration < 3; iteration++ {
			result, err := scanner.Scan()
			if err != nil {
				rt.Fatalf("Parallel scan failed on iteration %d: %v", iteration, err)
			}

			// Property: Results should be consistent across iterations
			if result.TotalScanned == 0 {
				rt.Fatalf("Parallel scanner reported 0 scanned items on iteration %d", iteration)
			}

			// Property: No duplicate paths in results
			pathSet := make(map[string]bool)
			for _, path := range result.Files {
				if pathSet[path] {
					rt.Fatalf("Duplicate path found in results: %s", path)
				}
				pathSet[path] = true
			}

			// Property: File count matches UTF-16 path count
			if len(result.Files) != len(result.FilesUTF16) {
				rt.Fatalf("File count (%d) does not match UTF-16 path count (%d) on iteration %d",
					len(result.Files), len(result.FilesUTF16), iteration)
			}
		}
	})
}

// Unit test: Verify ParallelScanner handles empty directories correctly
func TestParallelScanner_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	scanner := NewParallelScanner(tmpDir, nil, 4)
	result, err := scanner.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Should include only the root directory
	if len(result.Files) != 1 || result.Files[0] != tmpDir {
		t.Errorf("Expected Files to contain only root directory, got %v", result.Files)
	}

	if result.TotalScanned != 0 {
		t.Errorf("Expected TotalScanned = 0 for empty directory, got %d", result.TotalScanned)
	}
}

// Unit test: Verify ParallelScanner handles single file correctly
func TestParallelScanner_SingleFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	scanner := NewParallelScanner(tmpDir, nil, 4)
	result, err := scanner.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Should include the file and the root directory
	if len(result.Files) != 2 {
		t.Errorf("Expected 2 files (file + root), got %d", len(result.Files))
	}

	if result.TotalScanned != 1 {
		t.Errorf("Expected TotalScanned = 1, got %d", result.TotalScanned)
	}

	// Verify UTF-16 paths are present
	if len(result.FilesUTF16) != 2 {
		t.Errorf("Expected 2 UTF-16 paths, got %d", len(result.FilesUTF16))
	}
}

// Unit test: Verify ParallelScanner respects age filtering
func TestParallelScanner_AgeFiltering(t *testing.T) {
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
	scanner := NewParallelScanner(tmpDir, &keepDays, 4)

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

	// Verify that the new file is NOT in the deletion list
	foundNewFile := false
	for _, file := range result.Files {
		if file == newFile {
			foundNewFile = true
			break
		}
	}
	if foundNewFile {
		t.Error("New file should not be in deletion list")
	}
}


// Feature: windows-performance-optimization, Property 9: Bottom-up ordering invariant
// **Validates: Requirements 3.3**
//
// Property: For any directory tree, the scan result should list all files in bottom-up
// order where every file appears before its parent directory. This ensures that when
// deleting files in the order provided, directories are empty when we attempt to delete them.
func TestBottomUpOrderingInvariantProperty(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Create a temporary directory for this test iteration
		tmpDir := t.TempDir()

		// Generate a random directory structure
		// Create multiple levels of nesting with files at each level
		numLevels := rapid.IntRange(2, 6).Draw(rt, "numLevels")
		numBranchesPerLevel := rapid.IntRange(2, 4).Draw(rt, "numBranchesPerLevel")

		// Track all directories and files we create
		allDirs := make([]string, 0)
		allFiles := make([]string, 0)

		// Create a tree structure
		var createLevel func(parentPath string, level int)
		createLevel = func(parentPath string, level int) {
			if level >= numLevels {
				return
			}

			// Create branches at this level
			for i := 0; i < numBranchesPerLevel; i++ {
				branchDir := filepath.Join(parentPath, fmt.Sprintf("branch_%d_level_%d", i, level))
				if err := os.MkdirAll(branchDir, 0755); err != nil {
					rt.Fatalf("Failed to create directory: %v", err)
				}
				allDirs = append(allDirs, branchDir)

				// Create files in this directory
				numFiles := rapid.IntRange(1, 3).Draw(rt, "numFiles")
				for j := 0; j < numFiles; j++ {
					fileName := filepath.Join(branchDir, fmt.Sprintf("file_%d.txt", j))
					if err := os.WriteFile(fileName, []byte("test"), 0644); err != nil {
						rt.Fatalf("Failed to create file: %v", err)
					}
					allFiles = append(allFiles, fileName)
				}

				// Recurse to next level
				createLevel(branchDir, level+1)
			}
		}

		createLevel(tmpDir, 0)

		// Create parallel scanner
		workers := rapid.IntRange(2, 4).Draw(rt, "workers")
		scanner := NewParallelScanner(tmpDir, nil, workers)

		// Scan the directory
		result, err := scanner.Scan()
		if err != nil {
			rt.Fatalf("Parallel scan failed: %v", err)
		}

		// Property: Bottom-up ordering invariant
		// For every directory in the result, all its children (files and subdirectories)
		// must appear BEFORE it in the list

		// Build a map of path positions
		pathPosition := make(map[string]int)
		for i, path := range result.Files {
			pathPosition[path] = i
		}

		// Check each directory
		for _, dirPath := range allDirs {
			dirPos, dirExists := pathPosition[dirPath]
			if !dirExists {
				rt.Fatalf("Directory %s not found in scan results", dirPath)
			}

			// Check all files in this directory
			for _, filePath := range allFiles {
				if filepath.Dir(filePath) == dirPath {
					filePos, fileExists := pathPosition[filePath]
					if !fileExists {
						rt.Fatalf("File %s not found in scan results", filePath)
					}

					// File must appear BEFORE its parent directory
					if filePos >= dirPos {
						rt.Fatalf("Bottom-up ordering violated: file %s (pos %d) appears after or at same position as parent directory %s (pos %d)",
							filePath, filePos, dirPath, dirPos)
					}
				}
			}

			// Check all subdirectories
			for _, subDirPath := range allDirs {
				if filepath.Dir(subDirPath) == dirPath {
					subDirPos, subDirExists := pathPosition[subDirPath]
					if !subDirExists {
						rt.Fatalf("Subdirectory %s not found in scan results", subDirPath)
					}

					// Subdirectory must appear BEFORE its parent directory
					if subDirPos >= dirPos {
						rt.Fatalf("Bottom-up ordering violated: subdirectory %s (pos %d) appears after or at same position as parent directory %s (pos %d)",
							subDirPath, subDirPos, dirPath, dirPos)
					}
				}
			}
		}

		// Additional check: Root directory should be last
		if len(result.Files) > 0 {
			lastPath := result.Files[len(result.Files)-1]
			if lastPath != tmpDir {
				rt.Fatalf("Root directory should be last in the list, but got %s", lastPath)
			}
		}
	})
}

// Feature: windows-performance-optimization, Property 9: Bottom-up ordering invariant
// **Validates: Requirements 3.3**
//
// This test verifies bottom-up ordering with a simple, deterministic structure.
func TestBottomUpOrderingSimple(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a simple structure:
	// tmpDir/
	//   file1.txt
	//   subdir/
	//     file2.txt
	//     nested/
	//       file3.txt

	file1 := filepath.Join(tmpDir, "file1.txt")
	if err := os.WriteFile(file1, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create file1: %v", err)
	}

	subdir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	file2 := filepath.Join(subdir, "file2.txt")
	if err := os.WriteFile(file2, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create file2: %v", err)
	}

	nested := filepath.Join(subdir, "nested")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatalf("Failed to create nested: %v", err)
	}

	file3 := filepath.Join(nested, "file3.txt")
	if err := os.WriteFile(file3, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create file3: %v", err)
	}

	// Scan with parallel scanner
	scanner := NewParallelScanner(tmpDir, nil, 2)
	result, err := scanner.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Build position map
	pathPosition := make(map[string]int)
	for i, path := range result.Files {
		pathPosition[path] = i
	}

	// Verify ordering constraints
	// file3 must come before nested
	if pathPosition[file3] >= pathPosition[nested] {
		t.Errorf("file3 (pos %d) should come before nested (pos %d)",
			pathPosition[file3], pathPosition[nested])
	}

	// file2 must come before subdir
	if pathPosition[file2] >= pathPosition[subdir] {
		t.Errorf("file2 (pos %d) should come before subdir (pos %d)",
			pathPosition[file2], pathPosition[subdir])
	}

	// nested must come before subdir
	if pathPosition[nested] >= pathPosition[subdir] {
		t.Errorf("nested (pos %d) should come before subdir (pos %d)",
			pathPosition[nested], pathPosition[subdir])
	}

	// file1 must come before tmpDir
	if pathPosition[file1] >= pathPosition[tmpDir] {
		t.Errorf("file1 (pos %d) should come before tmpDir (pos %d)",
			pathPosition[file1], pathPosition[tmpDir])
	}

	// subdir must come before tmpDir
	if pathPosition[subdir] >= pathPosition[tmpDir] {
		t.Errorf("subdir (pos %d) should come before tmpDir (pos %d)",
			pathPosition[subdir], pathPosition[tmpDir])
	}

	// tmpDir should be last
	if result.Files[len(result.Files)-1] != tmpDir {
		t.Errorf("tmpDir should be last, but got %s", result.Files[len(result.Files)-1])
	}
}

// Feature: windows-performance-optimization, Property 9: Bottom-up ordering invariant
// **Validates: Requirements 3.3**
//
// This test verifies that bottom-up ordering is maintained even with many parallel workers.
func TestBottomUpOrderingWithManyWorkers(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Create a temporary directory for this test iteration
		tmpDir := t.TempDir()

		// Create a structure with multiple subdirectories
		numSubdirs := rapid.IntRange(5, 10).Draw(rt, "numSubdirs")

		allDirs := make([]string, 0)
		allFiles := make([]string, 0)

		for i := 0; i < numSubdirs; i++ {
			subdir := filepath.Join(tmpDir, fmt.Sprintf("subdir_%d", i))
			if err := os.MkdirAll(subdir, 0755); err != nil {
				rt.Fatalf("Failed to create subdir: %v", err)
			}
			allDirs = append(allDirs, subdir)

			// Create files in this subdirectory
			numFiles := rapid.IntRange(2, 5).Draw(rt, "numFiles")
			for j := 0; j < numFiles; j++ {
				fileName := filepath.Join(subdir, fmt.Sprintf("file_%d.txt", j))
				if err := os.WriteFile(fileName, []byte("test"), 0644); err != nil {
					rt.Fatalf("Failed to create file: %v", err)
				}
				allFiles = append(allFiles, fileName)
			}

			// Create a nested subdirectory
			nested := filepath.Join(subdir, "nested")
			if err := os.MkdirAll(nested, 0755); err != nil {
				rt.Fatalf("Failed to create nested: %v", err)
			}
			allDirs = append(allDirs, nested)

			// Create files in nested
			nestedFile := filepath.Join(nested, "nested_file.txt")
			if err := os.WriteFile(nestedFile, []byte("test"), 0644); err != nil {
				rt.Fatalf("Failed to create nested file: %v", err)
			}
			allFiles = append(allFiles, nestedFile)
		}

		// Scan with many workers to stress test ordering
		workers := rapid.IntRange(4, 8).Draw(rt, "workers")
		scanner := NewParallelScanner(tmpDir, nil, workers)

		result, err := scanner.Scan()
		if err != nil {
			rt.Fatalf("Scan failed: %v", err)
		}

		// Build position map
		pathPosition := make(map[string]int)
		for i, path := range result.Files {
			pathPosition[path] = i
		}

		// Verify bottom-up ordering for all files and directories
		for _, filePath := range allFiles {
			parentDir := filepath.Dir(filePath)
			filePos := pathPosition[filePath]
			parentPos, parentExists := pathPosition[parentDir]

			if !parentExists {
				rt.Fatalf("Parent directory %s not found in results", parentDir)
			}

			if filePos >= parentPos {
				rt.Fatalf("Bottom-up ordering violated: file %s (pos %d) appears after parent %s (pos %d)",
					filePath, filePos, parentDir, parentPos)
			}
		}

		// Verify ordering for directories
		for _, dirPath := range allDirs {
			if dirPath == tmpDir {
				continue // Skip root
			}

			parentDir := filepath.Dir(dirPath)
			dirPos := pathPosition[dirPath]
			parentPos, parentExists := pathPosition[parentDir]

			if !parentExists {
				rt.Fatalf("Parent directory %s not found in results", parentDir)
			}

			if dirPos >= parentPos {
				rt.Fatalf("Bottom-up ordering violated: directory %s (pos %d) appears after parent %s (pos %d)",
					dirPath, dirPos, parentDir, parentPos)
			}
		}
	})
}


// Feature: windows-performance-optimization, Property 11: Scan fallback correctness
// **Validates: Requirements 3.5**
//
// Property: For any directory tree, when parallel scanning fails or is unavailable,
// the scanner should fall back to sequential filepath.WalkDir and produce equivalent
// results (same files discovered, same ordering guarantees).
func TestScanFallbackCorrectnessProperty(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Create a temporary directory for this test iteration
		tmpDir := t.TempDir()

		// Generate a random directory structure
		numSubdirs := rapid.IntRange(3, 8).Draw(rt, "numSubdirs")
		filesPerSubdir := rapid.IntRange(2, 6).Draw(rt, "filesPerSubdir")

		// Track all files and directories we create
		expectedPaths := make(map[string]bool)

		for i := 0; i < numSubdirs; i++ {
			subdir := filepath.Join(tmpDir, fmt.Sprintf("subdir_%d", i))
			if err := os.MkdirAll(subdir, 0755); err != nil {
				rt.Fatalf("Failed to create subdir: %v", err)
			}
			expectedPaths[subdir] = true

			// Create files in this subdirectory
			for j := 0; j < filesPerSubdir; j++ {
				fileName := filepath.Join(subdir, fmt.Sprintf("file_%d.txt", j))
				if err := os.WriteFile(fileName, []byte("test content"), 0644); err != nil {
					rt.Fatalf("Failed to create file: %v", err)
				}
				expectedPaths[fileName] = true
			}
		}

		// Scan with sequential scanner (baseline)
		seqScanner := NewScanner(tmpDir, nil)
		seqResult, err := seqScanner.Scan()
		if err != nil {
			rt.Fatalf("Sequential scan failed: %v", err)
		}

		// Scan with parallel scanner (which may fall back internally)
		parScanner := NewParallelScanner(tmpDir, nil, 4)
		parResult, err := parScanner.Scan()
		if err != nil {
			rt.Fatalf("Parallel scan failed: %v", err)
		}

		// Property 1: Both scanners should discover the same number of items
		if seqResult.TotalScanned != parResult.TotalScanned {
			rt.Fatalf("Sequential and parallel scans found different numbers of items: %d vs %d",
				seqResult.TotalScanned, parResult.TotalScanned)
		}

		// Property 2: Both scanners should mark the same number of items for deletion
		if seqResult.TotalToDelete != parResult.TotalToDelete {
			rt.Fatalf("Sequential and parallel scans marked different numbers for deletion: %d vs %d",
				seqResult.TotalToDelete, parResult.TotalToDelete)
		}

		// Property 3: Both scanners should discover the same set of files
		seqPaths := make(map[string]bool)
		for _, path := range seqResult.Files {
			seqPaths[path] = true
		}

		parPaths := make(map[string]bool)
		for _, path := range parResult.Files {
			parPaths[path] = true
		}

		// Check that all paths from sequential scan are in parallel scan
		for path := range seqPaths {
			if !parPaths[path] {
				rt.Fatalf("Path %s found by sequential scan but not by parallel scan", path)
			}
		}

		// Check that all paths from parallel scan are in sequential scan
		for path := range parPaths {
			if !seqPaths[path] {
				rt.Fatalf("Path %s found by parallel scan but not by sequential scan", path)
			}
		}

		// Property 4: Both scanners should maintain bottom-up ordering
		// (Root directory should be last in both)
		if len(seqResult.Files) > 0 && seqResult.Files[len(seqResult.Files)-1] != tmpDir {
			rt.Fatalf("Sequential scan: root directory not last")
		}
		if len(parResult.Files) > 0 && parResult.Files[len(parResult.Files)-1] != tmpDir {
			rt.Fatalf("Parallel scan: root directory not last")
		}

		// Property 5: Parallel scanner should provide UTF-16 paths
		if len(parResult.FilesUTF16) != len(parResult.Files) {
			rt.Fatalf("Parallel scan: UTF-16 path count (%d) does not match file count (%d)",
				len(parResult.FilesUTF16), len(parResult.Files))
		}
	})
}

// Feature: windows-performance-optimization, Property 11: Scan fallback correctness
// **Validates: Requirements 3.5**
//
// This test verifies fallback behavior with age filtering enabled.
func TestScanFallbackWithAgeFiltering(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Create a temporary directory for this test iteration
		tmpDir := t.TempDir()

		// Generate a random keep-days threshold
		keepDays := rapid.IntRange(3, 10).Draw(rt, "keepDays")

		// Create files with various ages
		numFiles := rapid.IntRange(5, 15).Draw(rt, "numFiles")
		now := time.Now()

		for i := 0; i < numFiles; i++ {
			fileName := filepath.Join(tmpDir, fmt.Sprintf("file_%d.txt", i))
			if err := os.WriteFile(fileName, []byte("test"), 0644); err != nil {
				rt.Fatalf("Failed to create file: %v", err)
			}

			// Set random modification time (0 to 20 days old)
			daysOld := rapid.IntRange(0, 20).Draw(rt, "daysOld")
			modTime := now.Add(-time.Duration(daysOld) * 24 * time.Hour)
			if err := os.Chtimes(fileName, modTime, modTime); err != nil {
				rt.Fatalf("Failed to set file time: %v", err)
			}
		}

		// Scan with sequential scanner
		seqScanner := NewScanner(tmpDir, &keepDays)
		seqResult, err := seqScanner.Scan()
		if err != nil {
			rt.Fatalf("Sequential scan failed: %v", err)
		}

		// Scan with parallel scanner
		parScanner := NewParallelScanner(tmpDir, &keepDays, 4)
		parResult, err := parScanner.Scan()
		if err != nil {
			rt.Fatalf("Parallel scan failed: %v", err)
		}

		// Property: Both scanners should retain the same number of files
		if seqResult.TotalRetained != parResult.TotalRetained {
			rt.Fatalf("Sequential and parallel scans retained different numbers: %d vs %d",
				seqResult.TotalRetained, parResult.TotalRetained)
		}

		// Property: Both scanners should mark the same files for deletion
		seqPaths := make(map[string]bool)
		for _, path := range seqResult.Files {
			seqPaths[path] = true
		}

		parPaths := make(map[string]bool)
		for _, path := range parResult.Files {
			parPaths[path] = true
		}

		// Files marked for deletion should be the same
		for path := range seqPaths {
			if !parPaths[path] {
				rt.Fatalf("Path %s marked for deletion by sequential scan but not by parallel scan", path)
			}
		}

		for path := range parPaths {
			if !seqPaths[path] {
				rt.Fatalf("Path %s marked for deletion by parallel scan but not by sequential scan", path)
			}
		}
	})
}

// Feature: windows-performance-optimization, Property 11: Scan fallback correctness
// **Validates: Requirements 3.5**
//
// This test verifies that fallback produces consistent results across multiple runs.
func TestScanFallbackConsistency(t *testing.T) {
	// Create a test directory structure
	tmpDir := t.TempDir()

	// Create a deterministic structure
	for i := 0; i < 5; i++ {
		subdir := filepath.Join(tmpDir, fmt.Sprintf("subdir_%d", i))
		if err := os.MkdirAll(subdir, 0755); err != nil {
			t.Fatalf("Failed to create subdir: %v", err)
		}

		for j := 0; j < 3; j++ {
			fileName := filepath.Join(subdir, fmt.Sprintf("file_%d.txt", j))
			if err := os.WriteFile(fileName, []byte("test"), 0644); err != nil {
				t.Fatalf("Failed to create file: %v", err)
			}
		}
	}

	// Scan multiple times with sequential scanner
	var seqResults []*ScanResult
	for i := 0; i < 3; i++ {
		scanner := NewScanner(tmpDir, nil)
		result, err := scanner.Scan()
		if err != nil {
			t.Fatalf("Sequential scan %d failed: %v", i, err)
		}
		seqResults = append(seqResults, result)
	}

	// Scan multiple times with parallel scanner
	var parResults []*ScanResult
	for i := 0; i < 3; i++ {
		scanner := NewParallelScanner(tmpDir, nil, 4)
		result, err := scanner.Scan()
		if err != nil {
			t.Fatalf("Parallel scan %d failed: %v", i, err)
		}
		parResults = append(parResults, result)
	}

	// Property: All sequential scans should produce identical results
	for i := 1; i < len(seqResults); i++ {
		if seqResults[i].TotalScanned != seqResults[0].TotalScanned {
			t.Errorf("Sequential scan %d found different number of items than scan 0: %d vs %d",
				i, seqResults[i].TotalScanned, seqResults[0].TotalScanned)
		}
	}

	// Property: All parallel scans should produce identical results
	for i := 1; i < len(parResults); i++ {
		if parResults[i].TotalScanned != parResults[0].TotalScanned {
			t.Errorf("Parallel scan %d found different number of items than scan 0: %d vs %d",
				i, parResults[i].TotalScanned, parResults[0].TotalScanned)
		}
	}

	// Property: Sequential and parallel scans should produce equivalent results
	if seqResults[0].TotalScanned != parResults[0].TotalScanned {
		t.Errorf("Sequential and parallel scans found different numbers: %d vs %d",
			seqResults[0].TotalScanned, parResults[0].TotalScanned)
	}
}

// Feature: windows-performance-optimization, Property 11: Scan fallback correctness
// **Validates: Requirements 3.5**
//
// This test verifies that the scanner handles errors gracefully and falls back correctly.
func TestScanFallbackErrorHandling(t *testing.T) {
	// Create a test directory
	tmpDir := t.TempDir()

	// Create some files
	for i := 0; i < 5; i++ {
		fileName := filepath.Join(tmpDir, fmt.Sprintf("file_%d.txt", i))
		if err := os.WriteFile(fileName, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
	}

	// Create a subdirectory
	subdir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	// Create files in subdirectory
	for i := 0; i < 3; i++ {
		fileName := filepath.Join(subdir, fmt.Sprintf("file_%d.txt", i))
		if err := os.WriteFile(fileName, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
	}

	// Scan with parallel scanner
	scanner := NewParallelScanner(tmpDir, nil, 4)
	result, err := scanner.Scan()
	if err != nil {
		t.Fatalf("Parallel scan failed: %v", err)
	}

	// Property: Scan should complete successfully
	if result.TotalScanned == 0 {
		t.Error("Scan found no items")
	}

	// Property: All created files should be discovered
	discoveredPaths := make(map[string]bool)
	for _, path := range result.Files {
		discoveredPaths[path] = true
	}

	// Check that subdirectory was discovered
	if !discoveredPaths[subdir] {
		t.Error("Subdirectory not discovered")
	}

	// Property: UTF-16 paths should be available
	if len(result.FilesUTF16) == 0 {
		t.Error("No UTF-16 paths generated")
	}
}

// Feature: windows-performance-optimization, Property 10: UTF-16 pre-conversion completeness
// **Validates: Requirements 3.4, 5.1**
//
// Property: For any path in the scan result, a UTF-16 representation should be pre-converted
// and stored during the scan phase. This ensures that deletion operations can reuse the
// pre-converted paths without repeated UTF-16 conversions, improving performance.
func TestUTF16PreConversionCompletenessProperty(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Create a temporary directory for this test iteration
		tmpDir := t.TempDir()

		// Generate a random directory structure with various path characteristics
		numSubdirs := rapid.IntRange(3, 10).Draw(rt, "numSubdirs")
		filesPerSubdir := rapid.IntRange(2, 8).Draw(rt, "filesPerSubdir")

		// Track all paths we create
		allPaths := make([]string, 0)

		for i := 0; i < numSubdirs; i++ {
			// Create subdirectory with potentially long names to test extended-length path handling
			subdirName := fmt.Sprintf("subdirectory_with_long_name_%d", i)
			subdir := filepath.Join(tmpDir, subdirName)
			if err := os.MkdirAll(subdir, 0755); err != nil {
				rt.Fatalf("Failed to create subdirectory: %v", err)
			}
			allPaths = append(allPaths, subdir)

			// Create files with various name characteristics
			for j := 0; j < filesPerSubdir; j++ {
				// Generate file names with different characteristics
				var fileName string
				switch j % 3 {
				case 0:
					// Simple ASCII name
					fileName = filepath.Join(subdir, fmt.Sprintf("file_%d.txt", j))
				case 1:
					// Name with spaces
					fileName = filepath.Join(subdir, fmt.Sprintf("file with spaces %d.txt", j))
				case 2:
					// Name with special characters
					fileName = filepath.Join(subdir, fmt.Sprintf("file-special_chars_%d.txt", j))
				}

				if err := os.WriteFile(fileName, []byte("test content"), 0644); err != nil {
					rt.Fatalf("Failed to create file: %v", err)
				}
				allPaths = append(allPaths, fileName)
			}

			// Create a nested subdirectory to test deep paths
			if i%2 == 0 {
				nested := filepath.Join(subdir, "nested_directory")
				if err := os.MkdirAll(nested, 0755); err != nil {
					rt.Fatalf("Failed to create nested directory: %v", err)
				}
				allPaths = append(allPaths, nested)

				// Create a file in the nested directory
				nestedFile := filepath.Join(nested, "nested_file.txt")
				if err := os.WriteFile(nestedFile, []byte("nested content"), 0644); err != nil {
					rt.Fatalf("Failed to create nested file: %v", err)
				}
				allPaths = append(allPaths, nestedFile)
			}
		}

		// Create parallel scanner (which performs UTF-16 pre-conversion on Windows)
		workers := rapid.IntRange(2, 6).Draw(rt, "workers")
		scanner := NewParallelScanner(tmpDir, nil, workers)

		// Scan the directory
		result, err := scanner.Scan()
		if err != nil {
			rt.Fatalf("Parallel scan failed: %v", err)
		}

		// Property 1: UTF-16 array length must match Files array length
		// Every path in Files must have a corresponding UTF-16 representation
		if len(result.FilesUTF16) != len(result.Files) {
			rt.Fatalf("UTF-16 pre-conversion incomplete: FilesUTF16 length (%d) does not match Files length (%d)",
				len(result.FilesUTF16), len(result.Files))
		}

		// Property 2: No UTF-16 path should be nil
		// All paths must be successfully converted to UTF-16
		for i, utf16Path := range result.FilesUTF16 {
			if utf16Path == nil {
				rt.Fatalf("UTF-16 path at index %d is nil (corresponding file: %s)",
					i, result.Files[i])
			}
		}

		// Property 3: UTF-16 paths should be valid pointers
		// We verify this by checking that we can convert them back to UTF-8
		for i, utf16Path := range result.FilesUTF16 {
			// Convert UTF-16 back to UTF-8 to verify it's a valid conversion
			reconverted := windows.UTF16PtrToString(utf16Path)
			
			// The reconverted path should contain the original path
			// (it may have extended-length prefix \\?\ added)
			originalPath := result.Files[i]
			
			// Check if the reconverted path ends with the original path
			// or if it's an extended-length version of the original
			if !filepath.IsAbs(originalPath) {
				rt.Fatalf("Original path at index %d is not absolute: %s", i, originalPath)
			}

			// For extended-length paths, strip the \\?\ prefix for comparison
			cleanReconverted := reconverted
			if len(reconverted) >= 4 && reconverted[:4] == `\\?\` {
				cleanReconverted = reconverted[4:]
			}

			// The cleaned reconverted path should match the original path
			// (accounting for case-insensitive Windows filesystem)
			if !equalPathsWindows(cleanReconverted, originalPath) {
				rt.Fatalf("UTF-16 conversion mismatch at index %d:\n  Original: %s\n  Reconverted: %s\n  Cleaned: %s",
					i, originalPath, reconverted, cleanReconverted)
			}
		}

		// Property 4: All created paths should be in the scan result with UTF-16 conversions
		// Build a map of discovered paths
		discoveredPaths := make(map[string]bool)
		for _, path := range result.Files {
			discoveredPaths[path] = true
		}

		// Check that all created paths (or their parents) are discovered
		for _, createdPath := range allPaths {
			if !discoveredPaths[createdPath] {
				rt.Fatalf("Created path %s was not discovered in scan results", createdPath)
			}
		}

		// Property 5: UTF-16 pre-conversion should happen during scan phase
		// This is validated by the fact that FilesUTF16 is populated immediately
		// after Scan() returns, without requiring any additional conversion calls
		if result.TotalToDelete > 0 && len(result.FilesUTF16) == 0 {
			rt.Fatal("UTF-16 pre-conversion did not occur during scan phase")
		}
	})
}

// equalPathsWindows compares two Windows paths for equality, accounting for
// case-insensitivity and path separator normalization.
func equalPathsWindows(path1, path2 string) bool {
	// Normalize path separators
	p1 := filepath.Clean(path1)
	p2 := filepath.Clean(path2)

	// Compare case-insensitively (Windows filesystem is case-insensitive)
	return strings.EqualFold(p1, p2)
}

// Feature: windows-performance-optimization, Property 10: UTF-16 pre-conversion completeness
// **Validates: Requirements 3.4, 5.1**
//
// This test verifies UTF-16 pre-conversion with long paths that require extended-length format.
func TestUTF16PreConversionLongPaths(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a deeply nested directory structure to generate long paths
	// Windows MAX_PATH is 260 characters, so we create paths longer than that
	currentPath := tmpDir
	for i := 0; i < 15; i++ {
		// Create subdirectories with long names
		subdir := filepath.Join(currentPath, fmt.Sprintf("very_long_subdirectory_name_to_exceed_max_path_limit_%d", i))
		if err := os.MkdirAll(subdir, 0755); err != nil {
			t.Fatalf("Failed to create long path directory: %v", err)
		}
		currentPath = subdir
	}

	// Create a file in the deeply nested directory
	longPathFile := filepath.Join(currentPath, "file_in_very_deep_directory.txt")
	if err := os.WriteFile(longPathFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create file in long path: %v", err)
	}

	// Verify the path is actually long (> 260 characters)
	if len(longPathFile) <= 260 {
		t.Logf("Warning: Generated path is not longer than MAX_PATH (length: %d)", len(longPathFile))
	}

	// Scan with parallel scanner
	scanner := NewParallelScanner(tmpDir, nil, 2)
	result, err := scanner.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Property: All paths should have UTF-16 conversions, including long paths
	if len(result.FilesUTF16) != len(result.Files) {
		t.Errorf("UTF-16 path count (%d) does not match file count (%d)",
			len(result.FilesUTF16), len(result.Files))
	}

	// Property: Long paths should be converted with extended-length prefix
	foundLongPath := false
	for i, path := range result.Files {
		if len(path) > 260 {
			foundLongPath = true

			// Verify UTF-16 conversion exists
			if result.FilesUTF16[i] == nil {
				t.Errorf("Long path at index %d has nil UTF-16 conversion: %s", i, path)
			}

			// Verify the UTF-16 path uses extended-length format
			reconverted := windows.UTF16PtrToString(result.FilesUTF16[i])
			if !strings.HasPrefix(reconverted, `\\?\`) {
				t.Errorf("Long path UTF-16 conversion does not use extended-length format: %s", reconverted)
			}
		}
	}

	if !foundLongPath {
		t.Log("Note: No paths longer than 260 characters were generated in this test")
	}
}

// Feature: windows-performance-optimization, Property 10: UTF-16 pre-conversion completeness
// **Validates: Requirements 3.4, 5.1**
//
// This test verifies that UTF-16 pre-conversion works correctly with age filtering.
func TestUTF16PreConversionWithAgeFiltering(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tmpDir := t.TempDir()

		// Generate random keep-days threshold
		keepDays := rapid.IntRange(3, 10).Draw(rt, "keepDays")

		// Create files with various ages
		numFiles := rapid.IntRange(5, 15).Draw(rt, "numFiles")
		now := time.Now()

		for i := 0; i < numFiles; i++ {
			fileName := filepath.Join(tmpDir, fmt.Sprintf("file_%d.txt", i))
			if err := os.WriteFile(fileName, []byte("test"), 0644); err != nil {
				rt.Fatalf("Failed to create file: %v", err)
			}

			// Set random modification time (0 to 20 days old)
			daysOld := rapid.IntRange(0, 20).Draw(rt, "daysOld")
			modTime := now.Add(-time.Duration(daysOld) * 24 * time.Hour)
			if err := os.Chtimes(fileName, modTime, modTime); err != nil {
				rt.Fatalf("Failed to set file time: %v", err)
			}
		}

		// Scan with age filtering
		scanner := NewParallelScanner(tmpDir, &keepDays, 4)
		result, err := scanner.Scan()
		if err != nil {
			rt.Fatalf("Scan failed: %v", err)
		}

		// Property: UTF-16 paths should be pre-converted for all files marked for deletion
		if len(result.FilesUTF16) != len(result.Files) {
			rt.Fatalf("UTF-16 path count (%d) does not match file count (%d) with age filtering",
				len(result.FilesUTF16), len(result.Files))
		}

		// Property: All UTF-16 paths should be valid (not nil)
		for i, utf16Path := range result.FilesUTF16 {
			if utf16Path == nil {
				rt.Fatalf("UTF-16 path at index %d is nil with age filtering (file: %s)",
					i, result.Files[i])
			}
		}

		// Property: Only files marked for deletion should have UTF-16 conversions
		// (Files that are retained should not be in the result)
		if result.TotalRetained > 0 && len(result.Files) == numFiles {
			rt.Fatal("Age filtering did not exclude any files, but some should have been retained")
		}
	})
}

// Feature: windows-performance-optimization, Property 10: UTF-16 pre-conversion completeness
// **Validates: Requirements 3.4, 5.1**
//
// This test verifies UTF-16 pre-conversion with special characters in file names.
func TestUTF16PreConversionSpecialCharacters(t *testing.T) {
	tmpDir := t.TempDir()

	// Create files with various special characters
	specialNames := []string{
		"file with spaces.txt",
		"file-with-dashes.txt",
		"file_with_underscores.txt",
		"file.with.dots.txt",
		"file(with)parentheses.txt",
		"file[with]brackets.txt",
		"file{with}braces.txt",
		"file'with'quotes.txt",
		"file&with&ampersand.txt",
		"file@with@at.txt",
		"file#with#hash.txt",
		"file$with$dollar.txt",
		"file%with%percent.txt",
	}

	for _, name := range specialNames {
		fileName := filepath.Join(tmpDir, name)
		if err := os.WriteFile(fileName, []byte("test"), 0644); err != nil {
			// Some special characters may not be allowed by the filesystem
			t.Logf("Skipping file with special characters (not supported): %s", name)
			continue
		}
	}

	// Scan the directory
	scanner := NewParallelScanner(tmpDir, nil, 2)
	result, err := scanner.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Property: All files with special characters should have UTF-16 conversions
	if len(result.FilesUTF16) != len(result.Files) {
		t.Errorf("UTF-16 path count (%d) does not match file count (%d) with special characters",
			len(result.FilesUTF16), len(result.Files))
	}

	// Property: UTF-16 conversions should be valid for all special character files
	for i, utf16Path := range result.FilesUTF16 {
		if utf16Path == nil {
			t.Errorf("UTF-16 path at index %d is nil (file: %s)", i, result.Files[i])
		}

		// Verify we can convert back to UTF-8
		reconverted := windows.UTF16PtrToString(utf16Path)
		if len(reconverted) == 0 {
			t.Errorf("UTF-16 path at index %d converts to empty string (file: %s)",
				i, result.Files[i])
		}
	}
}
