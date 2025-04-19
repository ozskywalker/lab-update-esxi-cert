# ESXi Certificate Manager

A Golang utility for automating SSL certificate renewals for VMware ESXi hosts in lab environments using Let's Encrypt and AWS Route53.

## 🚧 Work in Progress 🚧

This tool is currently under active development and is not yet recommended for production use. Features may change, and additional testing is needed before it's considered stable.

## Overview

This utility addresses a specific need: automatically managing SSL certificates for standalone VMware ESXi 6.7 hosts (without vCenter) in lab environments. It uses Let's Encrypt for free certificate issuance and AWS Route53 for DNS validation.

Key features:
- Monitors certificate expiration and renews based on configurable thresholds
- Uses Let's Encrypt ACME protocol for certificate issuance
- Employs DNS validation through AWS Route53
- Uploads and installs certificates directly to ESXi hosts via the host API
- Optionally runs in dry-run mode to check certificates without renewal

## Prerequisites

- Go 1.24+
- AWS Account with Route53 access
- VMware ESXi 6.7 host(s)
- Domain name configured in Route53

## Installation

```bash
git clone https://github.com/yourusername/lab-update-esxi-cert.git
cd lab-update-esxi-cert
go build
```

## Usage

Basic usage to check a certificate without renewing (uses existing env variables from AWS):

```bash
./lab-update-esxi-cert -hostname esxi.example.com -dry-run
```

Full renewal with all options:

```bash
./lab-update-esxi-cert -hostname esxi.example.com \
  -domain example.com \
  -email admin@example.com \
  -esxi-user root \
  -esxi-pass password \
  -aws-key-id AKIAXXXXXXXX \
  -aws-secret-key xxxxxxxxxx \
  -aws-region us-east-1 \
  -threshold 0.33 \
  -key-type RSA \
  -key-size 4096 \
  -log /var/log/esxi-cert.log
```

## Configuration Options

| Option | Description | Default | Required |
|--------|-------------|---------|----------|
| `-hostname` | ESXi host FQDN | None | Yes |
| `-domain` | Domain for certificate | None | Yes (unless dry-run) |
| `-email` | Email for Let's Encrypt registration | None | Yes (unless dry-run) |
| `-esxi-user` | ESXi username | None | Yes (unless dry-run) |
| `-esxi-pass` | ESXi password | None | Yes (unless dry-run) |
| `-aws-key-id` | AWS Access Key ID | From env vars | Yes (unless dry-run) |
| `-aws-secret-key` | AWS Secret Access Key | From env vars | Yes (unless dry-run) |
| `-aws-region` | AWS Region for Route53 | us-east-1 | No |
| `-threshold` | Renewal threshold (remaining lifetime fraction) | 0.33 (33%) | No |
| `-key-type` | Certificate key type (RSA or ECDSA) | RSA | No |
| `-key-size` | Key size for RSA certificates | 4096 | No |
| `-log` | Path to log file | ./lab-update-esxi-cert.log | No |
| `-dry-run` | Check certificate without renewal | false | No |

## Certificate Renewal Logic

The tool determines if renewal is needed by calculating the certificate's remaining lifetime as a percentage of its total validity period:

```
percentRemaining = (NotAfter - Now) / (NotAfter - NotBefore)
```

If `percentRemaining` falls below the specified threshold (default 33%), the certificate is renewed.

For example, with the default 33% threshold:
- For a 90-day certificate (Let's Encrypt default), renewal occurs when there are 30 days or less remaining
- For a 1-year certificate, renewal occurs when there are ~4 months or less remaining

## Certificate Authentication Process

1. **Certificate Check**: Connects to the ESXi host and retrieves the current certificate
2. **Threshold Evaluation**: Determines if renewal is needed based on configured threshold
3. **Certificate Generation**: Uses Let's Encrypt ACME protocol with Route53 DNS validation
4. **Certificate Upload**: Uploads the new certificate to the ESXi host
5. **Validation**: Verifies the new certificate is properly installed

## Limitations

- Currently only supports AWS Route53 for DNS challenges
- Designed for standalone ESXi hosts, not vCenter-managed environments
- Limited error recovery capabilities in this early version

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the GNU GPL v3 License - see the LICENSE file for details.
