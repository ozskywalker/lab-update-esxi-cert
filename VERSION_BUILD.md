# Version and Build Information

This document explains how versioning and build-time information injection works for the ESXi Certificate Manager.

## Versioning Strategy

The application follows [Semantic Versioning](https://semver.org/) (SemVer) format:
- **MAJOR.MINOR.PATCH** (e.g., 1.2.3)
- **MAJOR**: Incompatible API changes
- **MINOR**: Backward compatible functionality additions
- **PATCH**: Backward compatible bug fixes

## Build-Time Version Injection

Version information is injected at build time using Go's `-ldflags` parameter. This approach allows:
- Clean source code without hardcoded version strings
- Dynamic version information based on Git state
- Comprehensive build metadata for debugging and monitoring

### Version Variables

The following variables in `internal/version/version.go` are set at build time:

```go
var (
    Version   = "development"      // Set via -X flag
    GitCommit = ""                 // Set via -X flag  
    BuildDate = "1970-01-01T00:00:00Z" // Set via -X flag
    GitTag    = ""                 // Set via -X flag
)
```

## Building with Version Information

### Using Build Scripts

**Linux/macOS:**
```bash
./build.sh
```

**Windows:**
```powershell
.\build.ps1
```

Both scripts automatically extract version information from Git and inject it during build.

### Manual Build

You can also build manually with custom version information:

```bash
go build -ldflags "
  -X 'lab-update-esxi-cert/internal/version.Version=v1.0.0'
  -X 'lab-update-esxi-cert/internal/version.GitCommit=$(git rev-parse HEAD)'
  -X 'lab-update-esxi-cert/internal/version.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)'
  -X 'lab-update-esxi-cert/internal/version.GitTag=$(git describe --tags --exact-match 2>/dev/null)'
" -o lab-update-esxi-cert
```

### Environment Variables for Build Scripts

You can customize build behavior:

**Linux/macOS (Environment Variables):**
- `VERSION`: Override version string (default: git describe output)
- `COMMIT`: Override git commit hash (default: git rev-parse HEAD)
- `BUILD_DATE`: Override build timestamp (default: current UTC time)
- `GIT_TAG`: Override git tag (default: exact tag match)
- `OUTPUT`: Override output binary name (default: lab-update-esxi-cert)

**Windows (PowerShell Parameters):**
- `-Version`: Override version string (default: git describe output)
- `-Commit`: Override git commit hash (default: git rev-parse HEAD)
- `-BuildDate`: Override build timestamp (default: current UTC time)
- `-GitTag`: Override git tag (default: exact tag match)
- `-Output`: Override output binary name (default: lab-update-esxi-cert.exe)

**Linux/macOS Example:**
```bash
VERSION=v2.0.0-beta OUTPUT=esxi-cert-beta ./build.sh
```

**Windows Example:**
```powershell
.\build.ps1 -Version "v2.0.0-beta" -Output "esxi-cert-beta.exe"
```

## Checking Version Information

### Command Line
```bash
# Show detailed version information
./lab-update-esxi-cert --version

# Version info is also shown at startup in logs
./lab-update-esxi-cert --hostname esxi01.lab.example.com --dry-run
```

### Sample Output
```
Version:    v1.0.0
Git Commit: a1b2c3d4e5f6789012345678901234567890abcd
Git Tag:    v1.0.0
Build Date: 2024-01-15T10:30:45Z
Go Version: go1.21.5
Compiler:   gc
Platform:   linux/amd64
```

## Update Checking

The application can check for newer versions on GitHub releases:

### Configuration Options

**Command Line:**
```bash
# Check for updates immediately
./lab-update-esxi-cert --check-updates --update-check-owner=username --update-check-repo=lab-update-esxi-cert

# Enable automatic update checks during normal operation
./lab-update-esxi-cert --check-updates=true --update-check-owner=username --update-check-repo=lab-update-esxi-cert [other options]
```

**Configuration File:**
```json
{
  "check_updates": true,
  "update_check_owner": "username",
  "update_check_repo": "lab-update-esxi-cert"
}
```

**Environment Variables:**
```bash
export CHECK_UPDATES=true
export UPDATE_CHECK_OWNER=username
export UPDATE_CHECK_REPO=lab-update-esxi-cert
```

### How Update Checking Works

1. Compares current version with latest GitHub release using semantic versioning
2. Uses GitHub API (no authentication required for public repos)
3. Includes built-in rate limiting and timeout protection
4. Shows non-intrusive notifications when updates are available
5. Never automatically downloads or installs updates

## Release Process

### Automated Releases via GitHub Actions

The project uses GitHub Actions with GoReleaser for fully automated releases:

1. **Tag the release** using semantic versioning:
   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   ```

2. **GitHub Actions automatically**:
   - Builds binaries for multiple platforms (Windows, Linux, macOS)
   - Injects proper version information using ldflags
   - Creates GitHub release with cross-platform assets
   - Generates checksums and signatures
   - Creates changelog from commit messages

3. **Release assets include**:
   - `lab-update-esxi-cert-Windows-x86_64.zip`
   - `lab-update-esxi-cert-Windows-arm64.zip`
   - `lab-update-esxi-cert-Linux-x86_64.tar.gz`
   - `lab-update-esxi-cert-Linux-arm64.tar.gz`
   - `lab-update-esxi-cert-Darwin-x86_64.tar.gz`
   - `lab-update-esxi-cert-Darwin-arm64.tar.gz`
   - `checksums.txt`

4. **Update checking** automatically detects the new release

### Manual Development Builds

For local development and testing:

**Linux/macOS:**
```bash
./build.sh
```

**Windows:**
```powershell
.\build.ps1
```

**GoReleaser (cross-platform):**
```bash
# Install GoReleaser
go install github.com/goreleaser/goreleaser@latest

# Build for current platform only
goreleaser build --single-target --snapshot --clean

# Build for all platforms (requires tag)
goreleaser release --snapshot --clean
```

## Development Builds

For development builds without Git tags:
- Version shows as "development" or git describe output
- Commit hash is still included for identification
- Update checking can be disabled or point to development repositories