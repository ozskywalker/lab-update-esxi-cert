# ESXi Certificate Manager

A Golang utility for automating SSL certificate renewals for VMware ESXi hosts in lab environments using Let's Encrypt and AWS Route53.

[![Go Version](https://img.shields.io/badge/Go-1.24.4-blue.svg)](https://golang.org/doc/devel/release.html)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](https://www.gnu.org/licenses/gpl-3.0)
![Claude Used](https://img.shields.io/badge/Claude-Used-4B5AEA)

## Overview

This utility addresses a specific need: automatically managing SSL certificates for standalone VMware ESXi 6.7 hosts (without vCenter) in lab environments. It uses Let's Encrypt for free certificate issuance and AWS Route53 for DNS validation.

Key features:
- Monitors certificate expiration and renews based on configurable thresholds (default = 33%)
- Can be triggered via cron, script, or manually
- Can force certificate renewal
- Uses DNS validation to avoid exposing ESXi to public internet
- Auto backup of old cert (1 copy max. retained)
- Auto start/stop of SSH service (TSM-SSH)

## Prerequisites

- Go 1.24.4+
- AWS Account with Route53 access
- VMware ESXi 6.7 host(s)
- Domain name configured in AWS Route53

## Installation

```bash
git clone https://github.com/yourusername/lab-update-esxi-cert.git
cd lab-update-esxi-cert
go build
```

## Usage

Basic usage to check a certificate without renewing (uses existing env variables from AWS):

```bash
./lab-update-esxi-cert --hostname esxi.example.com --dry-run
```

Full renewal with permanent AWS credentials:

```bash
./lab-update-esxi-cert --hostname esxi.example.com \
  --domain example.com \
  --email admin@example.com \
  --esxi-user root \
  --esxi-pass password \
  --aws-key-id AKIAXXXXXXXX \
  --aws-secret-key xxxxxxxxxx \
  --aws-region us-east-1 \
  --threshold 0.33 \
  --key-size 4096 \
  --log /var/log/esxi-cert.log
```

Using temporary AWS credentials (STS assume-role):

```bash
./lab-update-esxi-cert --hostname esxi.example.com \
  --domain example.com \
  --email admin@example.com \
  --esxi-user root \
  --esxi-pass password \
  --aws-key-id ASIAXXXXXXXX \
  --aws-secret-key xxxxxxxxxx \
  --aws-session-token IQoJb3JpZ2luX2VjEA... \
  --aws-region us-east-1
```

Force certificate renewal regardless of expiration:

```bash
./lab-update-esxi-cert --hostname esxi.example.com \
  --domain example.com \
  --email admin@example.com \
  --esxi-user root \
  --esxi-pass password \
  --aws-key-id AKIAXXXXXXXX \
  --aws-secret-key xxxxxxxxxx \
  --force
```

## Configuration Options

**Important**: The certificate is issued for the `--hostname` value (e.g., `esxlab01.longbranch.lwalker.me`), while `--domain` specifies the DNS zone managed by Route53 for validation (e.g., `longbranch.lwalker.me`). The hostname must be within the specified domain.

**Note**: Complex passwords with many special characters may cause SOAP API authentication failures. If you encounter authentication issues, try using a simpler password temporarily.

### Required config options

| Option | Environment Variable | Description | Default | Required |
|--------|---------------------|-------------|---------|----------|
| `--hostname` | `ESXI_HOSTNAME` | ESXi host FQDN (certificate subject) | | Yes |
| `--domain` | `AWS_ROUTE53_DOMAIN` | DNS domain managed by Route53 (for DNS validation) | | Yes (unless dry-run) |
| `--email` | `EMAIL` | Email for Let's Encrypt registration | | Yes (unless dry-run) |
| `--esxi-user` | `ESXI_USERNAME` | ESXi username | | Yes (unless dry-run) |
| `--esxi-pass` | `ESXI_PASSWORD` | ESXi password | | Yes (unless dry-run) |
| `--aws-key-id` | `AWS_ACCESS_KEY_ID` | AWS Access Key ID | | Yes |
| `--aws-secret-key` | `AWS_SECRET_ACCESS_KEY` | AWS Secret Access Key | | Yes |

### Optional config options

| Option | Environment Variable | Description | Default | Required |
|--------|---------------------|-------------|---------|----------|
| `--aws-session-token` | `AWS_SESSION_TOKEN` | AWS Session Token (for temporary credentials) | | No |
| `--aws-region` | `AWS_REGION` | AWS Region for Route53 | us-east-1 | No |
| `--threshold` | `CERT_THRESHOLD` | Renewal threshold (remaining lifetime fraction) | 0.33 (33%) | No |
| `--key-size` | `CERT_KEY_SIZE` | RSA key size for certificates (2048, 4096) - generates SHA256WithRSA signatures | 4096 | No |
| `--log` | `LOG_FILE` | Path to log file | ./lab-update-esxi-cert.log | No |
| `--log-level` | `LOG_LEVEL` | Log level (ERROR, WARN, INFO, DEBUG) | INFO | No |
| `--dry-run` | `DRY_RUN` | Check certificate without renewal | false | No |
| `--force` | `FORCE_RENEWAL` | Force certificate renewal regardless of expiration threshold | false | No |

## Certificate Renewal Logic

The tool determines if renewal is needed by calculating the certificate's remaining lifetime as a percentage of its total validity period:

```
percentRemaining = (NotAfter - Now) / (NotAfter - NotBefore)
```

If `percentRemaining` falls below the specified threshold (default 33%), the certificate is renewed.

For example, with the default 33% threshold:
- For a 90-day certificate (Let's Encrypt default), renewal occurs when there are 30 days or less remaining
- For a 1-year certificate, renewal occurs when there are ~4 months or less remaining

## Configuration Precedence

Configuration options are chosen based on the following precedence:

  1. **Defaults:** threshold: 0.33
  2. **Config file:** {"threshold": 0.5} → threshold: 0.5
  3. **Environment variables:** CERT_THRESHOLD=0.6 → threshold: 0.6
  4. **Command-line:** ```--threshold 0.7``` → threshold: 0.7 (final value)

## AWS Credentials and Authentication

The tool supports both permanent and temporary AWS credentials for Route53 DNS validation:

### Permanent Credentials
- Standard AWS Access Key ID and Secret Access Key
- Can be provided via environment variables or command-line flags

### Temporary Credentials (STS Assume Role)
- Supports AWS Session Tokens for temporary credentials
- Useful for cross-account access or enhanced security
- Session tokens are typically obtained through AWS STS assume-role operations

### Credential Validation
The tool validates AWS credentials at startup using AWS STS GetCallerIdentity before proceeding with certificate operations. This validation occurs for both dry-run and normal execution modes to ensure credentials are valid and have proper permissions.

**Environment Variables:**
- `AWS_ACCESS_KEY_ID`
- `AWS_SECRET_ACCESS_KEY`
- `AWS_SESSION_TOKEN` (optional, for temporary credentials)
- `AWS_REGION` (optional, defaults to us-east-1)

## Certificate Installation Process

1. **AWS Credential Validation**: Validates AWS credentials using STS GetCallerIdentity
2. **Certificate Check**: Connects to the ESXi host and retrieves the current certificate
3. **Threshold Evaluation**: Determines if renewal is needed based on configured threshold
4. **Certificate Generation**: Uses Let's Encrypt ACME protocol with Route53 DNS validation (RSA signatures only)
5. **SSH Service Management**: Uses SOAP API to start TSM-SSH service if not already running
6. **Certificate Backup**: Creates backup copies of existing certificates (rui.crt.backup, rui.key.backup)  
7. **Certificate Installation**: Copies new certificate and key files to /etc/vmware/ssl/ via SSH
8. **Service Restart**: Restarts hostd and vpxa services via SSH to apply new certificates
9. **SSH Cleanup**: Stops TSM-SSH service via SOAP API
10. **Validation**: Verifies the new certificate is properly installed

## Limitations

- Currently only supports AWS Route53 for DNS challenges
- Designed for standalone ESXi hosts, not vCenter-managed environments
- Limited error recovery capabilities in this early version

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the GNU GPL v3 License - see the LICENSE file for details.
