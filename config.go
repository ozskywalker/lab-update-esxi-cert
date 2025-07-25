package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ConfigSource represents the source of a configuration value
type ConfigSource string

const (
	ConfigSourceDefault    ConfigSource = "default"
	ConfigSourceConfigFile ConfigSource = "config_file"
	ConfigSourceEnvVar     ConfigSource = "environment"
	ConfigSourceFlag       ConfigSource = "command_line"
)

// ConfigValue holds a configuration value with its source
type ConfigValue struct {
	Value  interface{}
	Source ConfigSource
}

// ConfigManager handles configuration from multiple sources
type ConfigManager struct {
	values map[string]ConfigValue
}

// NewConfigManager creates a new configuration manager
func NewConfigManager() *ConfigManager {
	return &ConfigManager{
		values: make(map[string]ConfigValue),
	}
}

// Set sets a configuration value with its source
func (cm *ConfigManager) Set(key string, value interface{}, source ConfigSource) {
	cm.values[key] = ConfigValue{Value: value, Source: source}
}

// Get gets a configuration value
func (cm *ConfigManager) Get(key string) (interface{}, bool) {
	if val, exists := cm.values[key]; exists {
		return val.Value, true
	}
	return nil, false
}

// GetString gets a string configuration value
func (cm *ConfigManager) GetString(key string) string {
	if val, exists := cm.Get(key); exists {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// GetBool gets a boolean configuration value
func (cm *ConfigManager) GetBool(key string) bool {
	if val, exists := cm.Get(key); exists {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return false
}

// GetFloat64 gets a float64 configuration value
func (cm *ConfigManager) GetFloat64(key string) float64 {
	if val, exists := cm.Get(key); exists {
		if f, ok := val.(float64); ok {
			return f
		}
	}
	return 0.0
}

// GetInt gets an int configuration value
func (cm *ConfigManager) GetInt(key string) int {
	if val, exists := cm.Get(key); exists {
		if i, ok := val.(int); ok {
			return i
		}
	}
	return 0
}

// GetSource gets the source of a configuration value
func (cm *ConfigManager) GetSource(key string) ConfigSource {
	if val, exists := cm.values[key]; exists {
		return val.Source
	}
	return ConfigSourceDefault
}

// LoadDefaults loads default configuration values
func (cm *ConfigManager) LoadDefaults() {
	cm.Set("threshold", defaultThreshold, ConfigSourceDefault)
	cm.Set("key_size", 4096, ConfigSourceDefault)
	cm.Set("log_level", "INFO", ConfigSourceDefault)
	cm.Set("aws_region", "us-east-1", ConfigSourceDefault)
	cm.Set("dry_run", false, ConfigSourceDefault)
	cm.Set("force", false, ConfigSourceDefault)
	cm.Set("check_updates", false, ConfigSourceDefault)
	cm.Set("update_check_owner", "", ConfigSourceDefault)
	cm.Set("update_check_repo", "", ConfigSourceDefault)
}

// LoadEnvironmentVariables loads configuration from environment variables
func (cm *ConfigManager) LoadEnvironmentVariables() {
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
		"check_updates":      "CHECK_UPDATES",
		"update_check_owner": "UPDATE_CHECK_OWNER",
		"update_check_repo":  "UPDATE_CHECK_REPO",
	}

	for configKey, envVar := range envMappings {
		if value := os.Getenv(envVar); value != "" {
			// Type conversion based on the configuration key
			switch configKey {
			case "threshold":
				if f, err := strconv.ParseFloat(value, 64); err == nil {
					cm.Set(configKey, f, ConfigSourceEnvVar)
				}
			case "key_size":
				if i, err := strconv.Atoi(value); err == nil {
					cm.Set(configKey, i, ConfigSourceEnvVar)
				}
			case "dry_run", "force", "check_updates":
				if b, err := strconv.ParseBool(value); err == nil {
					cm.Set(configKey, b, ConfigSourceEnvVar)
				}
			default:
				cm.Set(configKey, value, ConfigSourceEnvVar)
			}
		}
	}
}

// ConfigFile represents the structure of a configuration file
type ConfigFile struct {
	Hostname         string  `json:"hostname,omitempty"`
	Domain           string  `json:"domain,omitempty"`
	Email            string  `json:"email,omitempty"`
	Threshold        float64 `json:"threshold,omitempty"`
	LogFile          string  `json:"log_file,omitempty"`
	LogLevel         string  `json:"log_level,omitempty"`
	AWSKeyID         string  `json:"aws_key_id,omitempty"`
	AWSSecretKey     string  `json:"aws_secret_key,omitempty"`
	AWSSessionToken  string  `json:"aws_session_token,omitempty"`
	AWSRegion        string  `json:"aws_region,omitempty"`
	DryRun           bool    `json:"dry_run,omitempty"`
	Force            bool    `json:"force,omitempty"`
	KeySize          int     `json:"key_size,omitempty"`
	ESXiUsername     string  `json:"esxi_username,omitempty"`
	ESXiPassword     string  `json:"esxi_password,omitempty"`
	CheckUpdates     bool    `json:"check_updates,omitempty"`
	UpdateCheckOwner string  `json:"update_check_owner,omitempty"`
	UpdateCheckRepo  string  `json:"update_check_repo,omitempty"`
}

// LoadConfigFile loads configuration from a JSON file
func (cm *ConfigManager) LoadConfigFile(filePath string) error {
	if filePath == "" {
		return nil // No config file specified
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil // Config file doesn't exist, not an error
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read config file %s: %v", filePath, err)
	}

	var configFile ConfigFile
	if err := json.Unmarshal(data, &configFile); err != nil {
		return fmt.Errorf("failed to parse config file %s: %v", filePath, err)
	}

	// Map config file values to configuration manager
	if configFile.Hostname != "" {
		cm.Set("hostname", configFile.Hostname, ConfigSourceConfigFile)
	}
	if configFile.Domain != "" {
		cm.Set("domain", configFile.Domain, ConfigSourceConfigFile)
	}
	if configFile.Email != "" {
		cm.Set("email", configFile.Email, ConfigSourceConfigFile)
	}
	if configFile.Threshold != 0 {
		cm.Set("threshold", configFile.Threshold, ConfigSourceConfigFile)
	}
	if configFile.LogFile != "" {
		cm.Set("log_file", configFile.LogFile, ConfigSourceConfigFile)
	}
	if configFile.LogLevel != "" {
		cm.Set("log_level", configFile.LogLevel, ConfigSourceConfigFile)
	}
	if configFile.AWSKeyID != "" {
		cm.Set("aws_key_id", configFile.AWSKeyID, ConfigSourceConfigFile)
	}
	if configFile.AWSSecretKey != "" {
		cm.Set("aws_secret_key", configFile.AWSSecretKey, ConfigSourceConfigFile)
	}
	if configFile.AWSSessionToken != "" {
		cm.Set("aws_session_token", configFile.AWSSessionToken, ConfigSourceConfigFile)
	}
	if configFile.AWSRegion != "" {
		cm.Set("aws_region", configFile.AWSRegion, ConfigSourceConfigFile)
	}
	if configFile.KeySize != 0 {
		cm.Set("key_size", configFile.KeySize, ConfigSourceConfigFile)
	}
	if configFile.ESXiUsername != "" {
		cm.Set("esxi_username", configFile.ESXiUsername, ConfigSourceConfigFile)
	}
	if configFile.ESXiPassword != "" {
		cm.Set("esxi_password", configFile.ESXiPassword, ConfigSourceConfigFile)
	}
	if configFile.UpdateCheckOwner != "" {
		cm.Set("update_check_owner", configFile.UpdateCheckOwner, ConfigSourceConfigFile)
	}
	if configFile.UpdateCheckRepo != "" {
		cm.Set("update_check_repo", configFile.UpdateCheckRepo, ConfigSourceConfigFile)
	}

	// Handle boolean values (they could be explicitly set to false)
	cm.Set("dry_run", configFile.DryRun, ConfigSourceConfigFile)
	cm.Set("force", configFile.Force, ConfigSourceConfigFile)
	cm.Set("check_updates", configFile.CheckUpdates, ConfigSourceConfigFile)

	logDebug("Loaded configuration from file: %s", filePath)
	return nil
}

// BuildConfig builds the final Config struct from the configuration manager
func (cm *ConfigManager) BuildConfig() Config {
	config := Config{
		Hostname:            cm.GetString("hostname"),
		Domain:              cm.GetString("domain"),
		Email:               cm.GetString("email"),
		Threshold:           cm.GetFloat64("threshold"),
		LogFile:             cm.GetString("log_file"),
		LogLevel:            cm.GetString("log_level"),
		Route53KeyID:        cm.GetString("aws_key_id"),
		Route53SecretKey:    cm.GetString("aws_secret_key"),
		Route53SessionToken: cm.GetString("aws_session_token"),
		Route53Region:       cm.GetString("aws_region"),
		DryRun:              cm.GetBool("dry_run"),
		Force:               cm.GetBool("force"),
		KeySize:             cm.GetInt("key_size"),
		ESXiUsername:        cm.GetString("esxi_username"),
		ESXiPassword:        cm.GetString("esxi_password"),
		CheckUpdates:        cm.GetBool("check_updates"),
		UpdateCheckOwner:    cm.GetString("update_check_owner"),
		UpdateCheckRepo:     cm.GetString("update_check_repo"),
	}

	// Set default log file if not specified
	if config.LogFile == "" {
		executableName := filepath.Base(os.Args[0])
		config.LogFile = executableName + ".log"
	}

	return config
}

// ValidateConfig validates the final configuration
func (cm *ConfigManager) ValidateConfig(config Config) error {
	// Required fields validation
	if config.Hostname == "" {
		return fmt.Errorf("hostname is required")
	}

	// AWS credentials are required for both dry-run and normal execution
	if config.Route53KeyID == "" || config.Route53SecretKey == "" {
		return fmt.Errorf("AWS credentials for Route53 are required")
	}

	// Validate flag combinations
	if config.DryRun && config.Force {
		return fmt.Errorf("cannot use dry-run and force together")
	}

	// Validate required fields for non-dry-run mode
	if !config.DryRun {
		if config.Domain == "" {
			return fmt.Errorf("domain is required for Route53 DNS validation")
		}
		if config.Email == "" {
			return fmt.Errorf("email is required for ACME registration")
		}
		if config.ESXiUsername == "" || config.ESXiPassword == "" {
			return fmt.Errorf("ESXi username and password are required for certificate upload")
		}
	}

	// Validate key size
	if config.KeySize != 2048 && config.KeySize != 4096 {
		return fmt.Errorf("invalid key size %d, must be 2048 or 4096", config.KeySize)
	}

	// Validate threshold
	if config.Threshold <= 0 || config.Threshold >= 1 {
		return fmt.Errorf("invalid threshold %.2f, must be between 0 and 1", config.Threshold)
	}

	// Validate log level
	validLogLevels := []string{"ERROR", "WARN", "INFO", "DEBUG"}
	isValidLogLevel := false
	upperLogLevel := strings.ToUpper(config.LogLevel)
	for _, level := range validLogLevels {
		if upperLogLevel == level {
			isValidLogLevel = true
			break
		}
	}
	if !isValidLogLevel {
		return fmt.Errorf("invalid log level %s, must be one of: %s", config.LogLevel, strings.Join(validLogLevels, ", "))
	}

	return nil
}

// PrintConfigSources prints the sources of all configuration values (for debugging)
func (cm *ConfigManager) PrintConfigSources() {
	logDebug("Configuration sources:")
	for key, value := range cm.values {
		logDebug("  %s: %v (from %s)", key, value.Value, value.Source)
	}
}
