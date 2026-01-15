# Project Structure

## Directory Layout

```
fast-file-deletion/
├── cmd/fast-file-deletion/     # CLI entry point
│   ├── main.go                 # Argument parsing, workflow orchestration
│   └── main_test.go            # CLI integration tests
├── internal/                   # Internal packages (not importable externally)
│   ├── backend/                # Platform-specific deletion implementations
│   ├── engine/                 # Parallel deletion engine with goroutine workers
│   ├── logger/                 # Structured logging
│   ├── progress/               # Real-time progress reporting
│   ├── safety/                 # Path validation and user confirmation
│   └── scanner/                # Directory traversal and age filtering
├── bin/                        # Build output (gitignored)
├── .kiro/                      # Kiro AI assistant configuration
│   ├── specs/                  # Feature specifications
│   └── steering/               # AI guidance documents
└── Makefile                    # Build automation
```

## Package Architecture

### Core Workflow (main.go)

1. Parse arguments → 2. Safety validation → 3. Directory scan → 4. User confirmation → 5. Parallel deletion → 6. Progress reporting

### Package Responsibilities

**backend/** - Platform-specific deletion
- `Backend` interface: DeleteFile(), DeleteDirectory()
- `WindowsBackend`: Win32 API calls (Windows only)
- `GenericBackend`: Standard Go operations (all platforms)
- Factory pattern with build tags for platform selection

**engine/** - Parallel deletion coordination
- Worker pool pattern with goroutines
- Depth-based batching (delete children before parents)
- Context-based cancellation (Ctrl+C support)
- Thread-safe statistics collection

**scanner/** - Directory traversal
- Efficient filepath.WalkDir usage
- Age-based filtering (modification time)
- Bottom-up ordering (files before directories)
- Size calculation for progress reporting

**safety/** - Protection mechanisms
- Protected path validation (system directories)
- User confirmation prompts
- Exact path verification
- Dry-run support

**progress/** - Real-time feedback
- Deletion rate calculation (files/sec)
- Progress percentage and ETA
- Final statistics summary

**logger/** - Structured logging
- Configurable verbosity
- Optional file output
- Structured error reporting

## Code Conventions

### File Naming

- `*_windows.go`: Windows-specific implementations (build tag: `//go:build windows`)
- `*_generic.go`: Generic implementations (build tag: `//go:build !windows`)
- `*_test.go`: Unit tests
- `testdata/rapid/`: Property-based test data

### Error Handling

- Errors are logged but don't stop deletion (resilience)
- FileError struct tracks per-file failures
- Exit codes: 0 (success), 1 (partial failure), 2 (complete failure)

### Concurrency Patterns

- Worker pool with buffered channels
- Mutex-protected shared state
- Context-based cancellation
- WaitGroups for synchronization

## Testing Organization

- Unit tests co-located with source files
- Property-based tests use Rapid framework
- Integration tests in `internal/integration_test.go`
- Test data preserved in `testdata/rapid/` for reproducibility
