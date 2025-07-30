package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"lab-update-esxi-cert/internal/version"
)

// Constants
const (
	defaultThreshold     = 0.33
	defaultCheckInterval = 30 * time.Second
	maxCheckDuration     = 5 * time.Minute
	acmeServerProduction = "https://acme-v02.api.letsencrypt.org/directory"
)

// Log levels
type LogLevel int

const (
	LOG_ERROR LogLevel = iota
	LOG_WARN
	LOG_INFO
	LOG_DEBUG
)

var (
	currentLogLevel LogLevel = LOG_INFO
	logLevelNames            = map[LogLevel]string{
		LOG_ERROR: "ERROR",
		LOG_WARN:  "WARN",
		LOG_INFO:  "INFO",
		LOG_DEBUG: "DEBUG",
	}
)

// Configuration struct for the application
type Config struct {
	Hostname            string
	Domain              string
	Email               string
	Threshold           float64
	LogFile             string
	LogLevel            string
	Route53KeyID        string
	Route53SecretKey    string
	Route53SessionToken string
	Route53Region       string
	DryRun              bool
	Force               bool
	KeySize             int
	ESXiUsername        string
	ESXiPassword        string
}

// Parse log level from string
func parseLogLevel(level string) LogLevel {
	switch strings.ToUpper(level) {
	case "ERROR":
		return LOG_ERROR
	case "WARN", "WARNING":
		return LOG_WARN
	case "INFO":
		return LOG_INFO
	case "DEBUG":
		return LOG_DEBUG
	default:
		return LOG_INFO
	}
}

// Logging functions with level control
func logError(format string, args ...interface{}) {
	if currentLogLevel >= LOG_ERROR {
		log.Printf("[ERROR] "+format, args...)
	}
}

func logWarn(format string, args ...interface{}) {
	if currentLogLevel >= LOG_WARN {
		log.Printf("[WARN] "+format, args...)
	}
}

func logInfo(format string, args ...interface{}) {
	if currentLogLevel >= LOG_INFO {
		log.Printf("[INFO] "+format, args...)
	}
}

func logDebug(format string, args ...interface{}) {
	if currentLogLevel >= LOG_DEBUG {
		log.Printf("[DEBUG] "+format, args...)
	}
}

// Set up logging to file with secure permissions
func setupLogging(logFile, logLevel string) {
	// Set log level
	currentLogLevel = parseLogLevel(logLevel)

	// Create log file with secure permissions (owner read/write only)
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		fmt.Printf("Error opening log file: %v\n", err)
		return
	}

	// Set up multi-writer to log to both file and stdout
	multiWriter := io.MultiWriter(os.Stdout, file)
	log.SetOutput(multiWriter)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	logInfo("Logging to %s with level %s", logFile, logLevelNames[currentLogLevel])
}

// Validate AWS credentials by making a simple API call
func validateAWSCredentials(config Config) error {
	logDebug("Validating AWS credentials...")

	// Create a simple AWS session to test credentials
	cfg, err := awsConfig.LoadDefaultConfig(context.TODO(),
		awsConfig.WithRegion(config.Route53Region),
		awsConfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			config.Route53KeyID,
			config.Route53SecretKey,
			config.Route53SessionToken,
		)),
	)
	if err != nil {
		return fmt.Errorf("failed to create AWS config: %v", err)
	}

	// Create STS client to test credentials
	stsClient := sts.NewFromConfig(cfg)

	// Call GetCallerIdentity to validate credentials
	_, err = stsClient.GetCallerIdentity(context.TODO(), &sts.GetCallerIdentityInput{})
	if err != nil {
		return fmt.Errorf("AWS credential validation failed: %v", err)
	}

	logDebug("AWS credentials validated successfully")
	return nil
}

// Main function
func main() {
	// Parse the command-line arguments
	config, err := parseArgs()
	if err != nil {
		logError("Error parsing arguments: %s\n", err)
		os.Exit(1)
	}

	// Set up logging
	setupLogging(config.LogFile, config.LogLevel)

	// Log version information
	v := version.Get()
	logInfo("Starting %s", v.String())

	// Check for updates and display notification
	if updateMsg := version.GetUpdateNotification(); updateMsg != "" {
		logInfo(updateMsg)
		fmt.Println(updateMsg)
	}

	// Validate AWS credentials (required for both dry-run and normal execution)
	err = validateAWSCredentials(config)
	if err != nil {
		logError("AWS credential validation failed: %v", err)
		os.Exit(1)
	}

	// If dry run, just check the certificate
	if config.DryRun {
		logInfo("Running in dry-run mode. Will only check certificate expiration.")
		checkCertificate(config.Hostname, config.Threshold)
		return
	}

	// Check if the certificate needs renewal (or if force is enabled)
	needsRenewal, certInfo := checkCertificate(config.Hostname, config.Threshold)
	if config.Force {
		logInfo("Force renewal enabled - bypassing expiration threshold check")
		needsRenewal = true //nolint:ineffassign // Intentional override for force renewal
	} else if !needsRenewal {
		logInfo("Certificate for %s is still valid (expires on %s) and doesn't need renewal yet.",
			config.Hostname, certInfo.NotAfter.Format(time.RFC3339))
		return
	}

	// Generate a new certificate
	logInfo("Generating new certificate...")
	certPath, keyPath, err := generateCertificate(config)
	if err != nil {
		logError("Failed to generate certificate: %v", err)
		os.Exit(1)
	}
	logInfo("Certificate generated successfully: %s", certPath)

	// Upload the certificate to ESXi
	logInfo("Uploading certificate to ESXi server...")
	err = uploadCertificate(config, certPath, keyPath)
	if err != nil {
		logError("Failed to upload certificate: %v", err)
		os.Exit(1)
	}
	logInfo("Certificate uploaded successfully.")

	// Validate the certificate installation
	logInfo("Validating new certificate installation...")
	validated := validateCertificate(config.Hostname, certInfo)
	if validated {
		logInfo("New certificate successfully validated!")
	} else {
		logWarn("Could not validate new certificate within the timeout period.")
	}
}
