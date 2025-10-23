package main

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lab-update-esxi-cert/testutil"
)

func TestConfigManager_LoadDefaults(t *testing.T) {
	cm := NewConfigManager()
	cm.LoadDefaults()

	// Test default values
	tests := []struct {
		key      string
		expected interface{}
		source   ConfigSource
	}{
		{"threshold", defaultThreshold, ConfigSourceDefault},
		{"key_size", 4096, ConfigSourceDefault},
		{"log_level", "INFO", ConfigSourceDefault},
		{"aws_region", "us-east-1", ConfigSourceDefault},
		{"dry_run", false, ConfigSourceDefault},
		{"force", false, ConfigSourceDefault},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			value, exists := cm.Get(tt.key)
			if !exists {
				t.Errorf("Expected key %s to exist", tt.key)
				return
			}

			if value != tt.expected {
				t.Errorf("Expected %s = %v, got %v", tt.key, tt.expected, value)
			}

			source := cm.GetSource(tt.key)
			if source != tt.source {
				t.Errorf("Expected source %s, got %s", tt.source, source)
			}
		})
	}
}

func TestConfigManager_LoadEnvironmentVariables(t *testing.T) {
	cm := NewConfigManager()
	cm.LoadDefaults()

	// Set up test environment variables
	testEnvVars := map[string]string{
		"ESXI_HOSTNAME":         "test-env.example.com",
		"AWS_ROUTE53_DOMAIN":    "env.example.com",
		"EMAIL":                 "env@example.com",
		"CERT_THRESHOLD":        "0.5",
		"LOG_LEVEL":             "DEBUG",
		"AWS_ACCESS_KEY_ID":     "AKIAENVTEST123",
		"AWS_SECRET_ACCESS_KEY": "env-secret-key",
		"AWS_SESSION_TOKEN":     "env-session-token",
		"AWS_REGION":            "us-west-2",
		"DRY_RUN":               "true",
		"FORCE_RENEWAL":         "false",
		"CERT_KEY_SIZE":         "2048",
		"ESXI_USERNAME":         "admin",
		"ESXI_PASSWORD":         "env-password",
	}

	// Set environment variables
	oldValues := make(map[string]string)
	for envVar, value := range testEnvVars {
		oldValues[envVar] = os.Getenv(envVar)
		os.Setenv(envVar, value)
	}

	// Cleanup function
	defer func() {
		for envVar, oldValue := range oldValues {
			if oldValue == "" {
				os.Unsetenv(envVar)
			} else {
				os.Setenv(envVar, oldValue)
			}
		}
	}()

	// Load environment variables
	cm.LoadEnvironmentVariables()

	// Test expected values
	tests := []struct {
		key      string
		expected interface{}
		source   ConfigSource
	}{
		{"hostname", "test-env.example.com", ConfigSourceEnvVar},
		{"domain", "env.example.com", ConfigSourceEnvVar},
		{"email", "env@example.com", ConfigSourceEnvVar},
		{"threshold", 0.5, ConfigSourceEnvVar},
		{"log_level", "DEBUG", ConfigSourceEnvVar},
		{"aws_key_id", "AKIAENVTEST123", ConfigSourceEnvVar},
		{"aws_secret_key", "env-secret-key", ConfigSourceEnvVar},
		{"aws_session_token", "env-session-token", ConfigSourceEnvVar},
		{"aws_region", "us-west-2", ConfigSourceEnvVar},
		{"dry_run", true, ConfigSourceEnvVar},
		{"force", false, ConfigSourceEnvVar},
		{"key_size", 2048, ConfigSourceEnvVar},
		{"esxi_username", "admin", ConfigSourceEnvVar},
		{"esxi_password", "env-password", ConfigSourceEnvVar},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			value, exists := cm.Get(tt.key)
			if !exists {
				t.Errorf("Expected key %s to exist", tt.key)
				return
			}

			if value != tt.expected {
				t.Errorf("Expected %s = %v, got %v", tt.key, tt.expected, value)
			}

			source := cm.GetSource(tt.key)
			if source != tt.source {
				t.Errorf("Expected source %s, got %s", tt.source, source)
			}
		})
	}
}

func TestConfigManager_LoadConfigFile(t *testing.T) {
	cm := NewConfigManager()
	cm.LoadDefaults()

	t.Run("valid config file", func(t *testing.T) {
		// Load valid config file
		configPath := filepath.Join("testdata", "configs", "valid.json")
		err := cm.LoadConfigFile(configPath)
		if err != nil {
			t.Fatalf("Failed to load valid config file: %v", err)
		}

		// Check values were loaded
		if hostname := cm.GetString("hostname"); hostname != "esxi01.test.example.com" {
			t.Errorf("Expected hostname from config file, got %s", hostname)
		}

		if source := cm.GetSource("hostname"); source != ConfigSourceConfigFile {
			t.Errorf("Expected source ConfigSourceConfigFile, got %s", source)
		}
	})

	t.Run("nonexistent config file", func(t *testing.T) {
		cm := NewConfigManager()
		err := cm.LoadConfigFile("nonexistent.json")
		if err != nil {
			t.Errorf("Expected no error for nonexistent config file, got %v", err)
		}
	})

	t.Run("malformed config file", func(t *testing.T) {
		cm := NewConfigManager()
		configPath := filepath.Join("testdata", "configs", "malformed.json")
		err := cm.LoadConfigFile(configPath)
		if err == nil {
			t.Error("Expected error for malformed config file")
		}
	})

	t.Run("empty config file path", func(t *testing.T) {
		cm := NewConfigManager()
		err := cm.LoadConfigFile("")
		if err != nil {
			t.Errorf("Expected no error for empty config file path, got %v", err)
		}
	})
}

func TestConfigManager_BuildConfig(t *testing.T) {
	cm := NewConfigManager()
	cm.LoadDefaults()

	// Set some test values
	cm.Set("hostname", "test.example.com", ConfigSourceFlag)
	cm.Set("domain", "example.com", ConfigSourceFlag)
	cm.Set("email", "test@example.com", ConfigSourceFlag)
	cm.Set("aws_key_id", "AKIATEST123", ConfigSourceFlag)
	cm.Set("aws_secret_key", "test-secret", ConfigSourceFlag)
	cm.Set("esxi_username", "root", ConfigSourceFlag)
	cm.Set("esxi_password", "password", ConfigSourceFlag)

	config := cm.BuildConfig()

	// Test that values are correctly mapped
	tests := []struct {
		field    string
		expected interface{}
	}{
		{"Hostname", "test.example.com"},
		{"Domain", "example.com"},
		{"Email", "test@example.com"},
		{"Threshold", defaultThreshold},
		{"LogLevel", "INFO"},
		{"Route53KeyID", "AKIATEST123"},
		{"Route53SecretKey", "test-secret"},
		{"Route53Region", "us-east-1"},
		{"DryRun", false},
		{"Force", false},
		{"KeySize", 4096},
		{"ESXiUsername", "root"},
		{"ESXiPassword", "password"},
	}

	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			// Use reflection or manual checking based on field
			switch tt.field {
			case "Hostname":
				if config.Hostname != tt.expected.(string) {
					t.Errorf("Expected %s = %v, got %v", tt.field, tt.expected, config.Hostname)
				}
			case "Domain":
				if config.Domain != tt.expected.(string) {
					t.Errorf("Expected %s = %v, got %v", tt.field, tt.expected, config.Domain)
				}
			case "Email":
				if config.Email != tt.expected.(string) {
					t.Errorf("Expected %s = %v, got %v", tt.field, tt.expected, config.Email)
				}
			case "Threshold":
				if config.Threshold != tt.expected.(float64) {
					t.Errorf("Expected %s = %v, got %v", tt.field, tt.expected, config.Threshold)
				}
			case "LogLevel":
				if config.LogLevel != tt.expected.(string) {
					t.Errorf("Expected %s = %v, got %v", tt.field, tt.expected, config.LogLevel)
				}
			case "Route53KeyID":
				if config.Route53KeyID != tt.expected.(string) {
					t.Errorf("Expected %s = %v, got %v", tt.field, tt.expected, config.Route53KeyID)
				}
			case "Route53SecretKey":
				if config.Route53SecretKey != tt.expected.(string) {
					t.Errorf("Expected %s = %v, got %v", tt.field, tt.expected, config.Route53SecretKey)
				}
			case "Route53Region":
				if config.Route53Region != tt.expected.(string) {
					t.Errorf("Expected %s = %v, got %v", tt.field, tt.expected, config.Route53Region)
				}
			case "DryRun":
				if config.DryRun != tt.expected.(bool) {
					t.Errorf("Expected %s = %v, got %v", tt.field, tt.expected, config.DryRun)
				}
			case "Force":
				if config.Force != tt.expected.(bool) {
					t.Errorf("Expected %s = %v, got %v", tt.field, tt.expected, config.Force)
				}
			case "KeySize":
				if config.KeySize != tt.expected.(int) {
					t.Errorf("Expected %s = %v, got %v", tt.field, tt.expected, config.KeySize)
				}
			case "ESXiUsername":
				if config.ESXiUsername != tt.expected.(string) {
					t.Errorf("Expected %s = %v, got %v", tt.field, tt.expected, config.ESXiUsername)
				}
			case "ESXiPassword":
				if config.ESXiPassword != tt.expected.(string) {
					t.Errorf("Expected %s = %v, got %v", tt.field, tt.expected, config.ESXiPassword)
				}
			}
		})
	}

	// Test default log file generation
	if config.LogFile == "" {
		t.Error("Expected LogFile to be set to default value")
	}
}

func TestConfigManager_ValidateConfig(t *testing.T) {
	cm := NewConfigManager()

	validConfig := testutil.NewConfigBuilder().Build()
	config := buildConfigFromMap(validConfig)

	t.Run("valid configuration", func(t *testing.T) {
		err := cm.ValidateConfig(config)
		if err != nil {
			t.Errorf("Expected valid configuration to pass validation, got error: %v", err)
		}
	})

	t.Run("invalid configurations", func(t *testing.T) {
		invalidConfigs := testutil.CreateInvalidConfigs()

		for name, builder := range invalidConfigs {
			t.Run(name, func(t *testing.T) {
				configMap := builder.Build()
				config := buildConfigFromMap(configMap)

				err := cm.ValidateConfig(config)
				if err == nil {
					t.Errorf("Expected invalid configuration %s to fail validation", name)
				}
			})
		}
	})
}

func TestConfigManager_ConfigurationPrecedence(t *testing.T) {
	// Test that command-line flags override environment variables which override config files
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "test.json")

	// Create config file
	configData := map[string]interface{}{
		"hostname":  "config-file.example.com",
		"log_level": "ERROR",
		"threshold": 0.25,
	}
	data, _ := json.Marshal(configData)
	os.WriteFile(configFile, data, 0644)

	// Set environment variable
	os.Setenv("ESXI_HOSTNAME", "env-var.example.com")
	os.Setenv("LOG_LEVEL", "WARN")
	defer func() {
		os.Unsetenv("ESXI_HOSTNAME")
		os.Unsetenv("LOG_LEVEL")
	}()

	cm := NewConfigManager()
	cm.LoadDefaults()
	cm.LoadConfigFile(configFile)
	cm.LoadEnvironmentVariables()

	// Override with command-line flag (highest precedence)
	cm.Set("hostname", "command-line.example.com", ConfigSourceFlag)

	// Test precedence
	if hostname := cm.GetString("hostname"); hostname != "command-line.example.com" {
		t.Errorf("Expected command-line value, got %s", hostname)
	}
	if source := cm.GetSource("hostname"); source != ConfigSourceFlag {
		t.Errorf("Expected ConfigSourceFlag, got %s", source)
	}

	if logLevel := cm.GetString("log_level"); logLevel != "WARN" {
		t.Errorf("Expected env var value WARN, got %s", logLevel)
	}
	if source := cm.GetSource("log_level"); source != ConfigSourceEnvVar {
		t.Errorf("Expected ConfigSourceEnvVar, got %s", source)
	}

	if threshold := cm.GetFloat64("threshold"); threshold != 0.25 {
		t.Errorf("Expected config file value 0.25, got %f", threshold)
	}
	if source := cm.GetSource("threshold"); source != ConfigSourceConfigFile {
		t.Errorf("Expected ConfigSourceConfigFile, got %s", source)
	}
}

func TestConfigManager_TypeConversions(t *testing.T) {
	cm := NewConfigManager()

	// Test environment variable type conversions
	testCases := []struct {
		envVar   string
		envValue string
		key      string
		expected interface{}
	}{
		{"CERT_THRESHOLD", "0.75", "threshold", 0.75},
		{"CERT_KEY_SIZE", "2048", "key_size", 2048},
		{"DRY_RUN", "true", "dry_run", true},
		{"FORCE_RENEWAL", "false", "force", false},
	}

	for _, tc := range testCases {
		t.Run(tc.envVar, func(t *testing.T) {
			// Set up environment
			oldValue := os.Getenv(tc.envVar)
			os.Setenv(tc.envVar, tc.envValue)
			defer func() {
				if oldValue == "" {
					os.Unsetenv(tc.envVar)
				} else {
					os.Setenv(tc.envVar, oldValue)
				}
			}()

			// Load environment variables
			cm.LoadEnvironmentVariables()

			value, exists := cm.Get(tc.key)
			if !exists {
				t.Fatalf("Expected key %s to exist", tc.key)
			}

			if value != tc.expected {
				t.Errorf("Expected %s = %v (type %T), got %v (type %T)",
					tc.key, tc.expected, tc.expected, value, value)
			}
		})
	}
}

// Helper function to build a Config struct from a map for testing
func buildConfigFromMap(configMap map[string]interface{}) Config {
	config := Config{}

	if v, ok := configMap["hostname"].(string); ok {
		config.Hostname = v
	}
	if v, ok := configMap["domain"].(string); ok {
		config.Domain = v
	}
	if v, ok := configMap["email"].(string); ok {
		config.Email = v
	}
	if v, ok := configMap["threshold"].(float64); ok {
		config.Threshold = v
	}
	if v, ok := configMap["log_level"].(string); ok {
		config.LogLevel = v
	}
	if v, ok := configMap["aws_key_id"].(string); ok {
		config.Route53KeyID = v
	}
	if v, ok := configMap["aws_secret_key"].(string); ok {
		config.Route53SecretKey = v
	}
	if v, ok := configMap["aws_session_token"].(string); ok {
		config.Route53SessionToken = v
	}
	if v, ok := configMap["aws_region"].(string); ok {
		config.Route53Region = v
	}
	if v, ok := configMap["dry_run"].(bool); ok {
		config.DryRun = v
	}
	if v, ok := configMap["force"].(bool); ok {
		config.Force = v
	}
	if v, ok := configMap["key_size"].(int); ok {
		config.KeySize = v
	}
	if v, ok := configMap["esxi_username"].(string); ok {
		config.ESXiUsername = v
	}
	if v, ok := configMap["esxi_password"].(string); ok {
		config.ESXiPassword = v
	}

	// Set defaults for required fields if not present
	if config.LogLevel == "" {
		config.LogLevel = "INFO"
	}
	if config.Route53Region == "" {
		config.Route53Region = "us-east-1"
	}
	if config.KeySize == 0 {
		config.KeySize = 4096
	}
	if config.Threshold == 0 {
		config.Threshold = defaultThreshold
	}

	return config
}

func TestConfigManager_GettersWithMissingKeys(t *testing.T) {
	cm := NewConfigManager()

	// Test GetString with missing key
	if str := cm.GetString("nonexistent"); str != "" {
		t.Errorf("Expected empty string for missing key, got %s", str)
	}

	// Test GetBool with missing key
	if b := cm.GetBool("nonexistent"); b != false {
		t.Errorf("Expected false for missing key, got %v", b)
	}

	// Test GetFloat64 with missing key
	if f := cm.GetFloat64("nonexistent"); f != 0.0 {
		t.Errorf("Expected 0.0 for missing key, got %f", f)
	}

	// Test GetInt with missing key
	if i := cm.GetInt("nonexistent"); i != 0 {
		t.Errorf("Expected 0 for missing key, got %d", i)
	}

	// Test GetSource with missing key
	if source := cm.GetSource("nonexistent"); source != ConfigSourceDefault {
		t.Errorf("Expected ConfigSourceDefault for missing key, got %s", source)
	}
}

func TestConfigManager_GettersWithWrongType(t *testing.T) {
	cm := NewConfigManager()

	// Set a string value
	cm.Set("test_key", "string_value", ConfigSourceDefault)

	// Try to get it as different types - should return zero values
	if b := cm.GetBool("test_key"); b != false {
		t.Errorf("Expected false when getting string as bool, got %v", b)
	}

	if f := cm.GetFloat64("test_key"); f != 0.0 {
		t.Errorf("Expected 0.0 when getting string as float64, got %f", f)
	}

	if i := cm.GetInt("test_key"); i != 0 {
		t.Errorf("Expected 0 when getting string as int, got %d", i)
	}
}

func TestConfigManager_BuildConfig_DefaultLogFile(t *testing.T) {
	cm := NewConfigManager()
	cm.LoadDefaults()

	// Set required fields but leave log_file empty
	cm.Set("hostname", "test.example.com", ConfigSourceFlag)

	config := cm.BuildConfig()

	// LogFile should be set to default (executable name + .log)
	if config.LogFile == "" {
		t.Error("Expected LogFile to have a default value")
	}

	// Check that it ends with .log
	if !strings.Contains(config.LogFile, ".log") {
		t.Errorf("Expected LogFile to end with .log, got %s", config.LogFile)
	}
}

func TestConfigManager_ValidateConfig_EdgeCases(t *testing.T) {
	cm := NewConfigManager()

	tests := []struct {
		name        string
		modifier    func(*Config)
		shouldError bool
		errorPart   string
	}{
		{
			name: "threshold at lower boundary",
			modifier: func(c *Config) {
				c.Threshold = 0.01 // Just above 0
			},
			shouldError: false,
		},
		{
			name: "threshold at upper boundary",
			modifier: func(c *Config) {
				c.Threshold = 0.99 // Just below 1
			},
			shouldError: false,
		},
		{
			name: "threshold exactly 0",
			modifier: func(c *Config) {
				c.Threshold = 0.0
			},
			shouldError: true,
			errorPart:   "threshold",
		},
		{
			name: "threshold exactly 1",
			modifier: func(c *Config) {
				c.Threshold = 1.0
			},
			shouldError: true,
			errorPart:   "threshold",
		},
		{
			name: "log level case insensitive - lowercase",
			modifier: func(c *Config) {
				c.LogLevel = "debug"
			},
			shouldError: false,
		},
		{
			name: "log level case insensitive - mixed case",
			modifier: func(c *Config) {
				c.LogLevel = "WaRn"
			},
			shouldError: false,
		},
		{
			name: "AWS key ID provided without secret",
			modifier: func(c *Config) {
				c.Route53KeyID = "AKIATEST123"
				c.Route53SecretKey = ""
			},
			shouldError: true,
			errorPart:   "both AWS Access Key ID and Secret Access Key",
		},
		{
			name: "AWS secret provided without key ID",
			modifier: func(c *Config) {
				c.Route53KeyID = ""
				c.Route53SecretKey = "secret"
			},
			shouldError: true,
			errorPart:   "both AWS Access Key ID and Secret Access Key",
		},
		{
			name: "both AWS credentials empty (should use default chain)",
			modifier: func(c *Config) {
				c.Route53KeyID = ""
				c.Route53SecretKey = ""
			},
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Start with a valid config
			config := Config{
				Hostname:         "test.example.com",
				Domain:           "example.com",
				Email:            "test@example.com",
				Threshold:        0.33,
				LogLevel:         "INFO",
				Route53KeyID:     "AKIATEST123",
				Route53SecretKey: "secret",
				Route53Region:    "us-east-1",
				KeySize:          4096,
				ESXiUsername:     "root",
				ESXiPassword:     "password",
			}

			// Apply the modifier
			tt.modifier(&config)

			err := cm.ValidateConfig(config)

			if tt.shouldError && err == nil {
				t.Errorf("Expected validation error for %s", tt.name)
			}
			if !tt.shouldError && err != nil {
				t.Errorf("Expected validation to pass for %s, got error: %v", tt.name, err)
			}
			if tt.shouldError && err != nil && !strings.Contains(err.Error(), tt.errorPart) {
				t.Errorf("Expected error to contain '%s', got: %v", tt.errorPart, err)
			}
		})
	}
}

func TestConfigManager_PrintConfigSources(t *testing.T) {
	// Capture log output
	var buf bytes.Buffer
	originalOutput := log.Writer()
	log.SetOutput(&buf)
	defer func() {
		log.SetOutput(originalOutput)
	}()

	// Set currentLogLevel to DEBUG to capture debug output
	originalLogLevel := currentLogLevel
	currentLogLevel = LOG_DEBUG
	defer func() {
		currentLogLevel = originalLogLevel
	}()

	cm := NewConfigManager()
	cm.Set("test_key1", "value1", ConfigSourceFlag)
	cm.Set("test_key2", 42, ConfigSourceEnvVar)
	cm.Set("test_key3", true, ConfigSourceConfigFile)

	cm.PrintConfigSources()

	output := buf.String()

	// Verify output contains expected information
	if !strings.Contains(output, "Configuration sources:") {
		t.Error("Expected output to contain 'Configuration sources:'")
	}
	if !strings.Contains(output, "test_key1") {
		t.Error("Expected output to contain test_key1")
	}
	if !strings.Contains(output, "command_line") {
		t.Error("Expected output to contain command_line source")
	}
}

func TestConfigManager_LoadConfigFile_JSONEdgeCases(t *testing.T) {
	t.Run("config file with all zero values", func(t *testing.T) {
		cm := NewConfigManager()
		cm.LoadDefaults()

		// Create a config file with explicit zero values
		tempDir := t.TempDir()
		configFile := filepath.Join(tempDir, "zero-values.json")

		configData := map[string]interface{}{
			"threshold": 0.0,   // Should NOT be loaded (zero value)
			"key_size":  0,     // Should NOT be loaded (zero value)
			"hostname":  "",    // Should NOT be loaded (empty string)
			"dry_run":   false, // Should be loaded (explicit boolean)
			"force":     false, // Should be loaded (explicit boolean)
		}

		data, _ := json.Marshal(configData)
		os.WriteFile(configFile, data, 0644)

		err := cm.LoadConfigFile(configFile)
		if err != nil {
			t.Fatalf("Failed to load config file: %v", err)
		}

		// Zero values should not override defaults (except booleans)
		if cm.GetFloat64("threshold") != defaultThreshold {
			t.Errorf("Expected default threshold, got %f", cm.GetFloat64("threshold"))
		}
		if cm.GetInt("key_size") != 4096 {
			t.Errorf("Expected default key size, got %d", cm.GetInt("key_size"))
		}

		// Booleans should be loaded even if false
		if source := cm.GetSource("dry_run"); source != ConfigSourceConfigFile {
			t.Errorf("Expected dry_run to be loaded from config file, got source %s", source)
		}
	})
}
