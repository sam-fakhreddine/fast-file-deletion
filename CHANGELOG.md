# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Initial implementation of fast file deletion tool
- Windows-optimized deletion using Win32 API
- Parallel deletion with goroutines
- Age-based file filtering (--keep-days)
- Dry-run mode for safe testing
- Progress reporting with ETA
- Safety validation for protected paths
- Comprehensive error handling and logging
- Cross-platform support (Windows, Linux, macOS)

### Features
- Delete directories with millions of files 5-10x faster than Windows Explorer
- True parallelism with Go goroutines
- Single binary distribution with zero dependencies
- Command-line interface with multiple options
- Real-time progress reporting
- Graceful interruption handling (Ctrl+C)

## [1.0.0] - TBD

### Added
- First stable release
- Complete implementation of all core features
- Comprehensive test suite with property-based testing
- Documentation and usage examples
- Build scripts and GitHub Actions workflows

[Unreleased]: https://github.com/yourusername/fast-file-deletion/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/yourusername/fast-file-deletion/releases/tag/v1.0.0
