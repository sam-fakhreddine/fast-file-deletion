# Windows File Deletion Optimization - Agentic Master Prompt

You are an expert systems programmer specializing in Windows internals, file system operations, and high-performance Go development. Your task is to implement a comprehensive set of optimizations to maximize file deletion speed on Windows for the Fast File Deletion (FFD) tool.

## Project Context

This is a Go-based high-performance file deletion tool that currently achieves 659-790 files/sec on Windows. The goal is to push this significantly higher through Windows-specific optimizations.

### Current Architecture

```
fastDelete/
├── cmd/fast-file-deletion/main.go          # CLI entry point
├── internal/
│   ├── backend/
│   │   ├── backend.go                      # Backend interface
│   │   ├── windows.go                      # Windows implementation (OPTIMIZE THIS)
│   │   ├── generic.go                      # Cross-platform fallback
│   │   ├── factory_windows.go              # Windows factory
│   │   └── factory_generic.go              # Generic factory
│   ├── engine/
│   │   └── engine.go                       # Deletion engine with worker pool (OPTIMIZE THIS)
│   ├── scanner/
│   │   └── scanner.go                      # Directory traversal (OPTIMIZE THIS)
│   ├── safety/
│   │   └── validator.go                    # Path validation
│   ├── progress/
│   │   └── reporter.go                     # Progress reporting
│   └── logger/
│       └── logger.go                       # Structured logging
└── go.mod
```

### Current Implementation Details

**Backend Interface (`backend.go`):**
```go
type Backend interface {
    DeleteFile(path string) error
    DeleteDirectory(path string) error
}
```

**Current Windows Backend (`windows.go`):**
- Uses `windows.DeleteFile()` and `windows.RemoveDirectory()` Win32 APIs
- Converts paths to extended-length format (`\\?\`) for long path support
- Converts paths to UTF-16 for each deletion call

**Current Engine (`engine.go`):**
- Worker pool pattern with `NumCPU * 2` goroutines
- Buffered channel of size 100 for work distribution
- Uses `sync.Mutex` for all counter updates
- Tries `DeleteFile` first, then `DeleteDirectory` on failure

**Current Scanner (`scanner.go`):**
- Uses `filepath.WalkDir` for single-threaded traversal
- Orders files bottom-up (files before parent directories)
- Supports age-based filtering via modification time

---

## Optimization Tasks

Implement ALL of the following optimizations. Each optimization should be implemented carefully with proper error handling and fallback mechanisms.

---

### TASK 1: Implement `SetFileInformationByHandle` Deletion Method (HIGH PRIORITY)

**File to modify:** `internal/backend/windows.go`

**What to implement:**
Replace the current `windows.DeleteFile()` approach with the more efficient `SetFileInformationByHandle` method using `FileDispositionInfo` or `FileDispositionInfoEx`.

**Implementation details:**

```go
import (
    "golang.org/x/sys/windows"
    "unsafe"
)

// FileDispositionInfo constants
const (
    FileDispositionInfo   = 4
    FileDispositionInfoEx = 64
)

// FILE_DISPOSITION_INFO structure
type FILE_DISPOSITION_INFO struct {
    DeleteFile bool
}

// FILE_DISPOSITION_INFO_EX structure (Windows 10 RS1+)
type FILE_DISPOSITION_INFO_EX struct {
    Flags uint32
}

const (
    FILE_DISPOSITION_FLAG_DELETE                  = 0x00000001
    FILE_DISPOSITION_FLAG_POSIX_SEMANTICS         = 0x00000002
    FILE_DISPOSITION_FLAG_FORCE_IMAGE_SECTION_CHECK = 0x00000004
    FILE_DISPOSITION_FLAG_IGNORE_READONLY_ATTRIBUTE = 0x00000008
)

func (b *WindowsBackend) DeleteFileOptimized(path string) error {
    extendedPath := toExtendedLengthPath(path)
    pathPtr, err := syscall.UTF16PtrFromString(extendedPath)
    if err != nil {
        return err
    }

    // Open with DELETE access and necessary flags
    handle, err := windows.CreateFile(
        pathPtr,
        windows.DELETE,
        windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE | windows.FILE_SHARE_DELETE,
        nil,
        windows.OPEN_EXISTING,
        windows.FILE_FLAG_OPEN_REPARSE_POINT | windows.FILE_FLAG_BACKUP_SEMANTICS,
        0,
    )
    if err != nil {
        return err
    }
    defer windows.CloseHandle(handle)

    // Try FileDispositionInfoEx first (faster, handles read-only)
    infoEx := FILE_DISPOSITION_INFO_EX{
        Flags: FILE_DISPOSITION_FLAG_DELETE |
               FILE_DISPOSITION_FLAG_POSIX_SEMANTICS |
               FILE_DISPOSITION_FLAG_IGNORE_READONLY_ATTRIBUTE,
    }

    err = windows.SetFileInformationByHandle(
        handle,
        FileDispositionInfoEx,
        (*byte)(unsafe.Pointer(&infoEx)),
        uint32(unsafe.Sizeof(infoEx)),
    )

    if err != nil {
        // Fallback to basic FileDispositionInfo
        info := FILE_DISPOSITION_INFO{DeleteFile: true}
        err = windows.SetFileInformationByHandle(
            handle,
            FileDispositionInfo,
            (*byte)(unsafe.Pointer(&info)),
            uint32(unsafe.Sizeof(info)),
        )
    }

    return err
}
```

**Why this is faster:**
- POSIX semantics allow immediate file name reuse
- Handles read-only files without separate attribute change
- Kernel-optimized path for modern Windows versions
- Single syscall instead of multiple

**Fallback strategy:**
If `FileDispositionInfoEx` fails (older Windows), fall back to `FileDispositionInfo`, then to original `DeleteFile`.

---

### TASK 2: Implement Alternative `FILE_FLAG_DELETE_ON_CLOSE` Method (HIGH PRIORITY)

**File to modify:** `internal/backend/windows.go`

**What to implement:**
Add an alternative deletion method using `FILE_FLAG_DELETE_ON_CLOSE` flag.

```go
func (b *WindowsBackend) DeleteFileOnClose(path string) error {
    extendedPath := toExtendedLengthPath(path)
    pathPtr, err := syscall.UTF16PtrFromString(extendedPath)
    if err != nil {
        return err
    }

    handle, err := windows.CreateFile(
        pathPtr,
        windows.DELETE,
        windows.FILE_SHARE_DELETE,
        nil,
        windows.OPEN_EXISTING,
        windows.FILE_FLAG_DELETE_ON_CLOSE | windows.FILE_FLAG_OPEN_REPARSE_POINT,
        0,
    )
    if err != nil {
        return err
    }

    // File is automatically deleted when handle is closed
    return windows.CloseHandle(handle)
}
```

**Benchmark both methods** and use the faster one as default.

---

### TASK 3: Implement Native API (`NtDeleteFile`) Option (HIGH PRIORITY)

**File to modify:** `internal/backend/windows.go`

**What to implement:**
Add direct NT Native API calls for maximum performance.

```go
import (
    "golang.org/x/sys/windows"
    "unsafe"
)

var (
    ntdll              = windows.NewLazySystemDLL("ntdll.dll")
    procNtDeleteFile   = ntdll.NewProc("NtDeleteFile")
    procRtlInitUnicodeString = ntdll.NewProc("RtlInitUnicodeString")
)

// UNICODE_STRING structure
type UNICODE_STRING struct {
    Length        uint16
    MaximumLength uint16
    Buffer        *uint16
}

// OBJECT_ATTRIBUTES structure
type OBJECT_ATTRIBUTES struct {
    Length                   uint32
    RootDirectory            windows.Handle
    ObjectName               *UNICODE_STRING
    Attributes               uint32
    SecurityDescriptor       *byte
    SecurityQualityOfService *byte
}

const (
    OBJ_CASE_INSENSITIVE = 0x00000040
)

func (b *WindowsBackend) NtDeleteFile(path string) error {
    extendedPath := toExtendedLengthPath(path)
    pathPtr, err := syscall.UTF16PtrFromString(extendedPath)
    if err != nil {
        return err
    }

    var unicodeString UNICODE_STRING
    procRtlInitUnicodeString.Call(
        uintptr(unsafe.Pointer(&unicodeString)),
        uintptr(unsafe.Pointer(pathPtr)),
    )

    objectAttributes := OBJECT_ATTRIBUTES{
        Length:     uint32(unsafe.Sizeof(OBJECT_ATTRIBUTES{})),
        ObjectName: &unicodeString,
        Attributes: OBJ_CASE_INSENSITIVE,
    }

    ret, _, _ := procNtDeleteFile.Call(
        uintptr(unsafe.Pointer(&objectAttributes)),
    )

    if ret != 0 {
        return fmt.Errorf("NtDeleteFile failed with status: 0x%X", ret)
    }
    return nil
}
```

**Note:** NT paths require `\??\C:\path` format instead of `\\?\C:\path`.

---

### TASK 4: Parallel Directory Scanning with `FindFirstFileEx` (HIGH PRIORITY)

**File to modify:** `internal/scanner/scanner.go`

**What to implement:**
Replace `filepath.WalkDir` with parallel scanning using Windows-native APIs.

```go
import (
    "golang.org/x/sys/windows"
    "sync"
)

// ParallelScanner performs multi-threaded directory enumeration
type ParallelScanner struct {
    rootPath     string
    keepDays     *int
    workerCount  int
    results      chan ScanItem
    errors       chan error
}

type ScanItem struct {
    Path        string
    IsDirectory bool
    Size        int64
    ModTime     time.Time
    UTF16Path   *uint16  // Pre-converted for deletion phase
}

func (s *ParallelScanner) ScanParallel() (*ScanResult, error) {
    dirQueue := make(chan string, 10000)
    var wg sync.WaitGroup

    // Start worker goroutines
    for i := 0; i < s.workerCount; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            s.scanWorker(dirQueue)
        }()
    }

    // Seed with root directory
    dirQueue <- s.rootPath

    // ... coordinate workers and collect results
}

func (s *ParallelScanner) scanWorker(dirQueue chan string) {
    for dir := range dirQueue {
        s.scanDirectory(dir, dirQueue)
    }
}

func (s *ParallelScanner) scanDirectory(dir string, dirQueue chan string) {
    pattern := dir + `\*`
    patternPtr, _ := syscall.UTF16PtrFromString(pattern)

    var findData windows.Win32finddata

    handle, err := windows.FindFirstFileEx(
        patternPtr,
        windows.FindExInfoBasic,           // Skip 8.3 short names - faster
        &findData,
        windows.FindExSearchNameMatch,
        nil,
        windows.FIND_FIRST_EX_LARGE_FETCH, // Optimize for large directories
    )
    if err != nil {
        return
    }
    defer windows.FindClose(handle)

    for {
        name := syscall.UTF16ToString(findData.FileName[:])

        // Skip . and ..
        if name != "." && name != ".." {
            fullPath := dir + `\` + name

            if findData.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0 {
                // Queue subdirectory for parallel scanning
                select {
                case dirQueue <- fullPath:
                default:
                    // Queue full, scan inline
                    s.scanDirectory(fullPath, dirQueue)
                }
            } else {
                // Process file
                s.processFile(fullPath, &findData)
            }
        }

        err = windows.FindNextFile(handle, &findData)
        if err != nil {
            break
        }
    }
}
```

**Key optimizations:**
- `FindExInfoBasic`: Skips retrieving 8.3 short filenames (significant speedup)
- `FIND_FIRST_EX_LARGE_FETCH`: Optimizes buffer sizes for large directories
- Parallel scanning of independent directory subtrees
- Pre-convert paths to UTF-16 during scan phase

---

### TASK 5: Pre-convert Paths to UTF-16 (MEDIUM PRIORITY)

**Files to modify:** `internal/scanner/scanner.go`, `internal/engine/engine.go`, `internal/backend/windows.go`

**What to implement:**
Convert paths to UTF-16 once during scanning, reuse during deletion.

```go
// In scanner.go - new structure
type DeletionTask struct {
    OriginalPath string
    UTF16Path    *uint16
    IsDirectory  bool
    Size         int64
}

// In scanner - during scan
func (s *Scanner) createTask(path string, isDir bool, size int64) DeletionTask {
    extPath := toExtendedLengthPath(path)
    utf16Ptr, _ := syscall.UTF16PtrFromString(extPath)

    return DeletionTask{
        OriginalPath: path,
        UTF16Path:    utf16Ptr,
        IsDirectory:  isDir,
        Size:         size,
    }
}

// In backend - use pre-converted path
func (b *WindowsBackend) DeleteFilePreConverted(utf16Path *uint16) error {
    return windows.DeleteFile(utf16Path)
}
```

---

### TASK 6: Use Atomic Counters Instead of Mutex (MEDIUM PRIORITY)

**File to modify:** `internal/engine/engine.go`

**What to implement:**
Replace mutex-protected counters with lock-free atomic operations.

```go
import "sync/atomic"

type DeletionResult struct {
    deletedCount    int64       // Use atomic operations
    failedCount     int64       // Use atomic operations
    Errors          []FileError // Still needs mutex for slice
    errorMu         sync.Mutex  // Only for error slice
    DurationSeconds float64
}

// Getters for external access
func (r *DeletionResult) DeletedCount() int64 {
    return atomic.LoadInt64(&r.deletedCount)
}

func (r *DeletionResult) FailedCount() int64 {
    return atomic.LoadInt64(&r.failedCount)
}

// In worker function
func (e *Engine) worker(ctx context.Context, workChan <-chan DeletionTask, dryRun bool, result *DeletionResult) {
    for {
        select {
        case <-ctx.Done():
            return
        case task, ok := <-workChan:
            if !ok {
                return
            }

            err := e.deleteItem(task, dryRun)

            if err != nil {
                atomic.AddInt64(&result.failedCount, 1)

                // Only lock for error slice modification
                result.errorMu.Lock()
                result.Errors = append(result.Errors, FileError{
                    Path:  task.OriginalPath,
                    Error: err.Error(),
                })
                result.errorMu.Unlock()
            } else {
                atomic.AddInt64(&result.deletedCount, 1)

                // Progress callback using atomic load
                if e.progressCallback != nil {
                    e.progressCallback(int(atomic.LoadInt64(&result.deletedCount)))
                }
            }
        }
    }
}
```

---

### TASK 7: Skip Double-Call Pattern with IsDirectory Flag (MEDIUM PRIORITY)

**Files to modify:** `internal/engine/engine.go`, `internal/backend/backend.go`

**What to implement:**
Pass directory information to avoid trying file deletion on directories.

```go
// Update backend interface
type Backend interface {
    DeleteFile(path string) error
    DeleteDirectory(path string) error
    DeleteItem(task DeletionTask) error  // New unified method
}

// In windows.go
func (b *WindowsBackend) DeleteItem(task DeletionTask) error {
    if task.IsDirectory {
        return b.deleteDirectoryInternal(task.UTF16Path)
    }
    return b.deleteFileInternal(task.UTF16Path)
}

// Internal methods that use pre-converted UTF-16 paths
func (b *WindowsBackend) deleteFileInternal(utf16Path *uint16) error {
    // Use SetFileInformationByHandle or other optimized method
    // ...
}

func (b *WindowsBackend) deleteDirectoryInternal(utf16Path *uint16) error {
    return windows.RemoveDirectory(utf16Path)
}
```

---

### TASK 8: Increase and Auto-Tune Worker Count (MEDIUM PRIORITY)

**File to modify:** `internal/engine/engine.go`

**What to implement:**
Implement adaptive worker count based on I/O characteristics.

```go
const (
    MinWorkers     = 4
    MaxWorkers     = 128
    DefaultWorkers = 0  // Auto-detect
)

func (e *Engine) autoDetectWorkers() int {
    cpuCount := runtime.NumCPU()

    // File deletion is I/O bound, not CPU bound
    // Use higher multiplier for SSDs
    baseWorkers := cpuCount * 4

    // Clamp to reasonable range
    if baseWorkers < MinWorkers {
        return MinWorkers
    }
    if baseWorkers > MaxWorkers {
        return MaxWorkers
    }

    return baseWorkers
}

// Optional: Adaptive tuning during execution
func (e *Engine) adaptiveWorkerTuning(result *DeletionResult) {
    // Monitor deletion rate
    // If rate plateaus, try adjusting worker count
    // Track optimal worker count for future runs
}
```

**Add CLI flag:**
```go
--workers N    // 0 = auto-detect (default)
               // Recommended: 32-64 for SSD, 8-16 for HDD
```

---

### TASK 9: Increase Work Channel Buffer (MEDIUM PRIORITY)

**File to modify:** `internal/engine/engine.go`

**What to implement:**
Dynamic buffer sizing based on file count.

```go
func (e *Engine) Delete(ctx context.Context, tasks []DeletionTask, dryRun bool) (*DeletionResult, error) {
    // Dynamic buffer size: min(fileCount, 10000)
    bufferSize := len(tasks)
    if bufferSize > 10000 {
        bufferSize = 10000
    }
    if bufferSize < 100 {
        bufferSize = 100
    }

    workChan := make(chan DeletionTask, bufferSize)

    // ... rest of implementation
}
```

---

### TASK 10: Handle Read-Only Files Automatically (MEDIUM PRIORITY)

**File to modify:** `internal/backend/windows.go`

**What to implement:**
Clear read-only attribute before deletion if needed.

```go
func (b *WindowsBackend) DeleteFileWithRetry(path string) error {
    utf16Path, _ := syscall.UTF16PtrFromString(toExtendedLengthPath(path))

    // First attempt - optimized deletion
    err := b.deleteFileOptimized(utf16Path)
    if err == nil {
        return nil
    }

    // Check if error is due to read-only attribute
    if isAccessDenied(err) {
        // Clear read-only and other restrictive attributes
        err = windows.SetFileAttributes(utf16Path, windows.FILE_ATTRIBUTE_NORMAL)
        if err != nil {
            return fmt.Errorf("failed to clear attributes: %w", err)
        }

        // Retry deletion
        return b.deleteFileOptimized(utf16Path)
    }

    return err
}

func isAccessDenied(err error) bool {
    if errno, ok := err.(syscall.Errno); ok {
        return errno == windows.ERROR_ACCESS_DENIED
    }
    return false
}
```

**Note:** If using `FileDispositionInfoEx` with `FILE_DISPOSITION_FLAG_IGNORE_READONLY_ATTRIBUTE`, this is handled automatically.

---

### TASK 11: Implement Benchmarking Mode (LOW PRIORITY)

**File to create:** `internal/benchmark/benchmark.go`

**What to implement:**
A benchmarking system to compare deletion methods.

```go
package benchmark

type DeletionMethod int

const (
    MethodWin32DeleteFile DeletionMethod = iota
    MethodSetFileInfo
    MethodDeleteOnClose
    MethodNtDeleteFile
)

type BenchmarkResult struct {
    Method        DeletionMethod
    FilesDeleted  int
    Duration      time.Duration
    FilesPerSec   float64
    ErrorCount    int
}

func RunBenchmark(testDir string, methods []DeletionMethod) []BenchmarkResult {
    // Create test files
    // Run each method
    // Report results
}
```

Add CLI flag: `--benchmark` to run performance comparison.

---

### TASK 12: Batch Operations for Small Files (LOW PRIORITY)

**File to modify:** `internal/engine/engine.go`

**What to implement:**
Group small files for batch processing to reduce syscall overhead.

```go
const (
    SmallFileBatchSize = 100
    SmallFileThreshold = 4096  // bytes
)

func (e *Engine) processBatch(tasks []DeletionTask) error {
    // For very small files, process in tight loop
    // Minimizes goroutine scheduling overhead
    for _, task := range tasks {
        e.backend.DeleteItem(task)
    }
    return nil
}
```

---

## Implementation Order

Execute tasks in this order for maximum incremental benefit:

1. **TASK 1** - `SetFileInformationByHandle` (biggest single improvement)
2. **TASK 4** - Parallel scanning with `FindFirstFileEx`
3. **TASK 7** - Skip double-call with IsDirectory flag
4. **TASK 5** - Pre-convert UTF-16 paths
5. **TASK 6** - Atomic counters
6. **TASK 8** - Increase worker count
7. **TASK 9** - Larger channel buffer
8. **TASK 10** - Handle read-only files
9. **TASK 2** - `FILE_FLAG_DELETE_ON_CLOSE` alternative
10. **TASK 3** - Native API option
11. **TASK 11** - Benchmarking mode
12. **TASK 12** - Batch operations

---

## Testing Requirements

For each optimization:

1. **Unit tests** - Test the new functionality in isolation
2. **Integration tests** - Test with real file systems
3. **Benchmark tests** - Compare before/after performance
4. **Edge cases:**
   - Long paths (>260 characters)
   - Read-only files
   - Files in use
   - Symbolic links / junction points
   - Empty directories
   - Deeply nested directories
   - Unicode file names
   - Files with special characters

---

## Expected Performance Targets

| Metric | Current | Target |
|--------|---------|--------|
| Files/sec (SSD) | 659-790 | 1500-2000+ |
| Files/sec (HDD) | ~300 | 500-700 |
| Scan time (1M files) | ~30s | ~10s |
| Memory usage | Moderate | Optimized |

---

## Code Quality Requirements

- All new code must have proper error handling
- Fallback mechanisms for older Windows versions
- No panics - graceful degradation
- Proper resource cleanup (handles, memory)
- Comprehensive logging at DEBUG level
- Maintain backward compatibility with existing CLI interface
- Build tags for Windows-specific code (`//go:build windows`)

---

## Additional Windows System Optimizations (Document for Users)

Include in README or help:

```markdown
## System-Level Optimizations

For maximum deletion performance, consider these Windows settings:

1. **Disable Last Access Time updates:**
   ```cmd
   fsutil behavior set DisableLastAccess 1
   ```

2. **Disable 8.3 short name generation:**
   ```cmd
   fsutil behavior set disable8dot3 1
   ```

3. **Ensure SSD TRIM is enabled:**
   ```cmd
   fsutil behavior set DisableDeleteNotify 0
   ```

4. **Run from elevated command prompt** for certain protected files

5. **Exclude target directory from antivirus** real-time scanning
```

---

## Final Notes

- Always test on Windows 10/11 and Windows Server 2016+
- Some optimizations require Windows 10 RS1 (1607) or later
- Provide graceful fallbacks for older Windows versions
- Profile with Windows Performance Analyzer for bottleneck identification
- Consider NUMA topology for very large operations on server hardware

Begin implementation with TASK 1 and proceed sequentially through the task list.
