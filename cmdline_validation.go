package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

// Parse command-line arguments and return a Config
func parseArgs() (Config, error) {
	config := Config{
		Threshold: defaultThreshold,
		KeyType:   "RSA",
		KeySize:   4096,
	}

	// Define flags
	flag.StringVar(&config.Hostname, "hostname", "", "ESXi server hostname")
	flag.StringVar(&config.Domain, "domain", "", "Domain name for the certificate")
	flag.StringVar(&config.Email, "email", "", "Email address for ACME registration")
	flag.Float64Var(&config.Threshold, "threshold", defaultThreshold, "Renewal threshold (e.g., 0.33 for 1/3 of remaining lifetime)")
	flag.StringVar(&config.LogFile, "log", "", "Path to log file (defaults to binary_name.log)")
	flag.StringVar(&config.Route53KeyID, "aws-key-id", "", "AWS Access Key ID for Route53")
	flag.StringVar(&config.Route53SecretKey, "aws-secret-key", "", "AWS Secret Access Key for Route53")
	flag.StringVar(&config.Route53Region, "aws-region", "us-east-1", "AWS Region for Route53")
	flag.BoolVar(&config.DryRun, "dry-run", false, "Only check certificate without renewing")
	flag.StringVar(&config.KeyType, "key-type", "RSA", "Key type for certificate (RSA or ECDSA)")
	flag.IntVar(&config.KeySize, "key-size", 4096, "Key size for RSA certificates")
	flag.StringVar(&config.ESXiUsername, "esxi-user", "", "ESXi server username")
	flag.StringVar(&config.ESXiPassword, "esxi-pass", "", "ESXi server password")

	// Parse flags
	flag.Parse()

	// Print help if no arguments provided
	if len(os.Args) <= 1 {
		printHelp()
		os.Exit(0)
	}

	// Validate required parameters
	if config.Hostname == "" {
		return config, fmt.Errorf("hostname is required")
	}

	if !config.DryRun {
		if config.Domain == "" {
			return config, fmt.Errorf("domain is required for certificate renewal")
		}
		if config.Email == "" {
			return config, fmt.Errorf("email is required for ACME registration")
		}
		if config.ESXiUsername == "" || config.ESXiPassword == "" {
			return config, fmt.Errorf("ESXi username and password are required for certificate upload")
		}

		// Check for AWS credentials in environment if not provided via flags
		if config.Route53KeyID == "" {
			config.Route53KeyID = os.Getenv("AWS_ACCESS_KEY_ID")
		}
		if config.Route53SecretKey == "" {
			config.Route53SecretKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
		}
		if config.Route53KeyID == "" || config.Route53SecretKey == "" {
			return config, fmt.Errorf("AWS credentials for Route53 are required")
		}
	}

	// Set default log file if not specified
	if config.LogFile == "" {
		executableName := filepath.Base(os.Args[0])
		config.LogFile = executableName + ".log"
	}

	return config, nil
}

// Print help and usage examples
func printHelp() {
	fmt.Println("ESXi Certificate Manager")
	fmt.Println("=======================")
	fmt.Println("This tool checks and automatically renews SSL certificates for ESXi servers.")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Printf("  %s [options]\n", os.Args[0])
	fmt.Println("")
	fmt.Println("Options:")
	flag.PrintDefaults()
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Printf("  # Check certificate only\n")
	fmt.Printf("  %s -hostname esxi.example.com -dry-run\n", os.Args[0])
	fmt.Println("")
	fmt.Printf("  # Check and renew certificate if needed\n")
	fmt.Printf("  %s -hostname esxi.example.com -domain example.com -email admin@example.com \\\n", os.Args[0])
	fmt.Printf("    -esxi-user root -esxi-pass password -aws-key-id AKIAXXXXXXXX -aws-secret-key xxxxxxxx\n")
	fmt.Println("")
	fmt.Printf("  # With custom threshold and log file\n")
	fmt.Printf("  %s -hostname esxi.example.com -domain example.com -email admin@example.com \\\n", os.Args[0])
	fmt.Printf("    -esxi-user root -esxi-pass password -threshold 0.5 -log /var/log/esxi-cert.log\n")
}
