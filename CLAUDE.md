# Fast File Deletion (FFD)

High-performance CLI tool for deleting directories containing millions of small files, optimized for Windows. Written in Go.

## Quick Reference

```
Module:       github.com/yourusername/fast-file-deletion
Go version:   1.25.5
Entry point:  cmd/fast-file-deletion/main.go
Dependencies: golang.org/x/sys (Windows APIs), pgregory.net/rapid (property testing)
```

## Build & Test Commands

```bash
make test                # Quick tests (~30s) — use during development
make test-thorough       # Comprehensive tests — use before merging
make test-race           # Race condition detection
make test-coverage       # Generate coverage.html
make test-stress         # Stress tests (10+ min, build tag: stress)
make test-all            # Thorough + stress combined
make verify-all          # Verify compilation on all platforms
make build-all           # Cross-compile: Windows, Linux, macOS (AMD64 + ARM64)
```

### Running specific tests

```bash
go test ./internal/engine                          # One package
go test ./internal/engine -run TestEngineDeletion  # One test
TEST_INTENSITY=thorough go test ./internal/backend # Thorough mode for one package
```

### Test verbosity warning

**NEVER use `-v` flag with `go test`** — property-based tests produce massive output that will crash terminals. Use `VERBOSE_TESTS=1` env var for controlled verbosity on specific small packages only.

### Test environment variables

| Variable | Values | Purpose |
|----------|--------|---------|
| `TEST_INTENSITY` | `quick` (default), `thorough` | Controls iteration counts and data sizes |
| `TEST_QUICK` | `1` | Force quick mode regardless of TEST_INTENSITY |
| `VERBOSE_TESTS` | `1` | Enable verbose output (use sparingly) |

## Architecture

```
cmd/fast-file-deletion/    CLI: arg parsing, config validation, workflow orchestration
internal/
  backend/                 Platform-specific deletion (interfaces + implementations)
  engine/                  Parallel worker pool, batch processing, adaptive tuning
  scanner/                 Directory traversal, age filtering, UTF-16 pre-conversion
  progress/                Real-time stats, ETA, deletion rate
  safety/                  Protected path validation, user confirmation
  logger/                  Structured logging with optional file output
  monitor/                 System resource monitoring, bottleneck detection
  testutil/                Test config, fixtures, cleanup, rapid helpers, timeouts
```

### Core workflow

CLI parses args → Safety validates path → Scanner traverses directory → User confirms → Engine deletes in parallel → Progress reports stats

### Platform abstraction

- `*_windows.go` — Windows-specific code (build tag: `//go:build windows`)
- `*_generic.go` — Cross-platform fallback (build tag: `//go:build !windows`)
- Factory pattern: `backend.NewBackend()` returns platform-appropriate implementation at compile time

### Key interfaces

- `Backend` — `DeleteFile(path)`, `DeleteDirectory(path)`
- `UTF16Backend` — Extends Backend with `DeleteFileUTF16(*uint16, string)` for pre-converted paths
- `AdvancedBackend` — Extends Backend with `SetDeletionMethod()`, `GetDeletionStats()`
- `Scanner` / `ParallelScanner` — Directory traversal returning `ScanResult`

### Windows deletion method fallback chain

`FileDispositionInfoEx` → `FileDispositionInfo` → `FILE_FLAG_DELETE_ON_CLOSE` → `NtDeleteFile` → `DeleteFile`

### Memory management (v0.16+)

- **Dynamic allocation**: 25% of system RAM (up to 6GB max, 512MB min)
- **Platform detection**: Windows (GlobalMemoryStatusEx), macOS (sysctl hw.memsize), Linux (/proc/meminfo)
- **Environment override**: Respects `GOMEMLIMIT` if set by user
- **Initialization**: `initializeMemoryLimit()` called at startup in main.go
- **Threshold**: Memory pressure warnings at 95% (was 80%)
- **Cooldown**: 60-second cooldown between bottleneck warnings

## Code Conventions

- **File naming**: `*_windows.go` / `*_generic.go` for platform code, `*_test.go` for tests, `*_stress_test.go` for stress tests (behind `stress` build tag)
- **Concurrency**: Use `sync/atomic` for counters, not mutexes. Mutexes only for slices/maps.
- **Worker default**: `runtime.NumCPU() * 4` for I/O-bound deletion
- **Buffer sizing**: `min(fileCount, 10000)` for work channels
- **Error handling**: Log and continue — individual file failures don't stop the operation
- **Testing**: Dual approach — unit tests for specific cases + property-based tests (Rapid) for invariants. Min 100 iterations in thorough mode.
- **Property tests**: Use `testutil.GetRapidOptions()` to get iteration counts matching current TEST_INTENSITY. Never hardcode iteration counts.
- **Test data**: Use `testutil.CreateTestDirectory()` for fixture generation. Respect `testutil.GetTestConfig()` limits for file counts, sizes, and depth.

## Specs & Design Docs

Detailed requirements, design decisions, correctness properties, and task breakdowns live in `.kiro/specs/`:

- `.kiro/specs/fast-file-deletion/` — Core product (9 requirements, 18 correctness properties)
- `.kiro/specs/windows-performance-optimization/` — Windows APIs, parallelism, memory (12 requirements, 26 properties)
- `.kiro/specs/test-suite-optimization/` — Test performance, configurable intensity (9 requirements, 6 properties)

Steering docs in `.kiro/steering/` describe product context, tech stack, and project structure.
