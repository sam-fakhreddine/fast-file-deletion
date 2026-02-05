# GUI Build Instructions

## Important: Platform-Specific Builds

GUI applications with native WebView **cannot be cross-compiled**. You must build on the target platform:

- **Windows builds**: Must be built on Windows
- **macOS builds**: Must be built on macOS
- **Linux builds**: Must be built on Linux

This is because Wails embeds platform-specific WebView components (WebView2 on Windows, WebKit on macOS/Linux).

---

## Build on Windows

### Prerequisites

1. **Go 1.25.5+**: https://go.dev/dl/
2. **Node.js 18+**: https://nodejs.org/
3. **Git**: https://git-scm.com/downloads
4. **WebView2**: Pre-installed on Windows 11, download for Windows 10 from https://developer.microsoft.com/microsoft-edge/webview2/

### Build Steps

```powershell
# Clone repository
git clone https://github.com/yourusername/fast-file-deletion.git
cd fast-file-deletion

# First-time setup (installs Wails, npm deps, generates bindings)
make gui-setup

# Build production binary
make gui-build

# Output: bin/ffd-gui.exe (~15-20 MB)
```

### Development Mode

```powershell
# Start development server with hot reload
make gui-dev

# Frontend runs on http://localhost:34115 (auto-opens window)
# Changes to frontend/src/* reload automatically
# Changes to cmd/ffd-gui/*.go require restart
```

---

## Build on macOS

### Prerequisites

1. **Xcode Command Line Tools**: `xcode-select --install`
2. **Go 1.25.5+**: `brew install go`
3. **Node.js 18+**: `brew install node`

### Build Steps

```bash
# Clone repository
git clone https://github.com/yourusername/fast-file-deletion.git
cd fast-file-deletion

# First-time setup
make gui-setup

# Build for macOS
make gui-build

# Output: bin/ffd-gui (~15-20 MB)
```

### Run on macOS

```bash
# Make executable if needed
chmod +x bin/ffd-gui

# Run
./bin/ffd-gui
```

**Note**: macOS builds use the generic deletion backend (no advanced Windows deletion methods).

---

## Build on Linux

### Prerequisites

```bash
# Debian/Ubuntu
sudo apt-get install golang nodejs npm webkit2gtk-4.0-dev

# Fedora/RHEL
sudo dnf install golang nodejs npm webkit2gtk4.0-devel

# Arch
sudo pacman -S go nodejs npm webkit2gtk
```

### Build Steps

```bash
# Clone repository
git clone https://github.com/yourusername/fast-file-deletion.git
cd fast-file-deletion

# First-time setup
make gui-setup

# Build for Linux
make gui-build

# Output: bin/ffd-gui (~15-20 MB)
```

**Note**: Linux builds use the generic deletion backend.

---

## CI/CD Build Matrix

### GitHub Actions Example

```yaml
name: Build GUI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  build-windows:
    runs-on: windows-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.25.5'
      - uses: actions/setup-node@v3
        with:
          node-version: '18'

      - name: Setup and Build
        run: |
          make gui-setup
          make gui-build

      - name: Upload Windows Binary
        uses: actions/upload-artifact@v3
        with:
          name: ffd-gui-windows
          path: bin/ffd-gui.exe

  build-macos:
    runs-on: macos-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.25.5'
      - uses: actions/setup-node@v3
        with:
          node-version: '18'

      - name: Setup and Build
        run: |
          make gui-setup
          make gui-build

      - name: Upload macOS Binary
        uses: actions/upload-artifact@v3
        with:
          name: ffd-gui-macos
          path: bin/ffd-gui

  build-linux:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.25.5'
      - uses: actions/setup-node@v3
        with:
          node-version: '18'

      - name: Install Dependencies
        run: sudo apt-get install webkit2gtk-4.0-dev

      - name: Setup and Build
        run: |
          make gui-setup
          make gui-build

      - name: Upload Linux Binary
        uses: actions/upload-artifact@v3
        with:
          name: ffd-gui-linux
          path: bin/ffd-gui
```

---

## Manual Build Without Make

If you don't have `make` installed:

### Windows (PowerShell)

```powershell
# Install Wails
go install github.com/wailsapp/wails/v3/cmd/wails3@latest

# Install frontend deps
cd frontend
npm install
cd ..

# Generate bindings
cd cmd/ffd-gui
wails3 generate bindings
Copy-Item -Recurse frontend/bindings ../../frontend/
cd ../..

# Build frontend
cd frontend
npm run build
cd ..

# Build GUI
cd cmd/ffd-gui
wails3 build
Copy-Item build/bin/ffd-gui.exe ../../bin/
cd ../..
```

### macOS/Linux (Bash)

```bash
# Install Wails
go install github.com/wailsapp/wails/v3/cmd/wails3@latest

# Install frontend deps
cd frontend && npm install && cd ..

# Generate bindings
cd cmd/ffd-gui
wails3 generate bindings
cp -r frontend/bindings ../../frontend/
cd ../..

# Build frontend
cd frontend && npm run build && cd ..

# Build GUI
cd cmd/ffd-gui
wails3 build
cp build/bin/ffd-gui ../../bin/
cd ../..
```

---

## Troubleshooting

### "wails3: command not found"

Add Go bin to PATH:

```bash
# macOS/Linux
export PATH="$HOME/go/bin:$PATH"

# Windows (PowerShell)
$env:Path += ";$env:USERPROFILE\go\bin"
```

Or use full path: `~/go/bin/wails3` (macOS/Linux) or `%USERPROFILE%\go\bin\wails3.exe` (Windows)

### "WebView2 not found" (Windows 10)

Download and install WebView2 Runtime: https://developer.microsoft.com/microsoft-edge/webview2/

Windows 11 has this pre-installed.

### "webkit2gtk not found" (Linux)

Install the WebKitGTK development package:

```bash
# Debian/Ubuntu
sudo apt-get install webkit2gtk-4.0-dev

# Fedora
sudo dnf install webkit2gtk4.0-devel
```

### "npm install fails"

Clear cache and retry:

```bash
cd frontend
rm -rf node_modules package-lock.json
npm cache clean --force
npm install
```

### "Bindings not found" error during build

Regenerate bindings:

```bash
make gui-bindings
# or
cd cmd/ffd-gui
wails3 generate bindings
cp -r frontend/bindings ../../frontend/
```

### Build is slow

First build takes longer due to:
- Go module downloads
- npm package installation
- Wails setup

Subsequent builds are much faster (~5-10 seconds).

---

## Binary Sizes

| Platform | Size | Compressed (zip) |
|----------|------|------------------|
| Windows  | ~15-20 MB | ~6-8 MB |
| macOS    | ~15-20 MB | ~6-8 MB |
| Linux    | ~15-20 MB | ~6-8 MB |

Size includes:
- Go runtime
- WebView bridge
- Frontend bundle (970 KB JS + 2 KB CSS)
- Application icon
- Platform-specific libraries

---

## Development Tips

### Hot Reload

```bash
make gui-dev
```

Changes to `frontend/src/*` files reload automatically in the window. Backend changes require restarting the dev server.

### Frontend Only

```bash
# Build frontend without launching app
make gui-build-frontend

# Output: frontend/dist/
```

Useful for testing frontend changes in isolation.

### Clean Build

```bash
# Remove all build artifacts
make gui-clean

# Full rebuild
make gui-build
```

### Debugging

**Backend (Go)**:
```bash
# Run with verbose output
cd cmd/ffd-gui
go run . -verbose
```

**Frontend (JavaScript)**:
Open DevTools in the app window:
- **Windows/Linux**: F12 or Ctrl+Shift+I
- **macOS**: Cmd+Option+I

---

## Release Checklist

Before creating a release:

1. Update version in `wails.json`
2. Run tests: `make test-thorough`
3. Build on all platforms (Windows, macOS, Linux)
4. Test each binary (see `GUI_TESTING.md`)
5. Create release notes
6. Tag release: `git tag v0.17.0`
7. Upload binaries to GitHub Releases

---

## Questions?

- **Wails Documentation**: https://v3.wails.io/
- **Project Issues**: https://github.com/yourusername/fast-file-deletion/issues
- **Wails Discord**: https://discord.gg/wails
