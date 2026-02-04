//go:build windows

package scanner

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// TestReparsePointDetection tests that reparse points are correctly detected
// and classified by their tags.
func TestReparsePointDetection(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only test")
	}

	tests := []struct {
		name     string
		tag      uint32
		isDir    bool
		expected ReparseAction
	}{
		{
			name:     "Symlink",
			tag:      IO_REPARSE_TAG_SYMLINK,
			isDir:    false,
			expected: DeleteButDontTraverse,
		},
		{
			name:     "Directory Symlink",
			tag:      IO_REPARSE_TAG_SYMLINK,
			isDir:    true,
			expected: DeleteButDontTraverse,
		},
		{
			name:     "Junction (Mount Point Tag)",
			tag:      IO_REPARSE_TAG_MOUNT_POINT,
			isDir:    true,
			expected: DeleteButDontTraverse,
		},
		{
			name:     "OneDrive Placeholder",
			tag:      IO_REPARSE_TAG_ONEDRIVE,
			isDir:    false,
			expected: DeleteAndTraverse,
		},
		{
			name:     "Cloud Placeholder",
			tag:      IO_REPARSE_TAG_CLOUD,
			isDir:    false,
			expected: DeleteAndTraverse,
		},
		{
			name:     "Deduplication",
			tag:      IO_REPARSE_TAG_DEDUP,
			isDir:    false,
			expected: DeleteAndTraverse,
		},
		{
			name:     "WIM Mount",
			tag:      IO_REPARSE_TAG_WIM,
			isDir:    true,
			expected: SkipEntry,
		},
		{
			name:     "Unknown Reparse Point",
			tag:      0x12345678,
			isDir:    false,
			expected: SkipEntry,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action := handleReparsePoint("C:\\test\\path", tt.tag, tt.isDir)
			if action != tt.expected {
				t.Errorf("handleReparsePoint(%s, 0x%X, %v) = %v, want %v",
					"C:\\test\\path", tt.tag, tt.isDir, action, tt.expected)
			}
		})
	}
}

// TestSymlinkHandling tests that symlinks are detected and handled correctly.
// This test requires administrator privileges to create symlinks on Windows.
func TestSymlinkHandling(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only test")
	}

	// Check if we have admin privileges to create symlinks
	if !isAdmin() {
		t.Skip("Test requires administrator privileges to create symlinks")
	}

	tmpDir := t.TempDir()

	// Create a target file
	targetFile := filepath.Join(tmpDir, "target.txt")
	if err := os.WriteFile(targetFile, []byte("target content"), 0644); err != nil {
		t.Fatalf("Failed to create target file: %v", err)
	}

	// Create a symlink to the target file
	symlinkFile := filepath.Join(tmpDir, "symlink.txt")
	if err := os.Symlink(targetFile, symlinkFile); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	// Create a target directory
	targetDir := filepath.Join(tmpDir, "target_dir")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatalf("Failed to create target directory: %v", err)
	}

	// Create a file in the target directory
	targetDirFile := filepath.Join(targetDir, "file_in_target.txt")
	if err := os.WriteFile(targetDirFile, []byte("file in target"), 0644); err != nil {
		t.Fatalf("Failed to create file in target directory: %v", err)
	}

	// Create a directory symlink
	symlinkDir := filepath.Join(tmpDir, "symlink_dir")
	if err := os.Symlink(targetDir, symlinkDir); err != nil {
		t.Fatalf("Failed to create directory symlink: %v", err)
	}

	// Create scanner - scan the tmpDir
	scanner := NewParallelScanner(tmpDir, nil, 2)
	result, err := scanner.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Verify results
	scannedPaths := make(map[string]bool)
	for _, path := range result.Files {
		scannedPaths[path] = true
	}

	// The symlink files themselves should be in the scan result
	if !scannedPaths[symlinkFile] {
		t.Errorf("Symlink file not found in scan results")
	}
	if !scannedPaths[symlinkDir] {
		t.Errorf("Symlink directory not found in scan results")
	}

	// The file inside the target directory should NOT be in the scan result
	// (because we don't traverse into the symlink directory)
	if scannedPaths[targetDirFile] {
		t.Errorf("File inside symlinked directory was incorrectly scanned (symlink was followed)")
	}

	// The actual target directory and its contents should be in the scan
	// (because they are part of the original tree)
	if !scannedPaths[targetDir] {
		t.Errorf("Target directory not found in scan results")
	}

	// Target file should be there too
	if !scannedPaths[targetFile] {
		t.Errorf("Target file not found in scan results")
	}
}

// TestJunctionHandling tests that junctions are detected and handled correctly.
// Junctions don't require admin privileges on Windows.
func TestJunctionHandling(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only test")
	}

	tmpDir := t.TempDir()

	// Create a target directory
	targetDir := filepath.Join(tmpDir, "target_dir")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatalf("Failed to create target directory: %v", err)
	}

	// Create a file in the target directory
	targetFile := filepath.Join(targetDir, "file_in_target.txt")
	if err := os.WriteFile(targetFile, []byte("file in target"), 0644); err != nil {
		t.Fatalf("Failed to create file in target directory: %v", err)
	}

	// Create a junction using mklink /J
	junctionDir := filepath.Join(tmpDir, "junction_dir")
	cmd := exec.Command("cmd", "/C", "mklink", "/J", junctionDir, targetDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Skipf("Failed to create junction (requires appropriate permissions): %v, output: %s", err, output)
	}

	// Verify the junction was created
	info, err := os.Lstat(junctionDir)
	if err != nil {
		t.Fatalf("Junction was not created: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Logf("Warning: Junction created but not detected as symlink by Go's os package")
	}

	// Create scanner - scan the tmpDir
	scanner := NewParallelScanner(tmpDir, nil, 2)
	result, err := scanner.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Verify results
	scannedPaths := make(map[string]bool)
	for _, path := range result.Files {
		scannedPaths[path] = true
	}

	// The junction itself should be in the scan result
	if !scannedPaths[junctionDir] {
		t.Errorf("Junction directory not found in scan results")
	}

	// The file inside the target directory should NOT be scanned via the junction
	// (because we don't traverse into junctions)
	junctionFile := filepath.Join(junctionDir, "file_in_target.txt")
	if scannedPaths[junctionFile] {
		t.Errorf("File accessed via junction was incorrectly scanned (junction was traversed)")
	}

	// The actual target directory and file should be in the scan
	// (because they are part of the original tree)
	if !scannedPaths[targetDir] {
		t.Errorf("Target directory not found in scan results")
	}
	if !scannedPaths[targetFile] {
		t.Errorf("Target file not found in scan results")
	}
}

// TestCircularSymlinkNoInfiniteLoop tests that circular symlinks don't cause infinite loops.
func TestCircularSymlinkNoInfiniteLoop(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only test")
	}

	if !isAdmin() {
		t.Skip("Test requires administrator privileges to create symlinks")
	}

	tmpDir := t.TempDir()

	// Create two directories that symlink to each other
	dirA := filepath.Join(tmpDir, "dirA")
	dirB := filepath.Join(tmpDir, "dirB")

	if err := os.MkdirAll(dirA, 0755); err != nil {
		t.Fatalf("Failed to create dirA: %v", err)
	}
	if err := os.MkdirAll(dirB, 0755); err != nil {
		t.Fatalf("Failed to create dirB: %v", err)
	}

	// Create symlink from dirA to dirB
	symlinkAtoB := filepath.Join(dirA, "link_to_B")
	if err := os.Symlink(dirB, symlinkAtoB); err != nil {
		t.Fatalf("Failed to create symlink A->B: %v", err)
	}

	// Create symlink from dirB to dirA
	symlinkBtoA := filepath.Join(dirB, "link_to_A")
	if err := os.Symlink(dirA, symlinkBtoA); err != nil {
		t.Fatalf("Failed to create symlink B->A: %v", err)
	}

	// Create scanner with a timeout to detect infinite loops
	scanner := NewParallelScanner(tmpDir, nil, 2)

	// Run scan with a timeout
	done := make(chan struct{})
	var result *ScanResult
	var scanErr error

	go func() {
		result, scanErr = scanner.Scan()
		close(done)
	}()

	select {
	case <-done:
		// Scan completed - good!
		if scanErr != nil {
			t.Fatalf("Scan failed: %v", scanErr)
		}

		// Verify we didn't traverse the symlinks
		scannedPaths := make(map[string]bool)
		for _, path := range result.Files {
			scannedPaths[path] = true
		}

		// The symlinks themselves should be in the results
		if !scannedPaths[symlinkAtoB] {
			t.Errorf("Symlink A->B not found in scan results")
		}
		if !scannedPaths[symlinkBtoA] {
			t.Errorf("Symlink B->A not found in scan results")
		}

	case <-time.After(10 * time.Second):
		t.Fatal("Scan timed out - likely infinite loop due to circular symlinks")
	}
}

// TestReparsePointConstants verifies that our reparse tag constants are correct.
func TestReparsePointConstants(t *testing.T) {
	// Verify that our constants match the documented Windows values
	tests := []struct {
		name     string
		constant uint32
		expected uint32
	}{
		{"IO_REPARSE_TAG_SYMLINK", IO_REPARSE_TAG_SYMLINK, 0xA000000C},
		{"IO_REPARSE_TAG_MOUNT_POINT", IO_REPARSE_TAG_MOUNT_POINT, 0xA0000003},
		{"IO_REPARSE_TAG_DEDUP", IO_REPARSE_TAG_DEDUP, 0x80000013},
		{"IO_REPARSE_TAG_ONEDRIVE", IO_REPARSE_TAG_ONEDRIVE, 0x80000021},
		{"IO_REPARSE_TAG_CLOUD", IO_REPARSE_TAG_CLOUD, 0x9000001A},
		{"IO_REPARSE_TAG_WIM", IO_REPARSE_TAG_WIM, 0x80000008},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("%s = 0x%X, want 0x%X", tt.name, tt.constant, tt.expected)
			}
		})
	}
}

// isAdmin checks if the current process has administrator privileges.
// This is needed to create symlinks on Windows.
func isAdmin() bool {
	var sid *windows.SID
	err := windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY,
		2,
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0,
		&sid)
	if err != nil {
		return false
	}
	defer windows.FreeSid(sid)

	token := windows.Token(0)
	member, err := token.IsMember(sid)
	if err != nil {
		return false
	}
	return member
}

// TestReparsePointLogging tests that reparse points are logged appropriately.
func TestReparsePointLogging(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only test")
	}

	// This is a basic test to ensure the logging calls don't panic
	// In a real scenario, we'd want to capture and verify log output

	// Test each reparse action type
	actions := []struct {
		tag      uint32
		expected ReparseAction
	}{
		{IO_REPARSE_TAG_SYMLINK, DeleteButDontTraverse},
		{IO_REPARSE_TAG_MOUNT_POINT, DeleteButDontTraverse},
		{IO_REPARSE_TAG_ONEDRIVE, DeleteAndTraverse},
		{IO_REPARSE_TAG_WIM, SkipEntry},
		{0x99999999, SkipEntry}, // Unknown tag
	}

	for _, test := range actions {
		// This should not panic
		action := handleReparsePoint("C:\\test\\path", test.tag, false)
		if action != test.expected {
			t.Errorf("handleReparsePoint returned %v, expected %v for tag 0x%X",
				action, test.expected, test.tag)
		}
	}
}

// TestReparseActionEnum tests that the ReparseAction enum values are distinct.
func TestReparseActionEnum(t *testing.T) {
	// Ensure each action has a unique value
	actions := map[ReparseAction]string{
		SkipEntry:             "SkipEntry",
		DeleteButDontTraverse: "DeleteButDontTraverse",
		DeleteAndTraverse:     "DeleteAndTraverse",
	}

	// Verify all three values are different
	if SkipEntry == DeleteButDontTraverse {
		t.Error("SkipEntry and DeleteButDontTraverse have the same value")
	}
	if SkipEntry == DeleteAndTraverse {
		t.Error("SkipEntry and DeleteAndTraverse have the same value")
	}
	if DeleteButDontTraverse == DeleteAndTraverse {
		t.Error("DeleteButDontTraverse and DeleteAndTraverse have the same value")
	}

	// Verify they have expected sequential values (0, 1, 2)
	if SkipEntry != 0 {
		t.Errorf("SkipEntry = %d, expected 0", SkipEntry)
	}
	if DeleteButDontTraverse != 1 {
		t.Errorf("DeleteButDontTraverse = %d, expected 1", DeleteButDontTraverse)
	}
	if DeleteAndTraverse != 2 {
		t.Errorf("DeleteAndTraverse = %d, expected 2", DeleteAndTraverse)
	}

	// Log the actions for documentation
	for action, name := range actions {
		t.Logf("ReparseAction %s = %d", name, action)
	}
}

// TestMixedDirectoryWithReparsePoints tests scanning a directory containing
// normal files, directories, and various types of reparse points.
func TestMixedDirectoryWithReparsePoints(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only test")
	}

	tmpDir := t.TempDir()

	// Create normal file
	normalFile := filepath.Join(tmpDir, "normal.txt")
	if err := os.WriteFile(normalFile, []byte("normal"), 0644); err != nil {
		t.Fatalf("Failed to create normal file: %v", err)
	}

	// Create normal directory with file
	normalDir := filepath.Join(tmpDir, "normal_dir")
	if err := os.MkdirAll(normalDir, 0755); err != nil {
		t.Fatalf("Failed to create normal directory: %v", err)
	}
	normalDirFile := filepath.Join(normalDir, "file.txt")
	if err := os.WriteFile(normalDirFile, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create file in normal directory: %v", err)
	}

	// Create a junction (doesn't require admin)
	junctionTarget := filepath.Join(tmpDir, "junction_target")
	if err := os.MkdirAll(junctionTarget, 0755); err != nil {
		t.Fatalf("Failed to create junction target: %v", err)
	}
	junctionTargetFile := filepath.Join(junctionTarget, "target_file.txt")
	if err := os.WriteFile(junctionTargetFile, []byte("target"), 0644); err != nil {
		t.Fatalf("Failed to create file in junction target: %v", err)
	}

	junction := filepath.Join(tmpDir, "my_junction")
	cmd := exec.Command("cmd", "/C", "mklink", "/J", junction, junctionTarget)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Logf("Could not create junction (skipping junction test): %v, output: %s", err, output)
	}

	// Scan the directory
	scanner := NewParallelScanner(tmpDir, nil, 2)
	result, err := scanner.Scan()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Build set of scanned paths
	scannedPaths := make(map[string]bool)
	for _, path := range result.Files {
		scannedPaths[path] = true
	}

	// Verify normal files and directories are scanned
	if !scannedPaths[normalFile] {
		t.Errorf("Normal file not found in scan results")
	}
	if !scannedPaths[normalDir] {
		t.Errorf("Normal directory not found in scan results")
	}
	if !scannedPaths[normalDirFile] {
		t.Errorf("File in normal directory not found in scan results")
	}

	// Verify junction target directory and file are scanned (they're part of the tree)
	if !scannedPaths[junctionTarget] {
		t.Errorf("Junction target directory not found in scan results")
	}
	if !scannedPaths[junctionTargetFile] {
		t.Errorf("File in junction target not found in scan results")
	}

	// If junction was created, verify it's in results but not traversed
	if _, err := os.Lstat(junction); err == nil {
		if !scannedPaths[junction] {
			t.Errorf("Junction not found in scan results")
		}

		// File accessed via junction path should NOT be in results
		junctionFile := filepath.Join(junction, "target_file.txt")
		if scannedPaths[junctionFile] {
			t.Errorf("File accessed via junction path was incorrectly scanned (junction was traversed)")
		}
	}
}

// BenchmarkReparsePointDetection benchmarks the overhead of reparse point detection.
func BenchmarkReparsePointDetection(b *testing.B) {
	if runtime.GOOS != "windows" {
		b.Skip("Windows-only benchmark")
	}

	path := "C:\\test\\path"
	tags := []uint32{
		IO_REPARSE_TAG_SYMLINK,
		IO_REPARSE_TAG_MOUNT_POINT,
		IO_REPARSE_TAG_ONEDRIVE,
		IO_REPARSE_TAG_DEDUP,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tag := tags[i%len(tags)]
		_ = handleReparsePoint(path, tag, false)
	}
}
