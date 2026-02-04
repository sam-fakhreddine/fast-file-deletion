package testutil

import (
	"bufio"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// CreateTestDirectory creates a temporary directory with generated files
// according to the provided configuration. Returns the directory path.
// The directory is automatically cleaned up using t.TempDir().
func CreateTestDirectory(t *testing.T, config TestConfig) string {
	t.Helper()

	dir := t.TempDir()

	if err := GenerateTestFiles(dir, config); err != nil {
		t.Fatalf("Failed to generate test files: %v", err)
	}

	return dir
}

// GenerateTestFiles creates files in the given directory according to config.
// Files are created with random content up to MaxFileSize bytes.
// Uses buffered I/O for efficient file creation.
func GenerateTestFiles(dir string, config TestConfig) error {
	for i := 0; i < config.MaxFiles; i++ {
		filename := filepath.Join(dir, fmt.Sprintf("file_%d.txt", i))

		// Generate random file size between 1 byte and MaxFileSize
		size := int64(1)
		if config.MaxFileSize > 1 {
			// Use a simple approach: random size up to MaxFileSize
			randomBytes := make([]byte, 8)
			if _, err := rand.Read(randomBytes); err != nil {
				return fmt.Errorf("failed to generate random size: %w", err)
			}
			// Convert to size in range [1, MaxFileSize]
			size = 1 + (int64(randomBytes[0])%config.MaxFileSize)
			if size < 1 {
				size = 1
			}
			if size > config.MaxFileSize {
				size = config.MaxFileSize
			}
		}

		// Create file with buffered I/O for efficiency
		file, err := os.Create(filename)
		if err != nil {
			return fmt.Errorf("failed to create file %s: %w", filename, err)
		}

		// Use buffered writer for efficient I/O
		writer := bufio.NewWriter(file)

		// Generate and write random content
		content := make([]byte, size)
		if _, err := rand.Read(content); err != nil {
			file.Close()
			return fmt.Errorf("failed to generate random content: %w", err)
		}

		if _, err := writer.Write(content); err != nil {
			file.Close()
			return fmt.Errorf("failed to write content to %s: %w", filename, err)
		}

		// Flush buffered data to disk
		if err := writer.Flush(); err != nil {
			file.Close()
			return fmt.Errorf("failed to flush buffer for %s: %w", filename, err)
		}

		// Close file
		if err := file.Close(); err != nil {
			return fmt.Errorf("failed to close file %s: %w", filename, err)
		}
	}

	return nil
}

// GenerateTestTree creates a nested directory structure with files.
// The structure respects the MaxDepth limit from the configuration.
// Uses buffered I/O for efficient file creation.
func GenerateTestTree(dir string, depth int, filesPerDir int, config TestConfig) error {
	if depth > config.MaxDepth {
		return nil
	}

	// Create files in current directory
	for i := 0; i < filesPerDir; i++ {
		filename := filepath.Join(dir, fmt.Sprintf("file_%d_%d.txt", depth, i))

		// Generate random file size
		size := int64(1)
		if config.MaxFileSize > 1 {
			randomBytes := make([]byte, 8)
			if _, err := rand.Read(randomBytes); err != nil {
				return fmt.Errorf("failed to generate random size: %w", err)
			}
			size = 1 + (int64(randomBytes[0])%config.MaxFileSize)
			if size < 1 {
				size = 1
			}
			if size > config.MaxFileSize {
				size = config.MaxFileSize
			}
		}

		// Create file with buffered I/O for efficiency
		file, err := os.Create(filename)
		if err != nil {
			return fmt.Errorf("failed to create file %s: %w", filename, err)
		}

		// Use buffered writer for efficient I/O
		writer := bufio.NewWriter(file)

		// Generate and write random content
		content := make([]byte, size)
		if _, err := rand.Read(content); err != nil {
			file.Close()
			return fmt.Errorf("failed to generate random content: %w", err)
		}

		if _, err := writer.Write(content); err != nil {
			file.Close()
			return fmt.Errorf("failed to write content to %s: %w", filename, err)
		}

		// Flush buffered data to disk
		if err := writer.Flush(); err != nil {
			file.Close()
			return fmt.Errorf("failed to flush buffer for %s: %w", filename, err)
		}

		// Close file
		if err := file.Close(); err != nil {
			return fmt.Errorf("failed to close file %s: %w", filename, err)
		}
	}

	// Create subdirectories if we haven't reached max depth
	if depth < config.MaxDepth {
		// Create 2-3 subdirectories at each level
		numSubdirs := 2
		if depth < config.MaxDepth-1 {
			numSubdirs = 3
		}

		for i := 0; i < numSubdirs; i++ {
			subdir := filepath.Join(dir, fmt.Sprintf("subdir_%d_%d", depth, i))
			if err := os.MkdirAll(subdir, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", subdir, err)
			}

			// Recursively create files in subdirectory
			if err := GenerateTestTree(subdir, depth+1, filesPerDir, config); err != nil {
				return err
			}
		}
	}

	return nil
}

// CreateTestDirectoryWithTree creates a temporary directory with a nested
// directory structure according to the configuration.
func CreateTestDirectoryWithTree(t *testing.T, config TestConfig, filesPerDir int) string {
	t.Helper()

	dir := t.TempDir()

	if err := GenerateTestTree(dir, 0, filesPerDir, config); err != nil {
		t.Fatalf("Failed to generate test tree: %v", err)
	}

	return dir
}

// CountFiles recursively counts all files in a directory.
func CountFiles(dir string) (int, error) {
	count := 0
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			count++
		}
		return nil
	})
	return count, err
}

// GetMaxDepth returns the maximum depth of directories in a tree.
func GetMaxDepth(dir string) (int, error) {
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
		}
		return nil
	})

	return maxDepth, err
}
