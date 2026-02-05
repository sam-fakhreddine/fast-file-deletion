# Quick Start: Building the GUI

## Option 1: GitHub Actions (Recommended)

**Build on Windows, macOS, and Linux from any platform — for FREE!**

### Step 1: Push the workflow

```bash
git add .github/workflows/build-gui.yml
git commit -m "Add GUI build workflow"
git push
```

### Step 2: Trigger the build

1. Go to GitHub: `https://github.com/YOUR_USERNAME/fast-file-deletion`
2. Click **Actions** tab
3. Click **Build GUI** in left sidebar
4. Click **Run workflow** button (green, top right)
5. Click **Run workflow** again to confirm

### Step 3: Wait ~10 minutes

GitHub will:
- ✅ Run tests on Ubuntu
- ✅ Build CLI for all platforms (6 binaries)
- ✅ Build GUI on Windows (ffd-gui.exe)
- ✅ Build GUI on macOS (ffd-gui)
- ✅ Build GUI on Linux (ffd-gui)

### Step 4: Download binaries

1. Click on the completed workflow run
2. Scroll to **Artifacts** section at bottom
3. Download:
   - `gui-windows` → Extract `ffd-gui.exe`
   - `gui-macos` → Extract `ffd-gui`
   - `gui-linux` → Extract `ffd-gui`
   - `cli-binaries` → 6 CLI binaries

**Done!** You now have binaries for all platforms, built on actual Windows/Mac/Linux machines.

---

## Option 2: Local Build (macOS)

If you're on macOS and just want to test locally:

```bash
# First time setup
make gui-setup

# Build for macOS
make gui-build

# Run it
./bin/ffd-gui
```

**Note**: This only builds for macOS. For Windows builds, use Option 1.

---

## Option 3: Local Build (Windows)

If you're on Windows:

```powershell
# First time setup
make gui-setup

# Build for Windows
make gui-build

# Run it
.\bin\ffd-gui.exe
```

---

## Creating a Release

### Step 1: Trigger with release option

1. Go to **Actions** → **Build GUI** → **Run workflow**
2. ✅ Check **"Create GitHub Release"**
3. Enter **Release tag**: `v0.17.0`
4. Click **Run workflow**

### Step 2: Wait for completion

GitHub will:
- Build all binaries
- Generate SHA256 checksums
- Create a GitHub Release
- Upload all binaries as release assets

### Step 3: Release is ready!

Go to **Releases** tab — your new release is published with:
- 9 binaries (6 CLI + 3 GUI)
- SHA256SUMS file
- Auto-generated release notes

---

## Comparison

| Method | Platforms | Time | Cost | Requirements |
|--------|-----------|------|------|--------------|
| **GitHub Actions** | Win + Mac + Linux | 10 min | Free | GitHub account |
| **Local (macOS)** | macOS only | 2 min | Free | macOS, Go, Node |
| **Local (Windows)** | Windows only | 2 min | Free | Windows, Go, Node |

**Recommendation**: Use GitHub Actions to get all 3 platform binaries in one go.

---

## Troubleshooting

### "Actions tab not showing"

Make sure you've pushed the workflow file:

```bash
git add .github/
git commit -m "Add workflows"
git push
```

### "Build GUI workflow not listed"

Wait a few seconds and refresh the page. If still not showing, check the YAML syntax:

```bash
cat .github/workflows/build-gui.yml
```

Look for syntax errors (indentation, colons, dashes).

### "Build failed"

Click on the failed job to see logs. Common issues:

- **Frontend build failed**: `npm ci` error → Delete `frontend/package-lock.json` locally, run `npm install`, commit, and retry
- **Wails not found**: Workflow error → Check if `go install` step succeeded
- **Tests failed**: Code issue → Fix tests locally first with `make test`

---

## Cost Breakdown

GitHub Actions free tier:

- **Public repos**: Unlimited minutes
- **Private repos**: 2,000 minutes/month

This workflow uses ~15 minutes per run:

- **Public repo**: Run as many times as you want ✅
- **Private repo**: 133 runs/month (4-5 per day) ✅

---

## Next Steps

1. **Test the GUI**: See `GUI_TESTING.md`
2. **Build documentation**: See `GUI_BUILD.md`
3. **Customize workflow**: See `.github/README.md`

---

**Questions?** Open an issue: https://github.com/YOUR_USERNAME/fast-file-deletion/issues
