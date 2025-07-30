# Release Guide

This guide explains how to create releases for the ESXi Certificate Manager.

## Quick Release Process

1. **Ensure all changes are committed and pushed to main**:
   ```bash
   git status
   git push origin main
   ```

2. **Create and push a semantic version tag**:
   ```bash
   # For a new feature release
   git tag v1.1.0
   
   # For a patch/bugfix release  
   git tag v1.0.1
   
   # For a major breaking change release
   git tag v2.0.0
   
   # Push the tag to trigger the release
   git push origin v1.1.0
   ```

3. **GitHub Actions automatically**:
   - Builds cross-platform binaries
   - Creates GitHub release with assets
   - Generates changelog
   - Signs and checksums all artifacts

4. **Verify the release**:
   - Check [GitHub Releases](https://github.com/yourusername/lab-update-esxi-cert/releases)
   - Download and test binaries
   - Verify version information: `./lab-update-esxi-cert --version`

## Semantic Versioning

This project follows [Semantic Versioning](https://semver.org/):

- **MAJOR** version (`v2.0.0`): Incompatible API changes
- **MINOR** version (`v1.1.0`): New functionality, backward compatible
- **PATCH** version (`v1.0.1`): Bug fixes, backward compatible

## Release Assets

Each release automatically includes:

### Binaries
- `lab-update-esxi-cert-Windows-x86_64.zip` - Windows AMD64/Intel
- `lab-update-esxi-cert-Windows-arm64.zip` - Windows ARM64
- `lab-update-esxi-cert-Linux-x86_64.tar.gz` - Linux AMD64/Intel
- `lab-update-esxi-cert-Linux-arm64.tar.gz` - Linux ARM64
- `lab-update-esxi-cert-Darwin-x86_64.tar.gz` - macOS Intel
- `lab-update-esxi-cert-Darwin-arm64.tar.gz` - macOS Apple Silicon

### Security Files
- `checksums.txt` - SHA256 checksums for all binaries
- `checksums.txt.sig` - GPG signature of checksums (if configured)

### Documentation
- `README.md` - Usage instructions
- `LICENSE` - License information
- `VERSION_BUILD.md` - Build and version information

## Pre-release Testing

Before creating a release, test the build locally:

```bash
# Install GoReleaser
go install github.com/goreleaser/goreleaser@latest

# Test configuration
goreleaser check

# Build snapshot (no tag required)
goreleaser build --single-target --snapshot --clean

# Test the binary
./dist/lab-update-esxi-cert_*/lab-update-esxi-cert* --version
```

## Troubleshooting

### Release Failed
- Check GitHub Actions logs in the repository
- Ensure the tag follows semantic versioning (starts with `v`)
- Verify all tests pass: `go test ./...`

### Missing Assets
- Check if GoReleaser configuration is valid: `goreleaser check`
- Ensure build completes successfully for all platforms

### Version Not Updated
- Verify ldflags are properly configured in `.goreleaser.yml`
- Check that the tag was pushed: `git tag -l`

## Manual Release (Emergency)

If GitHub Actions is unavailable:

```bash
# Set required environment variable
export GITHUB_TOKEN="your_github_token"

# Create release manually
goreleaser release --clean

# Or create without publishing
goreleaser release --snapshot --clean
```

## GPG Signing (Optional)

To enable GPG signing of releases:

1. **Generate GPG key** (if you don't have one):
   ```bash
   gpg --full-generate-key
   ```

2. **Add GPG secrets to GitHub**:
   - `GPG_PRIVATE_KEY`: Your private key (`gpg --armor --export-secret-keys YOUR_KEY_ID`)
   - `PASSPHRASE`: Your GPG key passphrase

3. **Releases will be automatically signed** with checksums and signatures included.

## Update Checking Integration

Once releases are published:

1. **Automatic update notifications**: Users will see update notifications during normal usage of the application
2. **No configuration needed**: The application automatically checks the correct GitHub repository
3. **Built-in version system**: Automatically detects new releases using GitHub's API without any user setup