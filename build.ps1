# Build script for lab-update-esxi-cert with version injection (PowerShell)
# This script demonstrates how to build the application with version information
# injected at build time using Go's -ldflags parameter

param(
    [string]$Output = "lab-update-esxi-cert.exe",
    [string]$Version = "",
    [string]$Commit = "",
    [string]$BuildDate = "",
    [string]$GitTag = ""
)

# Set error action preference to stop on errors
$ErrorActionPreference = "Stop"

Write-Host "Building ESXi Certificate Manager..." -ForegroundColor Green
Write-Host ""

# Get version information from Git if not provided
if ([string]::IsNullOrEmpty($Version)) {
    try {
        $Version = git describe --tags --always --dirty 2>$null
        if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrEmpty($Version)) {
            $Version = "development"
        }
    } catch {
        $Version = "development"
    }
}

if ([string]::IsNullOrEmpty($Commit)) {
    try {
        $Commit = git rev-parse HEAD 2>$null
        if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrEmpty($Commit)) {
            $Commit = "unknown"
        }
    } catch {
        $Commit = "unknown"
    }
}

if ([string]::IsNullOrEmpty($BuildDate)) {
    # Use ISO 8601 format in UTC
    $BuildDate = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
}

if ([string]::IsNullOrEmpty($GitTag)) {
    try {
        $GitTag = git describe --tags --exact-match 2>$null
        if ($LASTEXITCODE -ne 0) {
            $GitTag = ""
        }
    } catch {
        $GitTag = ""
    }
}

# Package path for version variables
$VersionPkg = "lab-update-esxi-cert/internal/version"

# Build flags - properly escape spaces and special characters
$LdFlags = @(
    "-X '$VersionPkg.Version=$Version'"
    "-X '$VersionPkg.GitCommit=$Commit'"
    "-X '$VersionPkg.BuildDate=$BuildDate'"
    "-X '$VersionPkg.GitTag=$GitTag'"
) -join " "

Write-Host "Building $Output with version information:" -ForegroundColor Yellow
Write-Host "  Version:    $Version" -ForegroundColor Cyan
Write-Host "  Git Commit: $Commit" -ForegroundColor Cyan
Write-Host "  Git Tag:    $GitTag" -ForegroundColor Cyan
Write-Host "  Build Date: $BuildDate" -ForegroundColor Cyan
Write-Host ""

# Build the application
try {
    $BuildCmd = "go build -ldflags `"$LdFlags`" -o `"$Output`""
    Write-Host "Executing: $BuildCmd" -ForegroundColor Gray
    
    # Use Invoke-Expression to properly handle the command with quotes
    Invoke-Expression $BuildCmd
    
    if ($LASTEXITCODE -eq 0) {
        Write-Host ""
        Write-Host "✅ Build completed successfully: $Output" -ForegroundColor Green
        Write-Host ""
        Write-Host "You can verify the version with:" -ForegroundColor Yellow
        Write-Host "  .\$Output --version" -ForegroundColor Cyan
    } else {
        throw "Go build failed with exit code $LASTEXITCODE"
    }
} catch {
    Write-Host ""
    Write-Host "❌ Build failed: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}