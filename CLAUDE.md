# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Golang utility for automating SSL certificate renewals for VMware ESXi hosts using Let's Encrypt and AWS Route53. The application monitors certificate expiration and renews certificates when they approach configurable thresholds.

## Build and Development Commands

```bash
# Build the application
go build

# Build with specific output name
go build -o lab-update-esxi-cert

# Run tests (standard Go testing)
go test ./...

# Format code
go fmt ./...

# Vet code for issues
go vet ./...

# Get dependencies
go mod tidy

# Clean build artifacts
go clean
```

## Architecture

The application is structured as a single Go module with four main files:

- **main.go**: Entry point containing the main workflow logic and structured logging
- **config.go**: Structured configuration management supporting multiple sources (files, env vars, CLI flags)
- **cmdline_validation.go**: Command-line argument parsing and validation using the configuration manager
- **lego_cert_work.go**: Certificate operations using the Lego ACME library (check, generate, validate)

### Key Components

**Config struct**: Central configuration holder containing all runtime parameters including AWS credentials, ESXi credentials, certificate options, and operational flags.

**ConfigManager**: Structured configuration management system that supports loading configuration from multiple sources with proper precedence handling:
- Defaults (lowest precedence)
- JSON configuration files
- Environment variables 
- Command-line flags (highest precedence)

**Structured Logging**: Multi-level logging system (ERROR, WARN, INFO, DEBUG) with secure log file permissions (0600) and configurable output levels.

**Certificate workflow**:
1. Validate AWS credentials via STS GetCallerIdentity call
2. Check current certificate expiration against threshold
3. Generate new certificate via Let's Encrypt ACME with Route53 DNS validation
4. Upload certificate to ESXi host via REST API
5. Validate installation

**Dependencies**:
- `github.com/go-acme/lego/v4`: ACME protocol implementation
- `github.com/aws/aws-sdk-go`: AWS Route53 integration (v1)
- `github.com/aws/aws-sdk-go-v2`: AWS STS for credential validation (v2)
- Standard Go crypto/tls libraries for certificate handling

### Certificate Renewal Logic

Renewal threshold is calculated as: `(NotAfter - Now) / (NotAfter - NotBefore)`

Default threshold is 0.33 (33% remaining lifetime), meaning a 90-day Let's Encrypt certificate renews when 30 days remain.

## AWS Credentials and Environment Variables

The application requires AWS credentials for Route53 DNS validation and validates them on startup for both dry-run and normal execution.

**Configuration Sources** (in order of precedence, highest to lowest):

**Command-line flags** (highest precedence):
- `-config`: Path to JSON configuration file
- `-hostname`, `-domain`, `-email`: Core certificate settings
- `-aws-key-id`, `-aws-secret-key`, `-aws-session-token`, `-aws-region`: AWS credentials
- `-esxi-user`, `-esxi-pass`: ESXi credentials
- `-log`, `-log-level`: Logging configuration
- `-threshold`, `-key-size`: Certificate renewal settings
- `-dry-run`, `-force`: Operational modes

**Environment Variables**:
- `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`, `AWS_REGION`: AWS credentials
- `ESXI_HOSTNAME`, `ESXI_DOMAIN`, `ESXI_EMAIL`: Core settings
- `ESXI_USERNAME`, `ESXI_PASSWORD`: ESXi credentials
- `ESXI_LOG_FILE`, `ESXI_LOG_LEVEL`: Logging configuration
- `ESXI_THRESHOLD`, `ESXI_KEY_SIZE`: Certificate settings
- `ESXI_DRY_RUN`, `ESXI_FORCE`: Operational modes

**JSON Configuration File**:
- Supports all configuration options in JSON format
- Specified via `-config` flag
- Example: `{"hostname": "esxi.example.com", "domain": "example.com", "log_level": "INFO"}`

**Credential Validation**: Uses AWS STS GetCallerIdentity to validate credentials before proceeding with certificate operations.