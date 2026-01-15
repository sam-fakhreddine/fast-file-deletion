# Release Process

This document describes how to create releases for Fast File Deletion.

## Prerequisites

- Push access to the GitHub repository
- All tests passing on main branch
- Updated CHANGELOG.md with release notes

## Creating a Release

### 1. Update Version Information

Update the CHANGELOG.md file with the new version and release date:

```markdown
## [1.0.0] - 2026-01-14

### Added
- Feature descriptions here
```

### 2. Commit Changes

```bash
git add CHANGELOG.md
git commit -m "chore: prepare release v1.0.0"
git push origin main
```

### 3. Create and Push Tag

```bash
# Create an annotated tag
git tag -a v1.0.0 -m "Release v1.0.0"

# Push the tag to GitHub
git push origin v1.0.0
```

### 4. Automated Release

Once the tag is pushed, GitHub Actions will automatically:

1. Run all tests to ensure quality
2. Build binaries for all platforms:
   - Windows (amd64, arm64)
   - Linux (amd64, arm64)
   - macOS (amd64, arm64)
3. Create archives (zip for Windows, tar.gz for others)
4. Generate SHA256 checksums
5. Create a GitHub release with:
   - Release notes from CHANGELOG.md
   - All binary archives
   - Checksum file
   - Installation instructions

### 5. Verify Release

After the GitHub Action completes:

1. Go to https://github.com/yourusername/fast-file-deletion/releases
2. Verify the release was created
3. Check that all binary archives are present
4. Test download and installation on at least one platform

## Manual Build (Local Testing)

To test the release process locally without creating a release:

### Using Make

```bash
# Build for all platforms
make build-all

# Build for specific platform
make build-windows
make build-linux
make build-darwin
```

### Using Build Script

```bash
# Build for all platforms
./build.sh all

# Build for specific platform
./build.sh windows
./build.sh linux
./build.sh darwin
```

### Using GoReleaser (Snapshot)

If you have GoReleaser installed locally:

```bash
# Install GoReleaser
go install github.com/goreleaser/goreleaser@latest

# Create a snapshot build (no release)
goreleaser release --snapshot --clean

# Output will be in ./dist/
```

## Version Numbering

This project follows [Semantic Versioning](https://semver.org/):

- **MAJOR** version (X.0.0): Incompatible API changes
- **MINOR** version (0.X.0): New functionality, backwards compatible
- **PATCH** version (0.0.X): Bug fixes, backwards compatible

Examples:
- `v1.0.0` - First stable release
- `v1.1.0` - Added new feature (--keep-days)
- `v1.1.1` - Fixed bug in age filtering
- `v2.0.0` - Changed CLI argument structure (breaking change)

## Pre-releases

For alpha, beta, or release candidate versions:

```bash
git tag -a v1.0.0-alpha.1 -m "Release v1.0.0-alpha.1"
git push origin v1.0.0-alpha.1
```

GoReleaser will automatically mark these as pre-releases on GitHub.

## Troubleshooting

### Release Failed

If the GitHub Action fails:

1. Check the Actions tab for error logs
2. Fix the issue in code
3. Delete the tag locally and remotely:
   ```bash
   git tag -d v1.0.0
   git push origin :refs/tags/v1.0.0
   ```
4. Recreate and push the tag after fixing

### Missing Binaries

If some platform binaries are missing:

1. Check the GoReleaser configuration in `.goreleaser.yml`
2. Verify the `goos` and `goarch` combinations
3. Check for build errors in the GitHub Actions log

### Checksum Mismatch

If users report checksum mismatches:

1. Verify the checksums.txt file in the release
2. Regenerate checksums if needed:
   ```bash
   cd dist
   shasum -a 256 *.tar.gz *.zip > checksums.txt
   ```

## Distribution Channels

### GitHub Releases (Primary)

All releases are published to GitHub Releases:
https://github.com/yourusername/fast-file-deletion/releases

### Go Install

Users can install directly using Go:

```bash
go install github.com/yourusername/fast-file-deletion/cmd/fast-file-deletion@latest
```

### Future Channels (Planned)

- Homebrew tap for macOS
- Chocolatey package for Windows
- Snap package for Linux
- Docker image

## Release Checklist

Before creating a release, ensure:

- [ ] All tests pass (`go test ./...`)
- [ ] Code is properly formatted (`go fmt ./...`)
- [ ] CHANGELOG.md is updated
- [ ] Version number follows semantic versioning
- [ ] Documentation is up to date
- [ ] No known critical bugs
- [ ] Performance benchmarks are acceptable
- [ ] Security vulnerabilities are addressed

## Post-Release

After a successful release:

1. Announce on relevant channels (if applicable)
2. Update documentation website (if applicable)
3. Monitor for bug reports and user feedback
4. Plan next release based on feedback
