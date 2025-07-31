package main

import (
	"flag"
	"os"
	"strings"
	"testing"

	"lab-update-esxi-cert/testutil"
)

func TestParseArgs_ValidConfiguration(t *testing.T) {
	// Reset flag package for each test
	resetFlags()

	// Set up valid command line arguments
	oldArgs := os.Args
	os.Args = []string{
		"test-program",
		"-hostname", "test.example.com",
		"-domain", "example.com",
		"-email", "test@example.com",
		"-aws-key-id", "AKIATEST123",
		"-aws-secret-key", "test-secret",
		"-esxi-user", "root",
		"-esxi-pass", "password",
	}
	defer func() { os.Args = oldArgs }()

	config, err := parseArgs()
	if err != nil {
		t.Fatalf("Expected valid configuration to parse successfully, got error: %v", err)
	}

	// Verify parsed values
	if config.Hostname != "test.example.com" {
		t.Errorf("Expected hostname test.example.com, got %s", config.Hostname)
	}
	if config.Domain != "example.com" {
		t.Errorf("Expected domain example.com, got %s", config.Domain)
	}
	if config.Email != "test@example.com" {
		t.Errorf("Expected email test@example.com, got %s", config.Email)
	}
	if config.Route53KeyID != "AKIATEST123" {
		t.Errorf("Expected AWS key ID AKIATEST123, got %s", config.Route53KeyID)
	}
	if config.Route53SecretKey != "test-secret" {
		t.Errorf("Expected AWS secret test-secret, got %s", config.Route53SecretKey)
	}
	if config.ESXiUsername != "root" {
		t.Errorf("Expected ESXi username root, got %s", config.ESXiUsername)
	}
	if config.ESXiPassword != "password" {
		t.Errorf("Expected ESXi password password, got %s", config.ESXiPassword)
	}
}

func TestParseArgs_ConfigFile(t *testing.T) {
	resetFlags()

	// Create a temporary config file
	configBuilder := testutil.NewConfigBuilder()
	tempDir := t.TempDir()
	configFile := tempDir + "/test.json"

	err := configBuilder.WriteToFile(configFile)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	oldArgs := os.Args
	os.Args = []string{
		"test-program",
		"-config", configFile,
	}
	defer func() { os.Args = oldArgs }()

	config, err := parseArgs()
	if err != nil {
		t.Fatalf("Expected config file to parse successfully, got error: %v", err)
	}

	// Verify values from config file
	if config.Hostname != "test.example.com" {
		t.Errorf("Expected hostname from config file, got %s", config.Hostname)
	}
}

func TestParseArgs_EnvironmentVariables(t *testing.T) {
	resetFlags()

	// Set up environment variables using testutil
	configBuilder := testutil.NewConfigBuilder().
		WithHostname("env-test.example.com").
		WithEmail("env@example.com")

	cleanup := configBuilder.SetEnv()
	defer cleanup()

	oldArgs := os.Args
	os.Args = []string{
		"test-program",
		"-aws-key-id", "AKIATEST123",
		"-aws-secret-key", "test-secret",
		"-esxi-user", "root",
		"-esxi-pass", "password",
	}
	defer func() { os.Args = oldArgs }()

	config, err := parseArgs()
	if err != nil {
		t.Fatalf("Expected environment variables to be loaded, got error: %v", err)
	}

	// Verify environment variable was loaded
	if config.Hostname != "env-test.example.com" {
		t.Errorf("Expected hostname from environment variable, got %s", config.Hostname)
	}
	if config.Email != "env@example.com" {
		t.Errorf("Expected email from environment variable, got %s", config.Email)
	}
}

func TestParseArgs_PrecedenceOrder(t *testing.T) {
	resetFlags()

	// Create config file with one value
	configBuilder := testutil.NewConfigBuilder().WithHostname("config-file.example.com")
	tempDir := t.TempDir()
	configFile := tempDir + "/test.json"

	err := configBuilder.WriteToFile(configFile)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	// Set environment variable with different value
	os.Setenv("ESXI_HOSTNAME", "env.example.com")
	defer os.Unsetenv("ESXI_HOSTNAME")

	// Command line flag should have highest precedence
	oldArgs := os.Args
	os.Args = []string{
		"test-program",
		"-config", configFile,
		"-hostname", "cmdline.example.com",
		"-aws-key-id", "AKIATEST123",
		"-aws-secret-key", "test-secret",
		"-esxi-user", "root",
		"-esxi-pass", "password",
	}
	defer func() { os.Args = oldArgs }()

	config, err := parseArgs()
	if err != nil {
		t.Fatalf("Expected precedence test to work, got error: %v", err)
	}

	// Command line flag should win
	if config.Hostname != "cmdline.example.com" {
		t.Errorf("Expected command line flag to take precedence, got %s", config.Hostname)
	}
}

func TestParseArgs_InvalidConfigurations(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		shouldFail bool
	}{
		{
			name:       "missing hostname",
			args:       []string{"test-program", "-domain", "example.com", "-email", "test@example.com"},
			shouldFail: true,
		},
		{
			name:       "missing AWS credentials",
			args:       []string{"test-program", "-hostname", "test.example.com", "-domain", "example.com", "-email", "test@example.com"},
			shouldFail: true,
		},
		{
			name:       "dry-run and force together",
			args:       []string{"test-program", "-hostname", "test.example.com", "-aws-key-id", "key", "-aws-secret-key", "secret", "-dry-run", "-force"},
			shouldFail: true,
		},
		{
			name:       "invalid key size",
			args:       []string{"test-program", "-hostname", "test.example.com", "-aws-key-id", "key", "-aws-secret-key", "secret", "-key-size", "1024"},
			shouldFail: true,
		},
		{
			name:       "invalid threshold",
			args:       []string{"test-program", "-hostname", "test.example.com", "-aws-key-id", "key", "-aws-secret-key", "secret", "-threshold", "1.5"},
			shouldFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetFlags()

			oldArgs := os.Args
			os.Args = tt.args
			defer func() { os.Args = oldArgs }()

			_, err := parseArgs()

			if tt.shouldFail && err == nil {
				t.Errorf("Expected %s to fail validation", tt.name)
			}
			if !tt.shouldFail && err != nil {
				t.Errorf("Expected %s to pass validation, got error: %v", tt.name, err)
			}
		})
	}
}

func TestParseArgs_DryRunMode(t *testing.T) {
	resetFlags()

	oldArgs := os.Args
	os.Args = []string{
		"test-program",
		"-hostname", "test.example.com",
		"-aws-key-id", "AKIATEST123",
		"-aws-secret-key", "test-secret",
		"-dry-run",
	}
	defer func() { os.Args = oldArgs }()

	config, err := parseArgs()
	if err != nil {
		t.Fatalf("Expected dry-run configuration to be valid, got error: %v", err)
	}

	if !config.DryRun {
		t.Error("Expected DryRun to be true")
	}

	// In dry-run mode, domain, email, and ESXi credentials are not required
	if config.Domain != "" || config.Email != "" || config.ESXiUsername != "" {
		t.Error("Expected domain, email, and ESXi credentials to be optional in dry-run mode")
	}
}

func TestParseArgs_ForceMode(t *testing.T) {
	resetFlags()

	oldArgs := os.Args
	os.Args = []string{
		"test-program",
		"-hostname", "test.example.com",
		"-domain", "example.com",
		"-email", "test@example.com",
		"-aws-key-id", "AKIATEST123",
		"-aws-secret-key", "test-secret",
		"-esxi-user", "root",
		"-esxi-pass", "password",
		"-force",
	}
	defer func() { os.Args = oldArgs }()

	config, err := parseArgs()
	if err != nil {
		t.Fatalf("Expected force configuration to be valid, got error: %v", err)
	}

	if !config.Force {
		t.Error("Expected Force to be true")
	}
}

func TestParseArgs_CustomThresholdAndKeySize(t *testing.T) {
	resetFlags()

	oldArgs := os.Args
	os.Args = []string{
		"test-program",
		"-hostname", "test.example.com",
		"-domain", "example.com",
		"-email", "test@example.com",
		"-aws-key-id", "AKIATEST123",
		"-aws-secret-key", "test-secret",
		"-esxi-user", "root",
		"-esxi-pass", "password",
		"-threshold", "0.5",
		"-key-size", "2048",
	}
	defer func() { os.Args = oldArgs }()

	config, err := parseArgs()
	if err != nil {
		t.Fatalf("Expected custom threshold and key size to be valid, got error: %v", err)
	}

	if config.Threshold != 0.5 {
		t.Errorf("Expected threshold 0.5, got %f", config.Threshold)
	}
	if config.KeySize != 2048 {
		t.Errorf("Expected key size 2048, got %d", config.KeySize)
	}
}

func TestParseArgs_LoggingOptions(t *testing.T) {
	resetFlags()

	oldArgs := os.Args
	os.Args = []string{
		"test-program",
		"-hostname", "test.example.com",
		"-aws-key-id", "AKIATEST123",
		"-aws-secret-key", "test-secret",
		"-log", "/tmp/test.log",
		"-log-level", "DEBUG",
		"-dry-run",
	}
	defer func() { os.Args = oldArgs }()

	config, err := parseArgs()
	if err != nil {
		t.Fatalf("Expected logging options to be valid, got error: %v", err)
	}

	if config.LogFile != "/tmp/test.log" {
		t.Errorf("Expected log file /tmp/test.log, got %s", config.LogFile)
	}
	if config.LogLevel != "DEBUG" {
		t.Errorf("Expected log level DEBUG, got %s", config.LogLevel)
	}
}

func TestParseArgs_AWSSessionToken(t *testing.T) {
	resetFlags()

	oldArgs := os.Args
	os.Args = []string{
		"test-program",
		"-hostname", "test.example.com",
		"-domain", "example.com",
		"-email", "test@example.com",
		"-aws-key-id", "ASIATEST123",
		"-aws-secret-key", "test-secret",
		"-aws-session-token", "test-session-token",
		"-aws-region", "us-west-2",
		"-esxi-user", "root",
		"-esxi-pass", "password",
	}
	defer func() { os.Args = oldArgs }()

	config, err := parseArgs()
	if err != nil {
		t.Fatalf("Expected AWS session token to be valid, got error: %v", err)
	}

	if config.Route53SessionToken != "test-session-token" {
		t.Errorf("Expected session token test-session-token, got %s", config.Route53SessionToken)
	}
	if config.Route53Region != "us-west-2" {
		t.Errorf("Expected region us-west-2, got %s", config.Route53Region)
	}
}

func TestParseArgs_MalformedConfigFile(t *testing.T) {
	resetFlags()

	// Use the malformed config file from testdata
	malformedConfig := "testdata/configs/malformed.json"

	oldArgs := os.Args
	os.Args = []string{
		"test-program",
		"-config", malformedConfig,
	}
	defer func() { os.Args = oldArgs }()

	_, err := parseArgs()
	if err == nil {
		t.Error("Expected malformed config file to cause an error")
	}

	if !strings.Contains(err.Error(), "failed to load config file") {
		t.Errorf("Expected config file error, got: %v", err)
	}
}

func TestParseArgs_NonexistentConfigFile(t *testing.T) {
	resetFlags()

	oldArgs := os.Args
	os.Args = []string{
		"test-program",
		"-config", "/nonexistent/config.json",
		"-hostname", "test.example.com",
		"-aws-key-id", "AKIATEST123",
		"-aws-secret-key", "test-secret",
		"-dry-run",
	}
	defer func() { os.Args = oldArgs }()

	// Non-existent config files should not cause errors (they're optional)
	config, err := parseArgs()
	if err != nil {
		t.Fatalf("Expected non-existent config file to be ignored, got error: %v", err)
	}

	// Should still work with command line args
	if config.Hostname != "test.example.com" {
		t.Errorf("Expected hostname from command line, got %s", config.Hostname)
	}
}

// resetFlags resets the flag package for testing
func resetFlags() {
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
}

// Mock exit for testing help output - in real tests you might want to capture output
func TestParseArgs_NoArguments(t *testing.T) {
	// This test is tricky because parseArgs calls os.Exit(0) when no args are provided
	// In a real implementation, you might want to refactor to make this more testable
	// or use techniques to capture the exit call
	t.Skip("Skipping test that would call os.Exit - would need refactoring to test properly")
}

func TestParseArgs_VersionFlag(t *testing.T) {
	// Similar to the no arguments test, this calls os.Exit(0)
	t.Skip("Skipping version flag test that would call os.Exit - would need refactoring to test properly")
}
