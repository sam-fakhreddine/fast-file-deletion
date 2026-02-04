# Bash Command Guidelines

## Shell Environment Setup

**CRITICAL**: All shell commands must start with `source ~/.zshrc` to ensure proper environment setup.

```bash
# ✅ Correct - Source zshrc for shell commands
source ~/.zshrc && git status
source ~/.zshrc && go test ./...
source ~/.zshrc && ls -la

# ❌ Wrong - Missing environment setup
git status
go test ./...
```

## Go Testing - Verbosity Control

**CRITICAL**: Go tests in this project are extremely verbose and WILL crash Kiro if run with `-v` flag or without output limits.

### ❌ Forbidden Test Patterns

```bash
# ALL of these will crash Kiro - NEVER use:
go test ./... -v                    # Too verbose
go test -v ./internal/...           # Too verbose
go test ./internal/engine -v        # Too verbose
make test                           # May be too verbose depending on Makefile
```

### ✅ Safe Test Patterns

```bash
# Run tests with minimal output (summary only)
source ~/.zshrc && go test ./...

# Run specific package without verbose flag
source ~/.zshrc && go test ./internal/engine

# Run specific test function (still no -v)
source ~/.zshrc && go test ./internal/engine -run TestSpecificFunction

# Run with race detection (no -v)
source ~/.zshrc && go test -race ./...

# Run with coverage (no -v)
source ~/.zshrc && go test -cover ./...

# If you MUST see details, test ONE small file at a time
source ~/.zshrc && go test ./internal/logger -v
```

### Output Management Strategy

1. **Default**: Run tests WITHOUT `-v` flag - only see pass/fail summary
2. **Debugging**: If you need details, test ONE small package at a time with `-v`
3. **Never**: Run verbose tests on the entire project or large packages
4. **Property-based tests**: These generate massive output - NEVER use `-v` with rapid tests

### When Tests Fail

```bash
# ✅ See which tests failed (minimal output)
source ~/.zshrc && go test ./...

# ✅ Run only the failing package without -v
source ~/.zshrc && go test ./internal/engine

# ✅ Run specific failing test without -v
source ~/.zshrc && go test ./internal/engine -run TestCompleteDirectoryRemoval

# ❌ Don't try to see all verbose output
source ~/.zshrc && go test ./... -v  # WILL CRASH KIRO
```

## When to Use Bash

- Git operations: `source ~/.zshrc && git status`
- File management: `source ~/.zshrc && ls -la`
- Go testing: `source ~/.zshrc && go test ./...` (NO -v flag)
- Go building: `source ~/.zshrc && go build ./cmd/fast-file-deletion`
- Make commands: `source ~/.zshrc && make build-windows`

## Error Handling

When a bash command fails, document the error and solution to avoid repeating mistakes.

### Common Go Test Issues

**Issue**: Tests are too verbose and crash Kiro
- **Cause**: Using `-v` flag with large test suites or property-based tests
- **Solution**: Remove `-v` flag, run tests with summary output only
- **Prevention**: Never use `-v` for project-wide or package-wide test runs

**Issue**: Need to debug a specific test failure
- **Cause**: Test fails but output is minimal
- **Solution**: Run ONLY that specific test function without `-v`, or test the single small package with `-v`
- **Prevention**: Isolate to smallest possible test scope before adding verbosity
