# Product Overview

Fast File Deletion (FFD) is a high-performance command-line tool for deleting large numbers of files, optimized for Windows systems.

## Core Purpose

Solve the Windows filesystem bottleneck when deleting directories containing millions of small files. FFD achieves 2-3x faster deletion rates compared to traditional methods (PowerShell, Windows Explorer, rmdir).

## Key Performance Characteristics

- **Speed**: 600-800 files/sec on Windows systems
- **Parallelism**: True concurrent deletion using Go goroutines
- **Windows Optimization**: Direct Win32 API calls (DeleteFile, RemoveDirectory) via golang.org/x/sys/windows
- **Cross-platform**: Works on Linux/macOS with standard operations (no platform-specific optimizations)

## Primary Features

1. **Safety-first design**: Protected paths, confirmation prompts, exact path verification
2. **Age-based filtering**: Keep recent files while deleting old data (--keep-days)
3. **Real-time progress**: Live deletion rate, progress percentage, ETA
4. **Dry-run mode**: Preview deletions without executing
5. **Error resilience**: Continues deletion even when individual files fail
6. **Graceful interruption**: Ctrl+C stops cleanly with progress report

## Target Users

Developers and system administrators dealing with:
- Large node_modules directories
- Build artifact cleanup
- Log file management
- Cache directory maintenance
- Any scenario with millions of small files on Windows
