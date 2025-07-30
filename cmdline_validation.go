package main

import (
	"flag"
	"fmt"
	"os"

	"lab-update-esxi-cert/internal/version"
)

// Parse command-line arguments and return a Config using structured configuration management
func parseArgs() (Config, error) {
	// Create configuration manager
	cm := NewConfigManager()

	// Load configuration in order of precedence (lowest to highest)
	cm.LoadDefaults()

	// Load from config file if specified
	var configFile string
	flag.StringVar(&configFile, "config", "", "Path to JSON configuration file")

	// Define command-line flags
	var (
		showVersion     = flag.Bool("version", false, "Show version information and exit")
		hostname        = flag.String("hostname", "", "ESXi server hostname")
		domain          = flag.String("domain", "", "DNS domain managed by Route53 (for DNS validation)")
		email           = flag.String("email", "", "Email address for ACME registration")
		threshold       = flag.Float64("threshold", 0, "Renewal threshold (e.g., 0.33 for 1/3 of remaining lifetime)")
		logFile         = flag.String("log", "", "Path to log file (defaults to binary_name.log)")
		logLevel        = flag.String("log-level", "", "Log level (ERROR, WARN, INFO, DEBUG)")
		awsKeyID        = flag.String("aws-key-id", "", "AWS Access Key ID for Route53")
		awsSecretKey    = flag.String("aws-secret-key", "", "AWS Secret Access Key for Route53")
		awsSessionToken = flag.String("aws-session-token", "", "AWS Session Token for Route53 (for temporary credentials)")
		awsRegion       = flag.String("aws-region", "", "AWS Region for Route53")
		dryRun          = flag.Bool("dry-run", false, "Only check certificate without renewing")
		force           = flag.Bool("force", false, "Force certificate renewal regardless of expiration threshold")
		keySize         = flag.Int("key-size", 0, "RSA key size for certificates (2048, 4096)")
		esxiUsername    = flag.String("esxi-user", "", "ESXi server username")
		esxiPassword    = flag.String("esxi-pass", "", "ESXi server password")
	)

	// Parse flags first to get config file path
	flag.Parse()

	// Handle version flag
	if *showVersion {
		v := version.Get()
		fmt.Println(v.Detailed())

		// Check for updates and display if available
		if updateMsg := version.GetUpdateNotification(); updateMsg != "" {
			fmt.Println()
			fmt.Println(updateMsg)
		}

		os.Exit(0)
	}

	// Print help if no arguments provided
	if len(os.Args) <= 1 {
		printHelp()
		os.Exit(0)
	}

	// Load configuration file if specified
	if err := cm.LoadConfigFile(configFile); err != nil {
		return Config{}, fmt.Errorf("failed to load config file: %v", err)
	}

	// Load environment variables
	cm.LoadEnvironmentVariables()

	// Override with command-line flags (highest precedence)
	if *hostname != "" {
		cm.Set("hostname", *hostname, ConfigSourceFlag)
	}
	if *domain != "" {
		cm.Set("domain", *domain, ConfigSourceFlag)
	}
	if *email != "" {
		cm.Set("email", *email, ConfigSourceFlag)
	}
	if *threshold != 0 {
		cm.Set("threshold", *threshold, ConfigSourceFlag)
	}
	if *logFile != "" {
		cm.Set("log_file", *logFile, ConfigSourceFlag)
	}
	if *logLevel != "" {
		cm.Set("log_level", *logLevel, ConfigSourceFlag)
	}
	if *awsKeyID != "" {
		cm.Set("aws_key_id", *awsKeyID, ConfigSourceFlag)
	}
	if *awsSecretKey != "" {
		cm.Set("aws_secret_key", *awsSecretKey, ConfigSourceFlag)
	}
	if *awsSessionToken != "" {
		cm.Set("aws_session_token", *awsSessionToken, ConfigSourceFlag)
	}
	if *awsRegion != "" {
		cm.Set("aws_region", *awsRegion, ConfigSourceFlag)
	}
	if *dryRun {
		cm.Set("dry_run", *dryRun, ConfigSourceFlag)
	}
	if *force {
		cm.Set("force", *force, ConfigSourceFlag)
	}
	if *keySize != 0 {
		cm.Set("key_size", *keySize, ConfigSourceFlag)
	}
	if *esxiUsername != "" {
		cm.Set("esxi_username", *esxiUsername, ConfigSourceFlag)
	}
	if *esxiPassword != "" {
		cm.Set("esxi_password", *esxiPassword, ConfigSourceFlag)
	}

	// Build final configuration
	config := cm.BuildConfig()

	// Validate configuration
	if err := cm.ValidateConfig(config); err != nil {
		return config, err
	}

	// Print configuration sources in debug mode
	if config.LogLevel == "DEBUG" {
		cm.PrintConfigSources()
	}

	return config, nil
}

// Print help and usage examples
func printHelp() {
	v := version.Get()
	fmt.Printf("ESXi Certificate Manager %s\n", v.String())
	fmt.Println("=======================")
	fmt.Println("This tool checks and automatically renews SSL certificates for ESXi servers.")
	fmt.Println("")

	// Check for updates and display if available
	if updateMsg := version.GetUpdateNotification(); updateMsg != "" {
		fmt.Println(updateMsg)
		fmt.Println("")
	}
	fmt.Println("Usage:")
	fmt.Printf("  %s [options]\n", os.Args[0])
	fmt.Println("")
	fmt.Println("Options:")
	flag.PrintDefaults()
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Printf("  # Show version information\n")
	fmt.Printf("  %s --version\n", os.Args[0])
	fmt.Println("")
	fmt.Printf("  # Check certificate only\n")
	fmt.Printf("  %s --hostname esxi01.lab.example.com --dry-run\n", os.Args[0])
	fmt.Println("")
	fmt.Printf("  # Using a configuration file\n")
	fmt.Printf("  %s --config /path/to/config.json\n", os.Args[0])
	fmt.Println("")
	fmt.Printf("  # Check and renew certificate if needed\n")
	fmt.Printf("  %s --hostname esxi01.lab.example.com --domain lab.example.com --email admin@example.com \\\n", os.Args[0])
	fmt.Printf("    --esxi-user root --esxi-pass password --aws-key-id AKIAXXXXXXXX --aws-secret-key xxxxxxxx\n")
	fmt.Println("")
	fmt.Printf("  # With temporary credentials (session token)\n")
	fmt.Printf("  %s --hostname esxi01.lab.example.com --domain lab.example.com --email admin@example.com \\\n", os.Args[0])
	fmt.Printf("    --esxi-user root --esxi-pass password --aws-key-id ASIAXXXXXXXX --aws-secret-key xxxxxxxx \\\n")
	fmt.Printf("    --aws-session-token xxxxxxxx\n")
	fmt.Println("")
	fmt.Printf("  # With custom threshold, log file, and debug logging\n")
	fmt.Printf("  %s --hostname esxi01.lab.example.com --domain lab.example.com --email admin@example.com \\\n", os.Args[0])
	fmt.Printf("    --esxi-user root --esxi-pass password --threshold 0.5 --log /var/log/esxi-cert.log --log-level DEBUG\n")
	fmt.Println("")
	fmt.Printf("  # Force certificate renewal regardless of expiration\n")
	fmt.Printf("  %s --hostname esxi01.lab.example.com --domain lab.example.com --email admin@example.com \\\n", os.Args[0])
	fmt.Printf("    --esxi-user root --esxi-pass password --force\n")
	fmt.Println("")
	fmt.Printf("Configuration File:\n")
	fmt.Printf("  You can use a JSON configuration file to specify options. The file supports all command-line options.\n")
	fmt.Printf("  Environment variables and command-line flags will override config file values.\n")
	fmt.Printf("  Configuration precedence (highest to lowest): command-line flags > environment variables > config file > defaults\n")
	fmt.Println("")
	fmt.Printf("  Example config.json:\n")
	fmt.Printf("  {\n")
	fmt.Printf("    \"hostname\": \"esxi01.lab.example.com\",\n")
	fmt.Printf("    \"domain\": \"lab.example.com\",\n")
	fmt.Printf("    \"email\": \"admin@example.com\",\n")
	fmt.Printf("    \"log_level\": \"INFO\",\n")
	fmt.Printf("    \"threshold\": 0.33,\n")
	fmt.Printf("    \"key_size\": 4096,\n")
	fmt.Printf("    \"check_updates\": true,\n")
	fmt.Printf("    \"update_check_owner\": \"yourusername\",\n")
	fmt.Printf("    \"update_check_repo\": \"lab-update-esxi-cert\"\n")
	fmt.Printf("  }\n")
	fmt.Println("")
	fmt.Printf("Notes: \n1. Certificates are installed by copying files to /etc/vmware/ssl/ via SSH.\n")
	fmt.Printf("2. Complex ESXi passwords with many special characters may cause SSH authentication failures.\n")
	fmt.Printf("3. Use ENV variables for credentials whenever possible to avoid exposing credentials in your terminal's history.\n")
	fmt.Printf("4. Use -force to renew certificates regardless of expiration threshold (bypasses cache).\n")
	fmt.Printf("5. Configuration can be specified via config file, environment variables, or command-line flags.\n")
}
