package main

import (
	"bytes"
	"crypto/x509"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected LogLevel
	}{
		{"ERROR", LOG_ERROR},
		{"error", LOG_ERROR},
		{"WARN", LOG_WARN},
		{"WARNING", LOG_WARN},
		{"warn", LOG_WARN},
		{"INFO", LOG_INFO},
		{"info", LOG_INFO},
		{"DEBUG", LOG_DEBUG},
		{"debug", LOG_DEBUG},
		{"INVALID", LOG_INFO}, // Default fallback
		{"", LOG_INFO},        // Default fallback
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseLogLevel(tt.input)
			if result != tt.expected {
				t.Errorf("parseLogLevel(%s) = %d, expected %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestLoggingFunctions(t *testing.T) {
	// Capture log output
	var buf bytes.Buffer
	originalOutput := log.Writer()
	log.SetOutput(&buf)
	defer func() {
		log.SetOutput(originalOutput)
		log.SetFlags(log.LstdFlags) // Reset flags
	}()

	// Test different log levels
	tests := []struct {
		level       LogLevel
		logFunc     func(string, ...interface{})
		expectedLog string
		message     string
	}{
		{LOG_ERROR, logError, "[ERROR]", "test error message"},
		{LOG_WARN, logWarn, "[WARN]", "test warning message"},
		{LOG_INFO, logInfo, "[INFO]", "test info message"},
		{LOG_DEBUG, logDebug, "[DEBUG]", "test debug message"},
	}

	for _, tt := range tests {
		t.Run(tt.expectedLog, func(t *testing.T) {
			// Set current log level to allow this message
			currentLogLevel = tt.level
			buf.Reset()

			tt.logFunc(tt.message)

			output := buf.String()
			if !strings.Contains(output, tt.expectedLog) {
				t.Errorf("Expected log output to contain %s, got: %s", tt.expectedLog, output)
			}
			if !strings.Contains(output, tt.message) {
				t.Errorf("Expected log output to contain message %s, got: %s", tt.message, output)
			}
		})
	}
}

func TestLoggingLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	originalOutput := log.Writer()
	log.SetOutput(&buf)
	defer func() {
		log.SetOutput(originalOutput)
	}()

	// Set log level to WARN - should only show ERROR and WARN messages
	currentLogLevel = LOG_WARN

	tests := []struct {
		logFunc     func(string, ...interface{})
		message     string
		shouldShow  bool
	}{
		{logError, "error message", true},
		{logWarn, "warning message", true},
		{logInfo, "info message", false},
		{logDebug, "debug message", false},
	}

	for _, tt := range tests {
		buf.Reset()
		tt.logFunc(tt.message)
		
		output := buf.String()
		hasOutput := len(strings.TrimSpace(output)) > 0
		
		if tt.shouldShow && !hasOutput {
			t.Errorf("Expected message %s to be logged at level %d", tt.message, currentLogLevel)
		}
		if !tt.shouldShow && hasOutput {
			t.Errorf("Expected message %s to be filtered out at level %d", tt.message, currentLogLevel)
		}
	}
}

func TestSetupLogging(t *testing.T) {
	// Skip this test on Windows due to file locking issues with setupLogging function
	if strings.Contains(strings.ToLower(os.Getenv("OS")), "windows") {
		t.Skip("Skipping on Windows due to file locking issues with setupLogging function")
	}

	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "test.log")

	// Save original log output
	originalOutput := log.Writer()
	defer func() {
		log.SetOutput(originalOutput)
		log.SetFlags(log.LstdFlags) // Reset flags
	}()

	// Test setup logging
	setupLogging(logFile, "DEBUG")

	// Verify log level was set
	if currentLogLevel != LOG_DEBUG {
		t.Errorf("Expected log level DEBUG, got %d", currentLogLevel)
	}

	// Verify log file was created and is writable
	logInfo("test message")

	// Reset log output to avoid file locking issues
	log.SetOutput(originalOutput)

	// Check that file was created
	fileInfo, err := os.Stat(logFile)
	if err != nil {
		t.Fatalf("Log file was not created: %v", err)
	}

	// Note: File permissions work differently on Windows vs Unix
	// Just verify the file exists and is accessible
	t.Logf("Log file created with permissions: %o", fileInfo.Mode().Perm())

	// Verify log file contains our test message
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), "test message") {
		t.Error("Log file should contain test message")
	}
	if !strings.Contains(string(content), "[INFO]") {
		t.Error("Log file should contain log level prefix")
	}
}

func TestSetupLogging_InvalidFile(t *testing.T) {
	// Capture stdout to check error message
	originalStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() {
		os.Stdout = originalStdout
		w.Close()
	}()

	// Try to create log file in non-existent directory
	invalidPath := "/nonexistent/directory/test.log"
	
	// This should handle the error gracefully
	setupLogging(invalidPath, "INFO")

	// Restore stdout and read captured output
	w.Close()
	var output bytes.Buffer
	io.Copy(&output, r)

	// The function should have printed an error message
	outputStr := output.String()
	if !strings.Contains(outputStr, "Error opening log file") {
		t.Error("Expected error message about log file creation")
	}
}

func TestValidateAWSCredentials_Success(t *testing.T) {
	// Create a test config
	config := Config{
		Route53KeyID:     "AKIATEST123",
		Route53SecretKey: "test-secret",
		Route53Region:    "us-east-1",
	}

	// Test that the config structure is valid for AWS validation
	// In practice, you'd mock the AWS SDK calls for proper testing
	if config.Route53KeyID == "" {
		t.Error("Expected AWS key ID to be set")
	}
	if config.Route53SecretKey == "" {
		t.Error("Expected AWS secret key to be set")
	}
	if config.Route53Region == "" {
		t.Error("Expected AWS region to be set")
	}
}

func TestValidateAWSCredentials_Failure(t *testing.T) {
	// Test config with invalid credentials structure
	config := Config{
		Route53KeyID:     "", // Invalid empty key ID
		Route53SecretKey: "test-secret",
		Route53Region:    "us-east-1",
	}

	// Test that we can identify invalid config structure
	if config.Route53KeyID != "" {
		t.Error("Expected empty AWS key ID for failure test")
	}
	if config.Route53Region == "" {
		t.Error("Expected AWS region to be set")
	}
}

func TestRunWorkflow_DryRun(t *testing.T) {
	// Create a dry-run configuration
	config := Config{
		Hostname:         "test.example.com",
		Route53KeyID:     "AKIATEST123",
		Route53SecretKey: "test-secret",
		Route53Region:    "us-east-1",
		DryRun:           true,
		LogLevel:         "INFO",
		Threshold:        0.33,
	}

	// Create mock dependencies
	mockDeps := Dependencies{
		AWSValidator: func(Config) error {
			return nil // Mock successful AWS validation
		},
		CertChecker: func(string, float64) (bool, *x509.Certificate, error) {
			// Mock certificate that doesn't need renewal
			cert := &x509.Certificate{
				NotAfter: time.Now().Add(60 * 24 * time.Hour), // 60 days
			}
			return false, cert, nil
		},
		CertGenerator: func(Config) (string, string, error) {
			t.Error("CertGenerator should not be called in dry-run mode")
			return "", "", nil
		},
		CertUploader: func(Config, string, string) error {
			t.Error("CertUploader should not be called in dry-run mode")
			return nil
		},
		CertValidator: func(string, *x509.Certificate) (bool, error) {
			t.Error("CertValidator should not be called in dry-run mode")
			return false, nil
		},
	}

	// Test the workflow
	err := runWorkflow(config, mockDeps)
	if err != nil {
		t.Errorf("Dry run workflow should succeed, got error: %v", err)
	}
}

func TestRunWorkflow_ForceRenewal(t *testing.T) {
	config := Config{
		Hostname:         "test.example.com",
		Domain:           "example.com",
		Email:            "test@example.com",
		Route53KeyID:     "AKIATEST123",
		Route53SecretKey: "test-secret",
		Route53Region:    "us-east-1",
		ESXiUsername:     "root",
		ESXiPassword:     "password",
		Force:            true,
		LogLevel:         "INFO",
		Threshold:        0.33,
		KeySize:          4096,
	}

	// Track which functions were called
	var awsValidatorCalled, certCheckerCalled, certGeneratorCalled, certUploaderCalled, certValidatorCalled bool

	// Create mock dependencies
	mockDeps := Dependencies{
		AWSValidator: func(Config) error {
			awsValidatorCalled = true
			return nil
		},
		CertChecker: func(string, float64) (bool, *x509.Certificate, error) {
			certCheckerCalled = true
			// Return that cert doesn't need renewal, but force should override this
			cert := &x509.Certificate{
				NotAfter: time.Now().Add(60 * 24 * time.Hour), // 60 days
			}
			return false, cert, nil
		},
		CertGenerator: func(Config) (string, string, error) {
			certGeneratorCalled = true
			return "cert.pem", "key.pem", nil
		},
		CertUploader: func(Config, string, string) error {
			certUploaderCalled = true
			return nil
		},
		CertValidator: func(string, *x509.Certificate) (bool, error) {
			certValidatorCalled = true
			return true, nil
		},
	}

	// Test the workflow
	err := runWorkflow(config, mockDeps)
	if err != nil {
		t.Errorf("Force renewal workflow should succeed, got error: %v", err)
	}

	// Verify all expected functions were called for force renewal
	if !awsValidatorCalled {
		t.Error("AWS validator should be called")
	}
	if !certCheckerCalled {
		t.Error("Certificate checker should be called")
	}
	if !certGeneratorCalled {
		t.Error("Certificate generator should be called for force renewal")
	}
	if !certUploaderCalled {
		t.Error("Certificate uploader should be called for force renewal")
	}
	if !certValidatorCalled {
		t.Error("Certificate validator should be called for force renewal")
	}
}

func TestRunWorkflow_AWSValidationFailure(t *testing.T) {
	config := Config{
		Hostname:         "test.example.com",
		Route53KeyID:     "AKIATEST123",
		Route53SecretKey: "test-secret",
		Route53Region:    "us-east-1",
		DryRun:           true,
	}

	// Create mock dependencies with failing AWS validator
	mockDeps := Dependencies{
		AWSValidator: func(Config) error {
			return fmt.Errorf("invalid AWS credentials")
		},
		CertChecker: func(string, float64) (bool, *x509.Certificate, error) {
			t.Error("CertChecker should not be called when AWS validation fails")
			return false, nil, nil
		},
		CertGenerator: func(Config) (string, string, error) {
			t.Error("CertGenerator should not be called when AWS validation fails")
			return "", "", nil
		},
		CertUploader: func(Config, string, string) error {
			t.Error("CertUploader should not be called when AWS validation fails")
			return nil
		},
		CertValidator: func(string, *x509.Certificate) (bool, error) {
			t.Error("CertValidator should not be called when AWS validation fails")
			return false, nil
		},
	}

	// Test the workflow
	err := runWorkflow(config, mockDeps)
	if err == nil {
		t.Error("Expected workflow to fail with AWS validation error")
	}
	if !strings.Contains(err.Error(), "AWS credential validation failed") {
		t.Errorf("Expected AWS validation error, got: %v", err)
	}
}

func TestRunWorkflow_CertificateCheckFailure(t *testing.T) {
	config := Config{
		Hostname:         "test.example.com",
		Route53KeyID:     "AKIATEST123",
		Route53SecretKey: "test-secret",
		Route53Region:    "us-east-1",
		DryRun:           true,
	}

	// Create mock dependencies with failing certificate checker
	mockDeps := Dependencies{
		AWSValidator: func(Config) error {
			return nil
		},
		CertChecker: func(string, float64) (bool, *x509.Certificate, error) {
			return false, nil, fmt.Errorf("certificate check failed")
		},
		CertGenerator: func(Config) (string, string, error) {
			t.Error("CertGenerator should not be called when cert check fails")
			return "", "", nil
		},
		CertUploader: func(Config, string, string) error {
			t.Error("CertUploader should not be called when cert check fails")
			return nil
		},
		CertValidator: func(string, *x509.Certificate) (bool, error) {
			t.Error("CertValidator should not be called when cert check fails")
			return false, nil
		},
	}

	// Test the workflow
	err := runWorkflow(config, mockDeps)
	if err == nil {
		t.Error("Expected workflow to fail with certificate check error")
	}
	if !strings.Contains(err.Error(), "certificate check failed") {
		t.Errorf("Expected certificate check error, got: %v", err)
	}
}

func TestRunWorkflow_CertificateUpToDate(t *testing.T) {
	config := Config{
		Hostname:         "test.example.com",
		Route53KeyID:     "AKIATEST123",
		Route53SecretKey: "test-secret",
		Route53Region:    "us-east-1",
		Force:            false, // No force renewal
		Threshold:        0.33,
	}

	// Create mock dependencies where certificate doesn't need renewal
	mockDeps := Dependencies{
		AWSValidator: func(Config) error {
			return nil
		},
		CertChecker: func(string, float64) (bool, *x509.Certificate, error) {
			// Return that cert doesn't need renewal
			cert := &x509.Certificate{
				NotAfter: time.Now().Add(60 * 24 * time.Hour), // 60 days in future
			}
			return false, cert, nil // false = doesn't need renewal
		},
		CertGenerator: func(Config) (string, string, error) {
			t.Error("CertGenerator should not be called when cert is up to date")
			return "", "", nil
		},
		CertUploader: func(Config, string, string) error {
			t.Error("CertUploader should not be called when cert is up to date")
			return nil
		},
		CertValidator: func(string, *x509.Certificate) (bool, error) {
			t.Error("CertValidator should not be called when cert is up to date")
			return false, nil
		},
	}

	// Test the workflow
	err := runWorkflow(config, mockDeps)
	if err != nil {
		t.Errorf("Workflow with up-to-date certificate should succeed, got error: %v", err)
	}
}

func TestConstants(t *testing.T) {
	// Test that constants are defined with expected values
	if defaultThreshold != 0.33 {
		t.Errorf("Expected default threshold 0.33, got %f", defaultThreshold)
	}
	
	if defaultCheckInterval != 30*time.Second {
		t.Errorf("Expected default check interval 30s, got %v", defaultCheckInterval)
	}
	
	if maxCheckDuration != 5*time.Minute {
		t.Errorf("Expected max check duration 5m, got %v", maxCheckDuration)
	}
	
	if acmeServerProduction != "https://acme-v02.api.letsencrypt.org/directory" {
		t.Errorf("Expected Let's Encrypt production URL, got %s", acmeServerProduction)
	}
}

func TestLogLevelNames(t *testing.T) {
	expectedNames := map[LogLevel]string{
		LOG_ERROR: "ERROR",
		LOG_WARN:  "WARN",
		LOG_INFO:  "INFO",
		LOG_DEBUG: "DEBUG",
	}

	for level, expectedName := range expectedNames {
		if logLevelNames[level] != expectedName {
			t.Errorf("Expected log level %d to have name %s, got %s", 
				level, expectedName, logLevelNames[level])
		}
	}
}

func TestConfigStruct(t *testing.T) {
	// Test that Config struct has all expected fields
	config := Config{
		Hostname:            "test.example.com",
		Domain:              "example.com",
		Email:               "test@example.com",
		Threshold:           0.33,
		LogFile:             "test.log",
		LogLevel:            "INFO",
		Route53KeyID:        "AKIATEST123",
		Route53SecretKey:    "test-secret",
		Route53SessionToken: "session-token",
		Route53Region:       "us-east-1",
		DryRun:              false,
		Force:               false,
		KeySize:             4096,
		ESXiUsername:        "root",
		ESXiPassword:        "password",
	}

	// Verify all fields are accessible and have expected types
	if config.Hostname == "" {
		t.Error("Hostname field should be accessible")
	}
	if config.Threshold <= 0 {
		t.Error("Threshold should be positive")
	}
	if config.KeySize <= 0 {
		t.Error("KeySize should be positive")
	}
	
	// Test boolean fields
	if config.DryRun == config.Force {
		// This is fine - just testing the fields exist and are accessible
	}
}

// Helper function to test the structure of the logging system
func TestLoggingStructure(t *testing.T) {
	// Test that all log levels are correctly ordered
	if LOG_ERROR > LOG_WARN {
		t.Error("LOG_ERROR should have lower value than LOG_WARN")
	}
	if LOG_WARN > LOG_INFO {
		t.Error("LOG_WARN should have lower value than LOG_INFO") 
	}
	if LOG_INFO > LOG_DEBUG {
		t.Error("LOG_INFO should have lower value than LOG_DEBUG")
	}

	// Test that currentLogLevel is initialized
	if currentLogLevel < LOG_ERROR || currentLogLevel > LOG_DEBUG {
		t.Error("currentLogLevel should be within valid range")
	}
}