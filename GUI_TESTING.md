# GUI Testing Guide

## Overview

This document provides comprehensive testing procedures for the Fast File Deletion GUI application built with Wails v3.

## Prerequisites

- Windows 11 (primary target platform)
- Node.js 18+ and npm
- Go 1.25.5+
- Wails v3 CLI installed (`make gui-install-wails`)

## Quick Start

```bash
# First-time setup
make gui-setup

# Development mode (hot reload)
make gui-dev

# Production build
make gui-build
```

## Testing Checklist

### 1. Installation & Setup

- [ ] Wails CLI installs correctly (`make gui-install-wails`)
- [ ] Frontend dependencies install without errors (`make gui-deps`)
- [ ] Bindings generate successfully (`make gui-bindings`)
- [ ] Development server starts (`make gui-dev`)
- [ ] Production build completes (`make gui-build`)

### 2. Configuration Form

#### Path Validation
- [ ] Browse button opens native Windows directory picker
- [ ] Invalid paths show error message immediately
- [ ] Protected paths (C:\Windows, C:\Program Files) are rejected
- [ ] Network paths (\\server\share) are validated correctly
- [ ] UNC paths work properly
- [ ] Very long paths (>260 chars) are handled

#### Form Controls
- [ ] All 12 configuration options are accessible
- [ ] Checkboxes toggle correctly
- [ ] Spin buttons increment/decrement properly
- [ ] Dropdown shows all deletion methods
- [ ] Advanced options collapse/expand smoothly
- [ ] Form resets to defaults with Reset button

#### Validation Logic
- [ ] Benchmark + Dry Run combination is rejected
- [ ] Benchmark + Keep Days combination is rejected
- [ ] Empty target directory shows error
- [ ] Workers set to 0 shows "auto-detect: CPU × 4"
- [ ] Buffer size set to 0 shows "auto-detect"

### 3. Scanning Phase

- [ ] Spinner displays during scan
- [ ] "Scanning directory..." message shows
- [ ] Scan completes for small directories (100 files)
- [ ] Scan completes for medium directories (10,000 files)
- [ ] Scan completes for large directories (1,000,000+ files)
- [ ] Deep directory structures (depth 10+) scan correctly
- [ ] Scan respects age filter (keep-days option)
- [ ] Scan errors are displayed properly

### 4. Confirmation Dialog

#### Display
- [ ] Modal appears after successful scan
- [ ] Target directory path is displayed correctly
- [ ] File count is formatted with commas (e.g., "1,023,444")
- [ ] Total size is shown in human-readable format (MB/GB)
- [ ] Retained files count shown when age filter active
- [ ] Dry run mode shows blue info banner
- [ ] Normal mode shows red warning banner

#### Actions
- [ ] Cancel button returns to configuration
- [ ] Delete/Preview button starts operation
- [ ] ESC key closes dialog
- [ ] Clicking outside dialog closes it (if supported)

### 5. Progress View

#### Progress Display
- [ ] Progress bar animates smoothly (0-100%)
- [ ] Percentage text updates in real-time
- [ ] Files deleted counter updates live
- [ ] Deletion rate (files/sec) updates smoothly
- [ ] Elapsed time increments correctly
- [ ] ETA calculation is reasonable
- [ ] Windows taskbar progress bar updates (Windows 11)

#### Live Chart
- [ ] Deletion rate chart displays after 2 data points
- [ ] Chart shows last 60 seconds of data
- [ ] X-axis shows time in seconds
- [ ] Y-axis shows files/sec
- [ ] Chart scrolls as new data arrives
- [ ] No animation lag or stutter

#### System Monitoring (if enabled)
- [ ] CPU percentage updates every 200ms
- [ ] Memory usage (MB) displays correctly
- [ ] I/O operations/sec shown
- [ ] Goroutine count displayed
- [ ] Saturation warnings appear when thresholds exceeded:
  - [ ] CPU > 90%
  - [ ] Memory pressure (95%)
  - [ ] GC pressure (>2.0)
- [ ] System metrics chart renders
- [ ] Accordion expands/collapses smoothly

#### Cancellation
- [ ] Cancel button shows confirmation dialog
- [ ] "Yes, Cancel" stops deletion immediately
- [ ] "No, Continue" resumes operation
- [ ] Partial results are displayed after cancellation

### 6. Results View

#### Statistics Display
- [ ] All counters formatted with commas
- [ ] Duration shown in human-readable format (e.g., "36m 39s")
- [ ] Average rate calculated correctly
- [ ] Peak rate displayed
- [ ] Success icon shown when no failures
- [ ] Warning icon shown when some files failed
- [ ] Retained files count shown (if age filter active)

#### Method Breakdown (Windows only)
- [ ] FileInfo method count and percentage
- [ ] DeleteOnClose method count and percentage
- [ ] NtAPI method count and percentage
- [ ] Fallback method count and percentage
- [ ] Percentages add up to 100%

#### Bottleneck Report (if monitoring enabled)
- [ ] Report displayed in monospace font
- [ ] Contains insights about CPU/memory/I/O
- [ ] Recommendations provided
- [ ] Report is readable and well-formatted

#### Error List
- [ ] Error accordion collapsed by default (if >10 errors)
- [ ] Error accordion expanded by default (if <10 errors)
- [ ] First 50 errors shown initially
- [ ] "Show All X Errors" button appears when >50 errors
- [ ] All errors displayed after clicking button
- [ ] Errors are in monospace font
- [ ] Error list is scrollable

#### Actions
- [ ] "Delete More Files" button resets to configuration
- [ ] Taskbar progress cleared after reset

### 7. Windows 11 Aesthetic

#### Visual Design
- [ ] Dark theme applied by default (if system prefers dark)
- [ ] Light theme applied if system prefers light
- [ ] Theme switches automatically when system theme changes
- [ ] All cards have rounded corners (8px)
- [ ] Cards have subtle shadow elevation
- [ ] Hover effects on cards (slight lift)
- [ ] Blur effects visible on dialogs (acrylic/mica)
- [ ] Smooth transitions on all interactive elements (cubic-bezier)

#### Typography
- [ ] Segoe UI font used throughout
- [ ] Font sizes are appropriate and readable
- [ ] Font weights match Windows 11 style
- [ ] Hero-sized numbers for key stats

#### Colors
- [ ] Brand color (blue) used consistently
- [ ] Success messages use green palette
- [ ] Warning messages use yellow palette
- [ ] Error messages use red palette
- [ ] Neutral grays for secondary text

### 8. Edge Cases & Stress Tests

#### Large Scale
- [ ] 1,000,000+ files delete successfully
- [ ] Progress updates don't cause UI lag
- [ ] Memory usage stays under 1GB
- [ ] Application remains responsive throughout
- [ ] No freezing or stuttering

#### Directory Structures
- [ ] Very deep directories (depth 20+)
- [ ] Directories with Unicode characters in names
- [ ] Directories with spaces in names
- [ ] Directories with special characters (!, @, #)
- [ ] Empty directories
- [ ] Nested empty directories

#### File Types
- [ ] Small files (0 bytes)
- [ ] Large files (>1 GB)
- [ ] Read-only files (should fail gracefully)
- [ ] Files in use (should fail gracefully)
- [ ] Symbolic links (should handle correctly)
- [ ] Junction points (Windows)

#### Error Scenarios
- [ ] Network drive disconnects during deletion
- [ ] Disk full during operation
- [ ] Insufficient permissions
- [ ] Path becomes invalid mid-operation
- [ ] Application closed during deletion (cleanup handled)
- [ ] Multiple instances prevented

#### Performance
- [ ] Deletion rate matches CLI (400-1,200 files/sec)
- [ ] No memory leaks during long operations
- [ ] Event throttling prevents flooding (100ms throttle)
- [ ] Chart updates don't cause performance issues
- [ ] Monitoring overhead is minimal (<5% CPU)

### 9. Accessibility

- [ ] All interactive elements keyboard-accessible
- [ ] Tab navigation works correctly
- [ ] Focus visible on focused elements
- [ ] Screen reader friendly (ARIA labels)
- [ ] High contrast mode supported
- [ ] Reduced motion respected (prefers-reduced-motion)

### 10. Regression Tests

After making changes, verify:

- [ ] CLI still works unchanged (`bin/ffd.exe`)
- [ ] GUI and CLI produce same results for same config
- [ ] Bindings regenerate without errors
- [ ] Frontend builds without TypeScript errors
- [ ] Backend tests still pass (`make test`)
- [ ] Cross-platform compilation works (`make verify-all`)

## Testing Scenarios

### Scenario 1: Basic Deletion
1. Launch GUI
2. Select directory with 1,000 files
3. Enable dry-run
4. Scan directory
5. Confirm preview
6. Verify all 1,000 files listed
7. Verify no actual deletion occurred

### Scenario 2: Filtered Deletion
1. Select directory with mix of old and new files
2. Set keep-days to 30
3. Scan directory
4. Verify scan result shows retained count
5. Confirm deletion
6. Verify only old files deleted

### Scenario 3: Monitoring
1. Enable system monitoring
2. Select large directory (100,000+ files)
3. Scan and start deletion
4. Open monitoring accordion
5. Verify metrics update every 200ms
6. Verify charts render correctly
7. Check for bottleneck warnings

### Scenario 4: Cancellation
1. Select very large directory (1,000,000+ files)
2. Start deletion
3. Wait until 30% complete
4. Click Cancel
5. Confirm cancellation
6. Verify operation stops within 1 second
7. Verify partial results displayed

### Scenario 5: Error Handling
1. Select directory with read-only files
2. Disable force mode
3. Start deletion
4. Verify errors appear in error list
5. Verify successful deletions still counted
6. Verify error messages are descriptive

### Scenario 6: Benchmark Mode (Windows only)
1. Select directory with 10,000 files
2. Enable benchmark mode
3. Start operation
4. Verify method breakdown shows all methods
5. Verify performance comparison in results

## Performance Benchmarks

Target metrics (measured on Windows 11, SSD):

| Metric | Target | Actual | Status |
|--------|--------|--------|--------|
| Small files (<1KB) | 800-1,200/sec | _____/sec | ☐ |
| Medium files (1-100KB) | 400-800/sec | _____/sec | ☐ |
| Large files (>100KB) | 200-400/sec | _____/sec | ☐ |
| Startup time | <2 seconds | _____ sec | ☐ |
| Memory usage | <1 GB | _____ MB | ☐ |
| CPU usage | <80% (avg) | _____ % | ☐ |

## Known Limitations

1. **Windows Only (Primary)**: Full feature set requires Windows 10+
2. **Disk I/O Bound**: Performance limited by disk speed, not CPU
3. **Bundle Size**: 970 KB JavaScript (274 KB gzipped) is expected for React + Fluent UI
4. **Single Directory**: Can only process one directory at a time
5. **No Undo**: Deleted files cannot be recovered (by design)

## Reporting Issues

When reporting GUI bugs, include:

1. Windows version (e.g., Windows 11 22H2)
2. Application version (e.g., v0.16.0)
3. Steps to reproduce
4. Expected vs actual behavior
5. Screenshots or screen recording
6. Console output (if applicable)
7. Config options used

## Development Testing

### Hot Reload Testing
```bash
make gui-dev
# Make changes to frontend/src/
# Verify changes appear immediately
# Check console for errors
```

### Build Verification
```bash
make gui-clean
make gui-build
./bin/ffd-gui.exe
```

### Bindings Regeneration
```bash
# After changing app.go methods
make gui-bindings
make gui-build-frontend
```

## CI/CD Integration

Add to GitHub Actions workflow:

```yaml
- name: Setup Wails
  run: make gui-install-wails

- name: Build GUI
  run: make gui-build

- name: Upload GUI Binary
  uses: actions/upload-artifact@v3
  with:
    name: ffd-gui-windows
    path: bin/ffd-gui.exe
```

## Success Criteria

The GUI is production-ready when:

- ✅ All checklist items pass
- ✅ No TypeScript compilation errors
- ✅ No console warnings or errors during normal use
- ✅ Performance matches or exceeds CLI
- ✅ Edge cases handled gracefully
- ✅ Accessibility requirements met
- ✅ Windows 11 aesthetic achieved
- ✅ No memory leaks over 1-hour operation

## Next Steps

After testing is complete:

1. Document any issues found
2. Fix critical bugs
3. Add automated E2E tests (Playwright/Wails test framework)
4. Create user documentation
5. Prepare release (v0.17.0 with GUI)
6. Update README with GUI screenshots
