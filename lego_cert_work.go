package main

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge/dns01"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/providers/dns/route53"
	"github.com/go-acme/lego/v4/registration"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
	"golang.org/x/crypto/ssh"
)

// TLSDialer interface for TLS connections (enables testing with custom dialers)
type TLSDialer interface {
	Dial(network, addr string, config *tls.Config) (*tls.Conn, error)
}

// DefaultTLSDialer implements TLSDialer using standard library
type DefaultTLSDialer struct{}

// Dial implements TLSDialer interface
func (d *DefaultTLSDialer) Dial(network, addr string, config *tls.Config) (*tls.Conn, error) {
	return tls.Dial(network, addr, config)
}

// User struct for ACME registration
type User struct {
	Email        string
	Registration *registration.Resource
	Key          crypto.PrivateKey
}

// Helper function to mask passwords in logs
func maskPassword(password string) string {
	if len(password) <= 2 {
		return "****"
	}
	return strings.Repeat("*", len(password))
}

// GetEmail returns the email of the User
func (u *User) GetEmail() string {
	return u.Email
}

// GetRegistration returns the registration resource of the User
func (u *User) GetRegistration() *registration.Resource {
	return u.Registration
}

// GetPrivateKey returns the private key of the User
func (u *User) GetPrivateKey() crypto.PrivateKey {
	return u.Key
}

// Check if certificate needs renewal based on threshold
func checkCertificate(hostname string, threshold float64) (bool, *x509.Certificate) {
	needsRenewal, cert, err := checkCertificateWithDialer(hostname, threshold, &DefaultTLSDialer{})
	if err != nil {
		logError("Failed to check certificate: %v", err)
		os.Exit(1)
	}
	return needsRenewal, cert
}

// Check if certificate needs renewal based on threshold with custom TLS dialer
func checkCertificateWithDialer(hostname string, threshold float64, dialer TLSDialer) (bool, *x509.Certificate, error) {
	logInfo("Checking certificate for %s with threshold %.2f", hostname, threshold)

	// Parse hostname to extract host and port
	host, port, err := net.SplitHostPort(hostname)
	if err != nil {
		// If no port specified, assume hostname only and add default port
		host = hostname
		port = "443"
	}
	
	// Connect to server and get certificate
	conn, err := dialer.Dial("tcp", net.JoinHostPort(host, port), &tls.Config{
		InsecureSkipVerify: true,
	})
	if err != nil {
		return false, nil, fmt.Errorf("failed to connect to %s: %v", hostname, err)
	}
	defer conn.Close()

	// Get the certificate
	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return false, nil, fmt.Errorf("no certificates found for %s", hostname)
	}
	
	cert := certs[0]
	logInfo("Certificate subject: %s", cert.Subject)
	logInfo("Issuer: %s", cert.Issuer)
	logInfo("Valid from: %s", cert.NotBefore.Format(time.RFC3339))
	logInfo("Valid until: %s", cert.NotAfter.Format(time.RFC3339))

	// Calculate the remaining lifetime
	now := time.Now()
	totalLifetime := cert.NotAfter.Sub(cert.NotBefore)
	remainingLifetime := cert.NotAfter.Sub(now)
	percentRemaining := float64(remainingLifetime) / float64(totalLifetime)

	logInfo("Certificate has %.2f%% of its lifetime remaining", percentRemaining*100)

	// Determine if renewal is needed
	needsRenewal := percentRemaining <= threshold
	if needsRenewal {
		logInfo("Certificate should be renewed (%.2f%% <= %.2f%%)", percentRemaining*100, threshold*100)
	} else {
		logInfo("Certificate does not need renewal yet (%.2f%% > %.2f%%)", percentRemaining*100, threshold*100)
	}

	return needsRenewal, cert, nil
}

// Check for cached certificate that's still valid
func getCachedCertificate(config Config) (string, string, bool) {
	// If force is enabled, skip cache completely
	if config.Force {
		logInfo("Force renewal enabled - skipping certificate cache")
		return "", "", false
	}

	cacheDir := filepath.Join(os.TempDir(), "esxi-cert-cache")
	os.MkdirAll(cacheDir, 0755)

	certPath := filepath.Join(cacheDir, fmt.Sprintf("%s-cert.pem", config.Hostname))
	keyPath := filepath.Join(cacheDir, fmt.Sprintf("%s-key.pem", config.Hostname))

	// Check if cached files exist
	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		return "", "", false
	}
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		return "", "", false
	}

	// Read and validate cached certificate
	certData, err := os.ReadFile(certPath)
	if err != nil {
		logWarn("Failed to read cached certificate: %v", err)
		return "", "", false
	}

	// Parse certificate to check expiration
	block, _ := pem.Decode(certData)
	if block == nil {
		logWarn("Failed to decode cached certificate PEM")
		return "", "", false
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		logWarn("Failed to parse cached certificate: %v", err)
		return "", "", false
	}

	// Check if certificate is still valid and has reasonable time left
	now := time.Now()
	timeRemaining := cert.NotAfter.Sub(now)
	totalLifetime := cert.NotAfter.Sub(cert.NotBefore)
	percentRemaining := timeRemaining.Seconds() / totalLifetime.Seconds()

	// Verify cached certificate uses RSA signature algorithm
	logDebug("Cached certificate signature algorithm: %s", cert.SignatureAlgorithm.String())
	if cert.SignatureAlgorithm != x509.SHA256WithRSA {
		logInfo("Cached certificate does not use SHA256WithRSA, regenerating...")
		return "", "", false
	}

	// Use a higher threshold for cached certificates to avoid frequent regeneration
	if percentRemaining > 0.5 { // 50% remaining
		logInfo("Using cached certificate (%.1f%% lifetime remaining) with SHA256WithRSA signature", percentRemaining*100)
		return certPath, keyPath, true
	}

	logInfo("Cached certificate too close to expiration (%.1f%% remaining), will generate new one", percentRemaining*100)
	return "", "", false
}

// Generate a new certificate using go-lego and Let's Encrypt
func generateCertificate(config Config) (string, string, error) {
	// First check for cached certificate
	if certPath, keyPath, found := getCachedCertificate(config); found {
		return certPath, keyPath, nil
	}

	logInfo("No valid cached certificate found, generating new certificate...")
	// Create a user
	user := &User{
		Email: config.Email,
		Key:   generatePrivateKey(config),
	}

	// Initialize ACME client
	legoCfg := lego.NewConfig(user)
	legoCfg.CADirURL = acmeServerProduction
	client, err := lego.NewClient(legoCfg)
	if err != nil {
		return "", "", fmt.Errorf("failed to create ACME client: %v", err)
	}

	// Set up Route53 provider
	provider, err := route53.NewDNSProviderConfig(&route53.Config{
		MaxRetries:         5,
		TTL:                60,
		PropagationTimeout: 2 * time.Minute,
		PollingInterval:    4 * time.Second,
		HostedZoneID:       "", // Auto-detect
		AccessKeyID:        config.Route53KeyID,
		SecretAccessKey:    config.Route53SecretKey,
		SessionToken:       config.Route53SessionToken,
		Region:             config.Route53Region,
	})

	if err != nil {
		return "", "", fmt.Errorf("failed to initialize Route53 provider: %v", err)
	}

	// Set DNS challenge provider
	err = client.Challenge.SetDNS01Provider(provider, dns01.AddRecursiveNameservers([]string{"8.8.8.8:53", "1.1.1.1:53"}))
	if err != nil {
		return "", "", fmt.Errorf("failed to set DNS challenge provider: %v", err)
	}

	// Register user
	reg, err := client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
	if err != nil {
		return "", "", fmt.Errorf("failed to register account: %v", err)
	}
	user.Registration = reg

	// Request certificate with RSA key (ensures RSA signature algorithm)
	domains := []string{config.Hostname}
	request := certificate.ObtainRequest{
		Domains: domains,
		Bundle:  true,
	}

	logInfo("Requesting certificate for hostname: %v using RSA private key", domains)
	certificates, err := client.Certificate.Obtain(request)
	if err != nil {
		return "", "", fmt.Errorf("failed to obtain certificate: %v", err)
	}

	// Verify the certificate uses RSA signature algorithm
	block, _ := pem.Decode(certificates.Certificate)
	if block != nil {
		cert, err := x509.ParseCertificate(block.Bytes)
		if err == nil {
			logDebug("Certificate signature algorithm: %s", cert.SignatureAlgorithm.String())
			if cert.SignatureAlgorithm != x509.SHA256WithRSA {
				logWarn("Warning: Certificate does not use SHA256WithRSA signature algorithm")
			} else {
				logInfo("Confirmed: Certificate uses SHA256WithRSA signature algorithm")
			}
		}
	}

	// Save certificate to cache directory for reuse
	cacheDir := filepath.Join(os.TempDir(), "esxi-cert-cache")
	os.MkdirAll(cacheDir, 0755)

	certPath := filepath.Join(cacheDir, fmt.Sprintf("%s-cert.pem", config.Hostname))
	keyPath := filepath.Join(cacheDir, fmt.Sprintf("%s-key.pem", config.Hostname))

	// Write certificate to cache
	if err := os.WriteFile(certPath, certificates.Certificate, 0600); err != nil {
		return "", "", fmt.Errorf("failed to write cert file: %v", err)
	}

	// Write key to cache
	if err := os.WriteFile(keyPath, certificates.PrivateKey, 0600); err != nil {
		return "", "", fmt.Errorf("failed to write key file: %v", err)
	}

	logInfo("Certificate cached to %s", cacheDir)
	return certPath, keyPath, nil
}

// Generate an RSA private key for certificate generation
func generatePrivateKey(config Config) crypto.PrivateKey {
	logInfo("Generating RSA private key with %d bits (ensures SHA256WithRSA signature algorithm)", config.KeySize)

	// Validate key size
	if config.KeySize != 2048 && config.KeySize != 4096 {
		logWarn("Warning: Unusual key size %d, using 4096 bits", config.KeySize)
		config.KeySize = 4096
	}

	key, err := rsa.GenerateKey(rand.Reader, config.KeySize)
	if err != nil {
		logError("Failed to generate RSA key: %v", err)
		os.Exit(1)
	}

	logInfo("RSA key generated successfully - will result in SHA256WithRSA certificate signature")
	return key
}

// Upload the certificate to the ESXi server using SSH file operations
func uploadCertificate(config Config, certPath, keyPath string) error {
	logInfo("Uploading certificate to ESXi host %s via SSH file operations", config.Hostname)

	// Read certificate and key files
	certData, err := os.ReadFile(certPath)
	if err != nil {
		return fmt.Errorf("failed to read certificate file: %v", err)
	}

	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return fmt.Errorf("failed to read key file: %v", err)
	}

	logDebug("Certificate length: %d bytes, Key length: %d bytes", len(certData), len(keyData))

	// Manage SSH service and perform certificate installation
	return installCertificateViaSSH(config, certData, keyData)
}

// Install certificate via SSH file operations with service management
func installCertificateViaSSH(config Config, certData, keyData []byte) error {
	logInfo("Installing certificate via SSH file operations with SOAP API service management...")

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create ESXi connection URL for SOAP API service management
	esxiURL, err := url.Parse(fmt.Sprintf("https://%s/sdk", config.Hostname))
	if err != nil {
		return fmt.Errorf("failed to parse ESXi URL for service management: %v", err)
	}

	// Set credentials
	esxiURL.User = url.UserPassword(config.ESXiUsername, config.ESXiPassword)

	// Connect to ESXi via SOAP API for service management
	logInfo("Connecting to ESXi SOAP API for SSH service management...")
	client, err := govmomi.NewClient(ctx, esxiURL, true)
	if err != nil {
		return fmt.Errorf("failed to connect to ESXi SOAP API for service management: %v", err)
	}
	defer client.Logout(ctx)

	logInfo("Successfully connected to ESXi SOAP API for service management")

	// Find the host system
	finder := find.NewFinder(client.Client, true)
	var hostSystem *object.HostSystem

	// Try to find host system
	hosts, err := finder.HostSystemList(ctx, "*")
	if err == nil && len(hosts) > 0 {
		hostSystem = hosts[0]
	} else {
		// Try common host references as fallback
		commonHostRefs := []string{"ha-host", "host-1", "host-0"}
		for _, hostRef := range commonHostRefs {
			hostMOR := types.ManagedObjectReference{
				Type:  "HostSystem",
				Value: hostRef,
			}
			testHost := object.NewHostSystem(client.Client, hostMOR)
			if _, err := testHost.ObjectName(ctx); err == nil {
				hostSystem = testHost
				break
			}
		}
	}

	if hostSystem == nil {
		return fmt.Errorf("failed to find ESXi host system for service management")
	}

	// Get the service system for managing SSH service
	serviceSystem, err := hostSystem.ConfigManager().ServiceSystem(ctx)
	if err != nil {
		return fmt.Errorf("failed to get service system: %v", err)
	}

	// Check and start TSM-SSH service if needed
	// sshServiceWasRunning, err := ensureSSHServiceRunning(ctx, serviceSystem)
	_, err = ensureSSHServiceRunning(ctx, serviceSystem)
	if err != nil {
		return fmt.Errorf("failed to manage SSH service: %v", err)
	}

	// Perform SSH certificate installation
	sshErr := performSSHCertificateInstallation(config, certData, keyData)

	// // Stop TSM-SSH service if we started it
	// if !sshServiceWasRunning {
	// 	log.Printf("Stopping TSM-SSH service as it was not originally running...")
	// 	err = stopSSHService(ctx, serviceSystem)
	// 	if err != nil {
	// 		log.Printf("Warning: Failed to stop TSM-SSH service: %v", err)
	// 	} else {
	// 		log.Printf("TSM-SSH service stopped successfully")
	// 	}
	// }

	// Stop TSM-SSH service anyway
	logInfo("Stopping TSM-SSH service...")
	err = stopSSHService(ctx, serviceSystem)
	if err != nil {
		logWarn("Warning: Failed to stop TSM-SSH service: %v", err)
	} else {
		logInfo("TSM-SSH service stopped successfully")
	}

	return sshErr
}

// Perform SSH certificate installation by copying files and restarting services
func performSSHCertificateInstallation(config Config, certData, keyData []byte) error {
	logInfo("Performing SSH certificate installation...")
	logDebug("SSH connection: %s@%s:22", config.ESXiUsername, config.Hostname)
	logDebug("SSH password: %s", maskPassword(config.ESXiPassword))

	// SSH configuration with multiple auth methods
	sshConfig := &ssh.ClientConfig{
		User: config.ESXiUsername,
		Auth: []ssh.AuthMethod{
			ssh.Password(config.ESXiPassword),
			ssh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) ([]string, error) {
				answers := make([]string, len(questions))
				for i := range questions {
					answers[i] = config.ESXiPassword
				}
				return answers, nil
			}),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
		ClientVersion:   "SSH-2.0-ESXi-Cert-Manager",
	}

	// Connect to ESXi host
	client, err := ssh.Dial("tcp", config.Hostname+":22", sshConfig)
	if err != nil {
		return fmt.Errorf("failed to connect via SSH: %v", err)
	}
	defer client.Close()

	logInfo("Connected to ESXi via SSH successfully!")

	// Step 1: Backup existing certificates
	err = backupExistingCertificates(client)
	if err != nil {
		logWarn("Warning: Failed to backup existing certificates: %v", err)
	}

	// Step 2: Copy new certificate and key files
	err = copyCertificateFiles(client, certData, keyData)
	if err != nil {
		return fmt.Errorf("failed to copy certificate files: %v", err)
	}

	// Step 3: Restart ESXi services
	err = restartESXiServicesViaSSH(client)
	if err != nil {
		return fmt.Errorf("failed to restart ESXi services: %v", err)
	}

	logInfo("Certificate installation completed successfully via SSH")
	return nil
}

// Backup existing certificates
func backupExistingCertificates(client *ssh.Client) error {
	logInfo("Backing up existing certificates...")

	commands := []string{
		"cp -f /etc/vmware/ssl/rui.crt /etc/vmware/ssl/rui.crt.backup 2>/dev/null || true",
		"cp -f /etc/vmware/ssl/rui.key /etc/vmware/ssl/rui.key.backup 2>/dev/null || true",
		"ls -la /etc/vmware/ssl/rui.*",
	}

	for _, cmd := range commands {
		session, err := client.NewSession()
		if err != nil {
			return fmt.Errorf("failed to create SSH session for backup: %v", err)
		}

		output, _ := session.CombinedOutput(cmd)
		session.Close()

		if strings.Contains(cmd, "ls -la") {
			logDebug("Certificate directory listing:\n%s", string(output))
		} else {
			logDebug("Backup command '%s' completed", cmd)
		}
	}

	return nil
}

// Copy certificate files to ESXi
func copyCertificateFiles(client *ssh.Client, certData, keyData []byte) error {
	logInfo("Copying new certificate and key files...")

	// Copy certificate file
	err := copyFileViaSSH(client, certData, "/etc/vmware/ssl/rui.crt")
	if err != nil {
		return fmt.Errorf("failed to copy certificate file: %v", err)
	}

	// Copy key file
	err = copyFileViaSSH(client, keyData, "/etc/vmware/ssl/rui.key")
	if err != nil {
		return fmt.Errorf("failed to copy key file: %v", err)
	}

	// Set proper permissions
	commands := []string{
		"chmod 644 /etc/vmware/ssl/rui.crt",
		"chmod 600 /etc/vmware/ssl/rui.key",
		"chown root:root /etc/vmware/ssl/rui.crt /etc/vmware/ssl/rui.key",
	}

	for _, cmd := range commands {
		session, err := client.NewSession()
		if err != nil {
			return fmt.Errorf("failed to create SSH session for permissions: %v", err)
		}

		err = session.Run(cmd)
		session.Close()

		if err != nil {
			logWarn("Warning: Permission command '%s' failed: %v", cmd, err)
		} else {
			logDebug("Permission command '%s' completed successfully", cmd)
		}
	}

	return nil
}

// Copy file content via SSH
func copyFileViaSSH(client *ssh.Client, data []byte, remotePath string) error {
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %v", err)
	}
	defer session.Close()

	// Use cat to write the file content
	session.Stdin = strings.NewReader(string(data))

	cmd := fmt.Sprintf("cat > %s", remotePath)
	err = session.Run(cmd)
	if err != nil {
		return fmt.Errorf("failed to copy file to %s: %v", remotePath, err)
	}

	logDebug("Successfully copied %d bytes to %s", len(data), remotePath)
	return nil
}

// Restart ESXi services via SSH
func restartESXiServicesViaSSH(client *ssh.Client) error {
	logInfo("Restarting ESXi services...")

	// Commands to restart services
	commands := []string{
		"/etc/init.d/hostd restart",
		"/etc/init.d/vpxa restart", // This may fail if not managed by vCenter, that's OK
	}

	success := true
	for _, cmd := range commands {
		logInfo("Executing: %s", cmd)
		session, err := client.NewSession()
		if err != nil {
			logError("Failed to create SSH session: %v", err)
			success = false
			continue
		}

		err = session.Run(cmd)
		session.Close()

		if err != nil {
			logWarn("Command '%s' failed: %v", cmd, err)
			if strings.Contains(cmd, "vpxa") {
				logInfo("vpxa restart failure is expected on standalone ESXi hosts")
			} else {
				success = false
			}
		} else {
			logInfo("Command '%s' completed successfully", cmd)
		}
	}

	if !success {
		return fmt.Errorf("some ESXi service restarts failed")
	}

	logInfo("ESXi services restarted successfully")
	return nil
}

// Get current certificate fingerprint for comparison
// func getCurrentCertificateFingerprint(hostname string) string {
// 	conn, err := tls.Dial("tcp", hostname+":443", &tls.Config{
// 		InsecureSkipVerify: true,
// 	})
// 	if err != nil {
// 		log.Printf("Failed to connect to get certificate fingerprint: %v", err)
// 		return ""
// 	}
// 	defer conn.Close()

// 	certs := conn.ConnectionState().PeerCertificates
// 	if len(certs) == 0 {
// 		return ""
// 	}

// 	// Return SHA-256 fingerprint
// 	fingerprint := strings.ToLower(fmt.Sprintf("%x", certs[0].Signature))
// 	return fingerprint[:16] // First 16 characters for comparison
// }

// Ensure SSH service is running, return true if it was already running
func ensureSSHServiceRunning(ctx context.Context, serviceSystem *object.HostServiceSystem) (bool, error) {
	logInfo("Checking TSM-SSH service status...")

	// Get service info
	services, err := serviceSystem.Service(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get services: %v", err)
	}

	var sshService *types.HostService
	for i := range services {
		if services[i].Key == "TSM-SSH" {
			sshService = &services[i]
			break
		}
	}

	if sshService == nil {
		return false, fmt.Errorf("TSM-SSH service not found")
	}

	logDebug("TSM-SSH service status: running=%t", sshService.Running)

	if sshService.Running {
		logInfo("TSM-SSH service is already running")
		return true, nil
	}

	// Start the SSH service
	logInfo("Starting TSM-SSH service...")
	err = serviceSystem.Start(ctx, "TSM-SSH")
	if err != nil {
		return false, fmt.Errorf("failed to start TSM-SSH service: %v", err)
	}

	// Wait a moment for service to start
	time.Sleep(3 * time.Second)

	logInfo("TSM-SSH service started successfully")
	return false, nil
}

// Stop SSH service
func stopSSHService(ctx context.Context, serviceSystem *object.HostServiceSystem) error {
	err := serviceSystem.Stop(ctx, "TSM-SSH")
	if err != nil {
		return fmt.Errorf("failed to stop TSM-SSH service: %v", err)
	}
	return nil
}

// Validate that the new certificate is installed on the ESXi server
func validateCertificate(hostname string, oldCert *x509.Certificate) bool {
	validated, err := validateCertificateWithDialer(hostname, oldCert, &DefaultTLSDialer{}, maxCheckDuration, defaultCheckInterval)
	if err != nil {
		logWarn("Certificate validation error: %v", err)
		return false
	}
	return validated
}

// Validate that the new certificate is installed on the ESXi server with custom dialer and timeouts
func validateCertificateWithDialer(hostname string, oldCert *x509.Certificate, dialer TLSDialer, maxDuration, checkInterval time.Duration) (bool, error) {
	logInfo("Validating certificate installation on %s", hostname)

	startTime := time.Now()
	deadline := startTime.Add(maxDuration)

	// Parse hostname to extract host and port
	host, port, err := net.SplitHostPort(hostname)
	if err != nil {
		// If no port specified, assume hostname only and add default port
		host = hostname
		port = "443"
	}

	for time.Now().Before(deadline) {
		// Connect to server and get certificate
		conn, err := dialer.Dial("tcp", net.JoinHostPort(host, port), &tls.Config{
			InsecureSkipVerify: true,
		})

		if err != nil {
			logWarn("Failed to connect to %s: %v. Retrying in %s...",
				hostname, err, checkInterval)
			time.Sleep(checkInterval)
			continue
		}

		// Get the new certificate
		certs := conn.ConnectionState().PeerCertificates
		conn.Close()
		
		if len(certs) == 0 {
			logWarn("No certificates found for %s. Retrying in %s...", hostname, checkInterval)
			time.Sleep(checkInterval)
			continue
		}
		
		newCert := certs[0]

		// Check if the certificate has changed
		if !newCert.NotAfter.Equal(oldCert.NotAfter) {
			timeDiff := math.Abs(float64(newCert.NotAfter.Unix() - oldCert.NotAfter.Unix()))

			// If the expiration times differ by more than 1 hour, consider it a new certificate
			if timeDiff > 3600 {
				logInfo("New certificate detected! Old expiry: %s, New expiry: %s",
					oldCert.NotAfter.Format(time.RFC3339),
					newCert.NotAfter.Format(time.RFC3339))
				return true, nil
			}
		}

		logDebug("Certificate not updated yet. Checking again in %s...", checkInterval)
		time.Sleep(checkInterval)
	}

	logWarn("Validation timeout reached after %s", maxDuration)
	return false, nil
}
