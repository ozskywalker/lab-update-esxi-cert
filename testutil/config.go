package testutil

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// ConfigBuilder helps build test configurations
type ConfigBuilder struct {
	config map[string]interface{}
}

// NewConfigBuilder creates a new configuration builder with defaults
func NewConfigBuilder() *ConfigBuilder {
	return &ConfigBuilder{
		config: map[string]interface{}{
			"hostname":      "test.example.com",
			"domain":        "example.com",
			"email":         "test@example.com",
			"threshold":     0.33,
			"log_level":     "INFO",
			"aws_region":    "us-east-1",
			"key_size":      4096,
			"dry_run":       false,
			"force":         false,
			"aws_key_id":    "AKIATEST12345",
			"aws_secret_key": "test-secret-key-123",
			"esxi_username": "root",
			"esxi_password": "test-password",
		},
	}
}

// WithHostname sets the hostname
func (cb *ConfigBuilder) WithHostname(hostname string) *ConfigBuilder {
	cb.config["hostname"] = hostname
	return cb
}

// WithDomain sets the domain
func (cb *ConfigBuilder) WithDomain(domain string) *ConfigBuilder {
	cb.config["domain"] = domain
	return cb
}

// WithEmail sets the email
func (cb *ConfigBuilder) WithEmail(email string) *ConfigBuilder {
	cb.config["email"] = email
	return cb
}

// WithThreshold sets the renewal threshold
func (cb *ConfigBuilder) WithThreshold(threshold float64) *ConfigBuilder {
	cb.config["threshold"] = threshold
	return cb
}

// WithLogLevel sets the log level
func (cb *ConfigBuilder) WithLogLevel(level string) *ConfigBuilder {
	cb.config["log_level"] = level
	return cb
}

// WithAWSCredentials sets AWS credentials
func (cb *ConfigBuilder) WithAWSCredentials(keyID, secretKey, sessionToken, region string) *ConfigBuilder {
	cb.config["aws_key_id"] = keyID
	cb.config["aws_secret_key"] = secretKey
	if sessionToken != "" {
		cb.config["aws_session_token"] = sessionToken
	}
	cb.config["aws_region"] = region
	return cb
}

// WithESXiCredentials sets ESXi credentials
func (cb *ConfigBuilder) WithESXiCredentials(username, password string) *ConfigBuilder {
	cb.config["esxi_username"] = username
	cb.config["esxi_password"] = password
	return cb
}

// WithDryRun sets dry run mode
func (cb *ConfigBuilder) WithDryRun(dryRun bool) *ConfigBuilder {
	cb.config["dry_run"] = dryRun
	return cb
}

// WithForce sets force renewal
func (cb *ConfigBuilder) WithForce(force bool) *ConfigBuilder {
	cb.config["force"] = force
	return cb
}

// WithKeySize sets the key size
func (cb *ConfigBuilder) WithKeySize(keySize int) *ConfigBuilder {
	cb.config["key_size"] = keySize
	return cb
}

// Build returns the configuration map
func (cb *ConfigBuilder) Build() map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range cb.config {
		result[k] = v
	}
	return result
}

// WriteToFile writes the configuration to a JSON file
func (cb *ConfigBuilder) WriteToFile(filePath string) error {
	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cb.config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, data, 0644)
}

// SetEnv sets environment variables for testing
func (cb *ConfigBuilder) SetEnv() func() {
	var cleanupFuncs []func()

	envMappings := map[string]string{
		"hostname":           "ESXI_HOSTNAME",
		"domain":             "AWS_ROUTE53_DOMAIN",
		"email":              "EMAIL",
		"threshold":          "CERT_THRESHOLD",
		"log_file":           "LOG_FILE",
		"log_level":          "LOG_LEVEL",
		"aws_key_id":         "AWS_ACCESS_KEY_ID",
		"aws_secret_key":     "AWS_SECRET_ACCESS_KEY",
		"aws_session_token":  "AWS_SESSION_TOKEN",
		"aws_region":         "AWS_REGION",
		"dry_run":            "DRY_RUN",
		"force":              "FORCE_RENEWAL",
		"key_size":           "CERT_KEY_SIZE",
		"esxi_username":      "ESXI_USERNAME",
		"esxi_password":      "ESXI_PASSWORD",
	}

	for configKey, envVar := range envMappings {
		if value, exists := cb.config[configKey]; exists {
			oldValue := os.Getenv(envVar)
			os.Setenv(envVar, toString(value))
			
			cleanupFuncs = append(cleanupFuncs, func() {
				if oldValue == "" {
					os.Unsetenv(envVar)
				} else {
					os.Setenv(envVar, oldValue)
				}
			})
		}
	}

	return func() {
		for _, cleanup := range cleanupFuncs {
			cleanup()
		}
	}
}

// toString converts interface{} to string for environment variables
func toString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case int:
		return string(rune(v))
	case float64:
		return string(rune(int(v)))
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

// CreateInvalidConfigs returns various invalid configurations for testing
func CreateInvalidConfigs() map[string]*ConfigBuilder {
	return map[string]*ConfigBuilder{
		"missing_hostname": NewConfigBuilder().WithHostname(""),
		"missing_aws_key": NewConfigBuilder().WithAWSCredentials("", "secret", "", "us-east-1"),
		"missing_aws_secret": NewConfigBuilder().WithAWSCredentials("key", "", "", "us-east-1"),
		"invalid_threshold_too_low": NewConfigBuilder().WithThreshold(-0.1),
		"invalid_threshold_too_high": NewConfigBuilder().WithThreshold(1.0),
		"invalid_key_size": NewConfigBuilder().WithKeySize(1024),
		"dry_run_and_force": NewConfigBuilder().WithDryRun(true).WithForce(true),
		"missing_domain_non_dry_run": NewConfigBuilder().WithDomain("").WithDryRun(false),
		"missing_email_non_dry_run": NewConfigBuilder().WithEmail("").WithDryRun(false),
		"missing_esxi_creds_non_dry_run": NewConfigBuilder().WithESXiCredentials("", "").WithDryRun(false),
	}
}