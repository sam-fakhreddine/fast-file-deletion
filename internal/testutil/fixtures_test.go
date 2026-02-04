package testutil

import (
	"os"
	"path/filepath"
	"testing"

	"pgregory.net/rapid"
)

// TestCreateTestDirectory verifies that CreateTestDirectory creates
// a directory with the correct number of files respecting config limits.
func TestCreateTestDirectory(t *testing.T) {
	tests := []struct {
		name   string
		config TestConfig
	}{
		{
			name: "quick mode defaults",
			config: TestConfig{
				Intensity:   IntensityQuick,
				MaxFiles:    100,
				MaxFileSize: 1024,
				MaxDepth:    3,
			},
		},
		{
			name: "thorough mode defaults",
			config: TestConfig{
				Intensity:   IntensityThorough,
				MaxFiles:    1000,
				MaxFileSize: 10240,
				MaxDepth:    5,
			},
		},
		{
			name: "minimal config",
			config: TestConfig{
				Intensity:   IntensityQuick,
				MaxFiles:    5,
				MaxFileSize: 10,
				MaxDepth:    1,
			},
		},
		{
			name: "zero files",
			config: TestConfig{
				Intensity:   IntensityQuick,
				MaxFiles:    0,
				MaxFileSize: 100,
				MaxDepth:    1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test directory
			dir := CreateTestDirectory(t, tt.config)

			// Verify directory exists
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				t.Fatalf("Directory was not created: %s", dir)
			}

			// Count files in directory
			fileCount := 0
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatalf("Failed to read directory: %v", err)
			}

			for _, entry := range entries {
				if !entry.IsDir() {
					fileCount++

					// Verify file size is within limits
					filePath := filepath.Join(dir, entry.Name())
					info, err := os.Stat(filePath)
					if err != nil {
						t.Fatalf("Failed to stat file %s: %v", filePath, err)
					}

					if info.Size() < 1 {
						t.Errorf("File %s has size %d, expected at least 1 byte", entry.Name(), info.Size())
					}
					if info.Size() > tt.config.MaxFileSize {
						t.Errorf("File %s has size %d, exceeds MaxFileSize %d", entry.Name(), info.Size(), tt.config.MaxFileSize)
					}
				}
			}

			// Verify file count matches config
			if fileCount != tt.config.MaxFiles {
				t.Errorf("Expected %d files, got %d", tt.config.MaxFiles, fileCount)
			}
		})
	}
}

// TestCreateTestDirectoryWithTree verifies that CreateTestDirectoryWithTree
// creates a nested directory structure respecting depth limits.
func TestCreateTestDirectoryWithTree(t *testing.T) {
	tests := []struct {
		name        string
		config      TestConfig
		filesPerDir int
	}{
		{
			name: "quick mode with depth 3",
			config: TestConfig{
				Intensity:   IntensityQuick,
				MaxFiles:    100,
				MaxFileSize: 1024,
				MaxDepth:    3,
			},
			filesPerDir: 5,
		},
		{
			name: "thorough mode with depth 5",
			config: TestConfig{
				Intensity:   IntensityThorough,
				MaxFiles:    1000,
				MaxFileSize: 10240,
				MaxDepth:    5,
			},
			filesPerDir: 10,
		},
		{
			name: "minimal depth",
			config: TestConfig{
				Intensity:   IntensityQuick,
				MaxFiles:    10,
				MaxFileSize: 100,
				MaxDepth:    1,
			},
			filesPerDir: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test directory with tree
			dir := CreateTestDirectoryWithTree(t, tt.config, tt.filesPerDir)

			// Verify directory exists
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				t.Fatalf("Directory was not created: %s", dir)
			}

			// Count total files and measure depth
			totalFiles := 0
			maxDepth := 0
			baseDepth := len(filepath.SplitList(dir))

			err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return err
				}

				if d.IsDir() {
					depth := len(filepath.SplitList(path)) - baseDepth
					if depth > maxDepth {
						maxDepth = depth
					}
				} else {
					totalFiles++

					// Verify file size is within limits
					info, err := os.Stat(path)
					if err != nil {
						return err
					}

					if info.Size() < 1 {
						t.Errorf("File %s has size %d, expected at least 1 byte", path, info.Size())
					}
					if info.Size() > tt.config.MaxFileSize {
						t.Errorf("File %s has size %d, exceeds MaxFileSize %d", path, info.Size(), tt.config.MaxFileSize)
					}
				}

				return nil
			})

			if err != nil {
				t.Fatalf("Failed to walk directory tree: %v", err)
			}

			// Verify depth doesn't exceed MaxDepth
			if maxDepth > tt.config.MaxDepth {
				t.Errorf("Directory depth %d exceeds MaxDepth %d", maxDepth, tt.config.MaxDepth)
			}

			// Verify we have some files
			if totalFiles == 0 {
				t.Error("Expected at least some files in the tree")
			}

			t.Logf("Created %d files with max depth %d", totalFiles, maxDepth)
		})
	}
}

// TestGenerateTestFiles verifies that GenerateTestFiles creates
// the correct number of files with appropriate sizes.
func TestGenerateTestFiles(t *testing.T) {
	config := TestConfig{
		Intensity:   IntensityQuick,
		MaxFiles:    10,
		MaxFileSize: 100,
		MaxDepth:    1,
	}

	dir := t.TempDir()

	err := GenerateTestFiles(dir, config)
	if err != nil {
		t.Fatalf("GenerateTestFiles failed: %v", err)
	}

	// Count files
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}

	fileCount := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			fileCount++
		}
	}

	if fileCount != config.MaxFiles {
		t.Errorf("Expected %d files, got %d", config.MaxFiles, fileCount)
	}
}

// TestGenerateTestTree verifies that GenerateTestTree creates
// a nested structure respecting depth limits.
func TestGenerateTestTree(t *testing.T) {
	config := TestConfig{
		Intensity:   IntensityQuick,
		MaxFiles:    100,
		MaxFileSize: 1024,
		MaxDepth:    3,
	}

	dir := t.TempDir()
	filesPerDir := 5

	err := GenerateTestTree(dir, 0, filesPerDir, config)
	if err != nil {
		t.Fatalf("GenerateTestTree failed: %v", err)
	}

	// Verify depth doesn't exceed MaxDepth
	maxDepth := 0
	baseDepth := len(filepath.SplitList(dir))

	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			depth := len(filepath.SplitList(path)) - baseDepth
			if depth > maxDepth {
				maxDepth = depth
			}
		}
		return nil
	})

	if err != nil {
		t.Fatalf("Failed to walk directory tree: %v", err)
	}

	if maxDepth > config.MaxDepth {
		t.Errorf("Directory depth %d exceeds MaxDepth %d", maxDepth, config.MaxDepth)
	}
}

// Property Test 1: File Count Limit Enforcement
// **Validates: Requirements 3.1, 3.2**
//
// For any test configuration (quick or thorough mode), when generating test directories,
// the number of files created shall never exceed the configured MaxFiles limit.
func TestProperty_FileCountLimitEnforcement(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate random test configuration
		maxFiles := rapid.IntRange(1, 200).Draw(rt, "maxFiles")
		maxFileSize := rapid.Int64Range(1, 10240).Draw(rt, "maxFileSize")
		maxDepth := rapid.IntRange(1, 5).Draw(rt, "maxDepth")

		config := TestConfig{
			Intensity:   IntensityQuick,
			MaxFiles:    maxFiles,
			MaxFileSize: maxFileSize,
			MaxDepth:    maxDepth,
		}

		// Create test directory with generated config
		dir := t.TempDir()
		err := GenerateTestFiles(dir, config)
		if err != nil {
			rt.Fatalf("Failed to generate test files: %v", err)
		}

		// Count files in directory
		fileCount := 0
		entries, err := os.ReadDir(dir)
		if err != nil {
			rt.Fatalf("Failed to read directory: %v", err)
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				fileCount++
			}
		}

		// Property: file count must never exceed MaxFiles
		if fileCount > config.MaxFiles {
			rt.Fatalf("File count %d exceeds MaxFiles limit %d", fileCount, config.MaxFiles)
		}

		// Also verify exact count (our implementation creates exactly MaxFiles)
		if fileCount != config.MaxFiles {
			rt.Fatalf("Expected exactly %d files, got %d", config.MaxFiles, fileCount)
		}
	})
}

// Property Test 2: File Size Range Compliance
// **Validates: Requirements 3.3, 3.4, 8.4**
//
// For any test configuration and any generated test file, the file size shall be
// within the configured range [1 byte, MaxFileSize].
func TestProperty_FileSizeRangeCompliance(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate random test configuration
		maxFiles := rapid.IntRange(1, 50).Draw(rt, "maxFiles")
		maxFileSize := rapid.Int64Range(1, 10240).Draw(rt, "maxFileSize")

		config := TestConfig{
			Intensity:   IntensityQuick,
			MaxFiles:    maxFiles,
			MaxFileSize: maxFileSize,
			MaxDepth:    1,
		}

		// Create test directory with generated config
		dir := t.TempDir()
		err := GenerateTestFiles(dir, config)
		if err != nil {
			rt.Fatalf("Failed to generate test files: %v", err)
		}

		// Check all file sizes
		entries, err := os.ReadDir(dir)
		if err != nil {
			rt.Fatalf("Failed to read directory: %v", err)
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				filePath := filepath.Join(dir, entry.Name())
				info, err := os.Stat(filePath)
				if err != nil {
					rt.Fatalf("Failed to stat file %s: %v", filePath, err)
				}

				size := info.Size()

				// Property: all file sizes must be in range [1, MaxFileSize]
				if size < 1 {
					rt.Fatalf("File %s has size %d, must be at least 1 byte", entry.Name(), size)
				}
				if size > config.MaxFileSize {
					rt.Fatalf("File %s has size %d, exceeds MaxFileSize %d", entry.Name(), size, config.MaxFileSize)
				}
			}
		}
	})
}

// Property Test 3: Directory Depth Limit Enforcement
// **Validates: Requirements 3.5, 3.6**
//
// For any test configuration and any generated directory structure, the maximum depth
// of nested directories shall never exceed the configured MaxDepth limit.
func TestProperty_DirectoryDepthLimitEnforcement(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate random test configuration
		maxDepth := rapid.IntRange(1, 5).Draw(rt, "maxDepth")
		filesPerDir := rapid.IntRange(1, 10).Draw(rt, "filesPerDir")
		maxFileSize := rapid.Int64Range(1, 1024).Draw(rt, "maxFileSize")

		config := TestConfig{
			Intensity:   IntensityQuick,
			MaxFiles:    100,
			MaxFileSize: maxFileSize,
			MaxDepth:    maxDepth,
		}

		// Create test directory with tree structure
		dir := t.TempDir()
		err := GenerateTestTree(dir, 0, filesPerDir, config)
		if err != nil {
			rt.Fatalf("Failed to generate test tree: %v", err)
		}

		// Measure actual depth
		actualMaxDepth := 0
		baseDepth := len(filepath.SplitList(dir))

		err = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				depth := len(filepath.SplitList(path)) - baseDepth
				if depth > actualMaxDepth {
					actualMaxDepth = depth
				}
			}
			return nil
		})

		if err != nil {
			rt.Fatalf("Failed to walk directory tree: %v", err)
		}

		// Property: actual depth must never exceed MaxDepth
		if actualMaxDepth > config.MaxDepth {
			rt.Fatalf("Directory depth %d exceeds MaxDepth limit %d", actualMaxDepth, config.MaxDepth)
		}
	})
}
