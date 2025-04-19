package main

import (
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge/dns01"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/providers/dns/route53"
	"github.com/go-acme/lego/v4/registration"
)

// User struct for ACME registration
type User struct {
	Email        string
	Registration *registration.Resource
	Key          crypto.PrivateKey
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
	log.Printf("Checking certificate for %s with threshold %.2f", hostname, threshold)

	// Connect to server and get certificate
	conn, err := tls.Dial("tcp", hostname+":443", &tls.Config{
		InsecureSkipVerify: true,
	})
	if err != nil {
		log.Fatalf("Failed to connect to %s: %v", hostname, err)
	}
	defer conn.Close()

	// Get the certificate
	cert := conn.ConnectionState().PeerCertificates[0]
	log.Printf("Certificate subject: %s", cert.Subject)
	log.Printf("Issuer: %s", cert.Issuer)
	log.Printf("Valid from: %s", cert.NotBefore.Format(time.RFC3339))
	log.Printf("Valid until: %s", cert.NotAfter.Format(time.RFC3339))

	// Calculate the remaining lifetime
	now := time.Now()
	totalLifetime := cert.NotAfter.Sub(cert.NotBefore)
	remainingLifetime := cert.NotAfter.Sub(now)
	percentRemaining := float64(remainingLifetime) / float64(totalLifetime)

	log.Printf("Certificate has %.2f%% of its lifetime remaining", percentRemaining*100)

	// Determine if renewal is needed
	needsRenewal := percentRemaining <= threshold
	if needsRenewal {
		log.Printf("Certificate should be renewed (%.2f%% <= %.2f%%)", percentRemaining*100, threshold*100)
	} else {
		log.Printf("Certificate does not need renewal yet (%.2f%% > %.2f%%)", percentRemaining*100, threshold*100)
	}

	return needsRenewal, cert
}

// Generate a new certificate using go-lego and Let's Encrypt
func generateCertificate(config Config) (string, string, error) {
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

	// Request certificate
	domains := []string{config.Domain}
	request := certificate.ObtainRequest{
		Domains: domains,
		Bundle:  true,
	}

	certificates, err := client.Certificate.Obtain(request)
	if err != nil {
		return "", "", fmt.Errorf("failed to obtain certificate: %v", err)
	}

	// Save certificate to temporary files
	certFile, err := os.CreateTemp("", "cert-*.pem")
	if err != nil {
		return "", "", fmt.Errorf("failed to create temp cert file: %v", err)
	}
	defer certFile.Close()

	keyFile, err := os.CreateTemp("", "key-*.pem")
	if err != nil {
		return "", "", fmt.Errorf("failed to create temp key file: %v", err)
	}
	defer keyFile.Close()

	// Write certificate and key to files
	if _, err := certFile.Write(certificates.Certificate); err != nil {
		return "", "", fmt.Errorf("failed to write cert file: %v", err)
	}

	if _, err := keyFile.Write(certificates.PrivateKey); err != nil {
		return "", "", fmt.Errorf("failed to write key file: %v", err)
	}

	return certFile.Name(), keyFile.Name(), nil
}

// Generate a private key for certificate generation
func generatePrivateKey(config Config) interface{} {
	// This is a placeholder. The go-lego library handles key generation based on options.
	// In a real implementation, we would use crypto packages to generate keys.
	return nil
}

// Upload the certificate to the ESXi server using the REST API
func uploadCertificate(config Config, certPath, keyPath string) error {
	// Read certificate and key files
	certData, err := os.ReadFile(certPath)
	if err != nil {
		return fmt.Errorf("failed to read certificate file: %v", err)
	}

	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return fmt.Errorf("failed to read key file: %v", err)
	}

	// Create ESXi client
	client := &ESXiClient{
		BaseURL:  fmt.Sprintf("https://%s/api", config.Hostname),
		Username: config.ESXiUsername,
		Password: config.ESXiPassword,
		HTTPClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		},
	}

	// Prepare certificate data for upload
	certPEM, _ := pem.Decode(certData)
	if certPEM == nil {
		return fmt.Errorf("failed to decode certificate PEM")
	}

	keyPEM, _ := pem.Decode(keyData)
	if keyPEM == nil {
		return fmt.Errorf("failed to decode key PEM")
	}

	// Format certificate as required by ESXi API
	certB64 := base64.StdEncoding.EncodeToString(certPEM.Bytes)
	keyB64 := base64.StdEncoding.EncodeToString(keyPEM.Bytes)

	// Upload certificate via ESXi REST API
	// Note: The actual API endpoint and payload format may vary based on ESXi version
	// This is a simplified example that needs to be adjusted for your specific ESXi version
	payload := map[string]interface{}{
		"cert_base64": certB64,
		"key_base64":  keyB64,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal certificate payload: %v", err)
	}

	// Create HTTP request for certificate upload
	req, err := http.NewRequest("PUT", fmt.Sprintf("%s/host/certificate", client.BaseURL), strings.NewReader(string(payloadBytes)))
	if err != nil {
		return fmt.Errorf("failed to create certificate upload request: %v", err)
	}

	req.SetBasicAuth(client.Username, client.Password)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Send the request
	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upload certificate: %v", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("certificate upload failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Clean up temporary files
	os.Remove(certPath)
	os.Remove(keyPath)

	return nil
}

// Validate that the new certificate is installed on the ESXi server
func validateCertificate(hostname string, oldCert *x509.Certificate) bool {
	log.Printf("Validating certificate installation on %s", hostname)

	startTime := time.Now()
	deadline := startTime.Add(maxCheckDuration)

	for time.Now().Before(deadline) {
		// Connect to server and get certificate
		conn, err := tls.Dial("tcp", hostname+":443", &tls.Config{
			InsecureSkipVerify: true,
		})

		if err != nil {
			log.Printf("Failed to connect to %s: %v. Retrying in %s...",
				hostname, err, defaultCheckInterval)
			time.Sleep(defaultCheckInterval)
			continue
		}

		// Get the new certificate
		newCert := conn.ConnectionState().PeerCertificates[0]
		conn.Close()

		// Check if the certificate has changed
		if !newCert.NotAfter.Equal(oldCert.NotAfter) {
			timeDiff := math.Abs(float64(newCert.NotAfter.Unix() - oldCert.NotAfter.Unix()))

			// If the expiration times differ by more than 1 hour, consider it a new certificate
			if timeDiff > 3600 {
				log.Printf("New certificate detected! Old expiry: %s, New expiry: %s",
					oldCert.NotAfter.Format(time.RFC3339),
					newCert.NotAfter.Format(time.RFC3339))
				return true
			}
		}

		log.Printf("Certificate not updated yet. Checking again in %s...", defaultCheckInterval)
		time.Sleep(defaultCheckInterval)
	}

	log.Printf("Validation timeout reached after %s", maxCheckDuration)
	return false
}
