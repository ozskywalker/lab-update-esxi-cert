package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"time"
)

// Constants
const (
	defaultThreshold     = 0.33
	defaultCheckInterval = 30 * time.Second
	maxCheckDuration     = 5 * time.Minute
	acmeServerProduction = "https://acme-v02.api.letsencrypt.org/directory"
)

// Configuration struct for the application
type Config struct {
	Hostname         string
	Domain           string
	Email            string
	Threshold        float64
	LogFile          string
	Route53KeyID     string
	Route53SecretKey string
	Route53Region    string
	DryRun           bool
	KeyType          string
	KeySize          int
	ESXiUsername     string
	ESXiPassword     string
}

// Set up logging to file
func setupLogging(logFile string) {
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Printf("Error opening log file: %v\n", err)
		return
	}

	// Set up multi-writer to log to both file and stdout
	multiWriter := io.MultiWriter(os.Stdout, file)
	log.SetOutput(multiWriter)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	log.Printf("Logging to %s", logFile)
}

// Main function
func main() {
	// Parse the command-line arguments
	config, err := parseArgs()
	if err != nil {
		fmt.Printf("Error parsing arguments: %s\n", err)
		os.Exit(1)
	}

	// Set up logging
	setupLogging(config.LogFile)

	// If dry run, just check the certificate
	if config.DryRun {
		log.Println("Running in dry-run mode. Will only check certificate expiration.")
		checkCertificate(config.Hostname, config.Threshold)
		return
	}

	// Check if the certificate needs renewal
	needsRenewal, certInfo := checkCertificate(config.Hostname, config.Threshold)
	if !needsRenewal {
		log.Printf("Certificate for %s is still valid (expires on %s) and doesn't need renewal yet.",
			config.Hostname, certInfo.NotAfter.Format(time.RFC3339))
		return
	}

	// Generate a new certificate
	log.Println("Generating new certificate...")
	certPath, keyPath, err := generateCertificate(config)
	if err != nil {
		log.Fatalf("Failed to generate certificate: %v", err)
	}
	log.Printf("Certificate generated successfully: %s", certPath)

	// Upload the certificate to ESXi
	log.Println("Uploading certificate to ESXi server...")
	err = uploadCertificate(config, certPath, keyPath)
	if err != nil {
		log.Fatalf("Failed to upload certificate: %v", err)
	}
	log.Println("Certificate uploaded successfully.")

	// Validate the certificate installation
	log.Println("Validating new certificate installation...")
	validated := validateCertificate(config.Hostname, certInfo)
	if validated {
		log.Println("New certificate successfully validated!")
	} else {
		log.Println("Warning: Could not validate new certificate within the timeout period.")
	}
}
