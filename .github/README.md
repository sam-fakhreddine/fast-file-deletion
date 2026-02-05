# GitHub Actions Workflows

## Build GUI Workflow

**File**: `workflows/build-gui.yml`

### Purpose

Builds the Fast File Deletion GUI for Windows, macOS, and Linux using GitHub's hosted runners. This solves the cross-compilation problem — you can trigger builds from any platform and get binaries for all platforms.

### How to Use

#### 1. Navigate to Actions

1. Go to your repository on GitHub
2. Click the **Actions** tab
3. Select **Build GUI** from the workflow list on the left

#### 2. Run Workflow

Click **Run workflow** button (top right), then:

**For development builds** (just to test):
- Leave both checkboxes unchecked
- Click **Run workflow**
- Wait ~5-10 minutes
- Download artifacts from the workflow run page

**For release builds** (create GitHub Release):
- ✅ Check **Create GitHub Release**
- Enter **Release tag**: `v0.17.0` (or your version)
- Click **Run workflow**
- Wait ~5-10 minutes
- A new GitHub Release will be created with all binaries

#### 3. Download Binaries

**Development builds (artifacts)**:
- Go to the completed workflow run
- Scroll to **Artifacts** section at the bottom
- Download:
  - `cli-binaries` - All CLI binaries (6 files)
  - `gui-windows` - Windows GUI (`ffd-gui.exe`)
  - `gui-macos` - macOS GUI (`ffd-gui`)
  - `gui-linux` - Linux GUI (`ffd-gui`)

**Release builds**:
- Go to **Releases** tab (right sidebar on main repo page)
- Your new release will be at the top
- Download binaries from **Assets** section

### What Gets Built

| Job | Platform | Output | Size |
|-----|----------|--------|------|
| **test** | Ubuntu | - | - |
| **build-cli** | Ubuntu (cross-compile) | 6 CLI binaries (Win/Mac/Linux, AMD64/ARM64) | ~4 MB each |
| **build-gui-windows** | Windows | `ffd-gui.exe` | ~15-20 MB |
| **build-gui-macos** | macOS | `ffd-gui` | ~15-20 MB |
| **build-gui-linux** | Ubuntu | `ffd-gui` | ~15-20 MB |

### Build Time

- **Tests**: ~1 minute
- **CLI**: ~2 minutes
- **GUI (each platform)**: ~5-7 minutes
- **Total**: ~10-15 minutes

### Cost

**Free!** GitHub provides:
- 2,000 Actions minutes/month for free (private repos)
- Unlimited minutes for public repos
- Windows, macOS, and Linux runners

This workflow uses ~15 minutes per run, so you can run it **133 times/month** on private repos, or **unlimited** on public repos.

### Workflow Steps

```
┌─────────────┐
│   Trigger   │ (Manual: workflow_dispatch)
└─────┬───────┘
      │
      ├─────────────────────────────────┐
      │                                 │
      v                                 v
┌─────────────┐                  ┌──────────────┐
│    Tests    │                  │  Build CLI   │
│  (Ubuntu)   │                  │  (Ubuntu)    │
│             │                  │              │
│ • Quick     │                  │ • Windows    │
│ • Thorough  │                  │ • macOS      │
│ • verify-all│                  │ • Linux      │
└─────────────┘                  └──────────────┘
      │                                 │
      v                                 v
┌──────────────────────────────────────────────┐
│          Build GUI (Parallel)                │
├──────────────┬──────────────┬────────────────┤
│   Windows    │    macOS     │     Linux      │
│              │              │                │
│ • WebView2   │ • WebKit     │ • webkit2gtk   │
│ • Wails v3   │ • Wails v3   │ • Wails v3     │
│ • npm build  │ • npm build  │ • npm build    │
│              │              │                │
│ → .exe       │ → binary     │ → binary       │
└──────────────┴──────────────┴────────────────┘
      │              │                │
      v              v                v
┌──────────────────────────────────────────────┐
│            Upload Artifacts                  │
│                                              │
│ • CLI binaries (6 files)                     │
│ • GUI Windows (.exe)                         │
│ • GUI macOS (binary)                         │
│ • GUI Linux (binary)                         │
└──────────────────────────────────────────────┘
      │
      v
┌──────────────────────────────────────────────┐
│   Create Release (if checkbox enabled)       │
│                                              │
│ • Download all artifacts                     │
│ • Rename GUI binaries for clarity            │
│ • Generate SHA256 checksums                  │
│ • Create GitHub Release with notes           │
│ • Upload all binaries as assets              │
└──────────────────────────────────────────────┘
```

### Troubleshooting

#### "workflow_dispatch event not found"

Make sure you've pushed the workflow file to GitHub first:

```bash
git add .github/workflows/build-gui.yml
git commit -m "Add GUI build workflow"
git push
```

Then refresh the Actions page.

#### "No workflows found"

The workflow file might have syntax errors. Check:

```bash
# Validate YAML syntax
yamllint .github/workflows/build-gui.yml

# Or use a web tool
cat .github/workflows/build-gui.yml | pbcopy
# Paste into: https://www.yamllint.com/
```

#### Build fails on Windows

Most common issues:
- **Node modules**: Frontend dependencies failed to install
  - Check `npm ci` logs
  - May need to clear cache (handled automatically)
- **Wails install**: `wails3` command not found
  - Check if `go install` step succeeded
  - Windows runner should have Go bin in PATH

#### Build fails on Linux

Usually due to missing system dependencies:
- The workflow installs `webkit2gtk-4.0-dev` and `libgtk-3-dev`
- If it fails, check the apt-get logs
- May need to add more packages

#### Artifacts not appearing

Artifacts only appear **after** the job completes successfully. If a job fails, no artifact is uploaded. Check the job logs to see what failed.

### Modifying the Workflow

#### Change Go version

Edit line 13:

```yaml
env:
  GO_VERSION: '1.26.0'  # Update this
```

#### Change Node version

Edit line 14:

```yaml
env:
  NODE_VERSION: '20'  # Update this
```

#### Add more platforms

To add ARM builds or other platforms:

1. Add new job (copy existing one)
2. Update runner: `runs-on: ubuntu-latest` (for ARM, use self-hosted runner)
3. Set environment variables:
   ```yaml
   env:
     GOOS: linux
     GOARCH: arm64
   ```

#### Skip tests

Comment out the `needs: test` line in build jobs:

```yaml
build-gui-windows:
  name: Build GUI (Windows)
  runs-on: windows-latest
  # needs: test  # <-- Comment this out
```

#### Change retention

Artifacts are kept for 30 days by default. Change it:

```yaml
- uses: actions/upload-artifact@v4
  with:
    retention-days: 90  # Keep for 90 days
```

### Security Notes

- ✅ Workflow uses `@v4` and `@v5` action versions (pinned)
- ✅ No third-party actions (except official GitHub and verified publishers)
- ✅ `GITHUB_TOKEN` has minimal permissions (only `contents: write` for releases)
- ✅ No secrets required (all dependencies are public)
- ✅ Artifacts are private (only repo collaborators can download)

### Alternative: Scheduled Builds

To run builds automatically (e.g., nightly):

```yaml
on:
  schedule:
    - cron: '0 2 * * *'  # 2 AM UTC daily
  workflow_dispatch:
```

### Alternative: Build on Tag

To trigger automatically when you create a Git tag:

```yaml
on:
  push:
    tags:
      - 'v*'  # Triggers on v0.17.0, v1.0.0, etc.
  workflow_dispatch:
```

Then:

```bash
git tag v0.17.0
git push origin v0.17.0
# → Triggers build automatically
```

---

## Questions?

- GitHub Actions docs: https://docs.github.com/en/actions
- Wails docs: https://v3.wails.io/
- Open an issue: https://github.com/yourusername/fast-file-deletion/issues
