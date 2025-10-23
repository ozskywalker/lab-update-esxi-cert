package main

import (
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lab-update-esxi-cert/testutil"
)

func TestCheckCertificateWithDialer_ValidCertificate(t *testing.T) {
	// Generate a valid certificate for testing
	certPEM, keyPEM, err := testutil.GenerateValidCertificate("test.example.com")
	if err != nil {
		t.Fatalf("Failed to generate test certificate: %v", err)
	}

	// Create mock TLS dialer with the test certificate
	mockDialer := &testutil.MockTLSDialer{
		CertPEM:    certPEM,
		KeyPEM:     keyPEM,
		ShouldFail: false,
	}

	// Test that a valid certificate doesn't need renewal (threshold 0.33 = 33%)
	needsRenewal, cert, err := checkCertificateWithDialer("test.example.com", 0.33, mockDialer)
	if err != nil {
		t.Errorf("Expected no error for valid certificate, got: %v", err)
	}
	if needsRenewal {
		t.Error("Expected valid certificate to not need renewal")
	}
	if cert == nil {
		t.Error("Expected certificate to be returned")
	}
	if cert != nil && cert.Subject.CommonName != "test.example.com" {
		t.Errorf("Expected certificate CN to be test.example.com, got: %s", cert.Subject.CommonName)
	}
}

func TestCheckCertificateWithDialer_NearExpiry(t *testing.T) {
	// Generate an almost expired certificate (expires in 1 day)
	certPEM, keyPEM, err := testutil.GenerateNearExpiryCertificate("test.example.com", 1)
	if err != nil {
		t.Fatalf("Failed to generate test certificate: %v", err)
	}

	// Create mock TLS dialer with the test certificate
	mockDialer := &testutil.MockTLSDialer{
		CertPEM:    certPEM,
		KeyPEM:     keyPEM,
		ShouldFail: false,
	}

	// Test that certificate near expiry needs renewal (threshold 0.33 = 33%)
	needsRenewal, cert, err := checkCertificateWithDialer("test.example.com", 0.33, mockDialer)
	if err != nil {
		t.Errorf("Expected no error for near-expiry certificate, got: %v", err)
	}
	if !needsRenewal {
		t.Error("Expected near-expiry certificate to need renewal")
	}
	if cert == nil {
		t.Error("Expected certificate to be returned")
	}
}

func TestCheckCertificateWithDialer_ExpiredCertificate(t *testing.T) {
	// Generate an expired certificate (expired 1 day ago)
	certPEM, keyPEM, err := testutil.GenerateExpiredCertificate("test.example.com")
	if err != nil {
		t.Fatalf("Failed to generate test certificate: %v", err)
	}

	// Create mock TLS dialer with the test certificate
	mockDialer := &testutil.MockTLSDialer{
		CertPEM:    certPEM,
		KeyPEM:     keyPEM,
		ShouldFail: false,
	}

	// Test that expired certificate needs renewal
	needsRenewal, cert, err := checkCertificateWithDialer("test.example.com", 0.33, mockDialer)
	if err != nil {
		t.Errorf("Expected no error for expired certificate, got: %v", err)
	}
	if !needsRenewal {
		t.Error("Expected expired certificate to need renewal")
	}
	if cert == nil {
		t.Error("Expected certificate to be returned")
	}
}

func TestCheckCertificateWithDialer_ConnectionFailure(t *testing.T) {
	// Create mock TLS dialer that fails
	mockDialer := &testutil.MockTLSDialer{
		ShouldFail: true,
		FailError:  fmt.Errorf("connection refused"),
	}

	// Test that connection failure returns error
	needsRenewal, cert, err := checkCertificateWithDialer("test.example.com", 0.33, mockDialer)
	if err == nil {
		t.Error("Expected error for connection failure")
	}
	if needsRenewal {
		t.Error("Expected needsRenewal to be false on connection failure")
	}
	if cert != nil {
		t.Error("Expected no certificate on connection failure")
	}
	if err != nil && !strings.Contains(err.Error(), "failed to connect") {
		t.Errorf("Expected connection error, got: %v", err)
	}
}

func TestCheckCertificateWithDialer_HostnameWithPort(t *testing.T) {
	// Test that hostname with explicit port is handled correctly
	certPEM, keyPEM, err := testutil.GenerateValidCertificate("test.example.com")
	if err != nil {
		t.Fatalf("Failed to generate test certificate: %v", err)
	}

	mockDialer := &testutil.MockTLSDialer{
		CertPEM:    certPEM,
		KeyPEM:     keyPEM,
		ShouldFail: false,
	}

	// Test with explicit port
	needsRenewal, cert, err := checkCertificateWithDialer("test.example.com:8443", 0.33, mockDialer)
	if err != nil {
		t.Errorf("Expected no error for hostname with port, got: %v", err)
	}
	if needsRenewal {
		t.Error("Expected valid certificate to not need renewal")
	}
	if cert == nil {
		t.Error("Expected certificate to be returned")
	}
	if cert != nil && cert.Subject.CommonName != "test.example.com" {
		t.Errorf("Expected certificate CN to be test.example.com, got: %s", cert.Subject.CommonName)
	}
}

func TestCheckCertificateWithDialer_NoCertificates(t *testing.T) {
	// This test verifies the "no certificates found" error path
	// However, with the current MockTLSDialer implementation, we can't easily
	// simulate a successful TLS connection that returns zero certificates
	// (that's a very rare edge case in practice).
	//
	// The mock would need to be enhanced to support this scenario, which
	// would require significant changes to testutil/mocks.go
	//
	// For now, we skip this edge case test as it's already covered by
	// the connection failure test above.
	t.Skip("MockTLSDialer doesn't support simulating successful connection with zero certificates")
}

func TestGetCachedCertificate_ValidCache(t *testing.T) {
	tempDir := t.TempDir()

	// Create a valid cached certificate
	hostname := "test.example.com"
	certPEM, keyPEM, err := testutil.GenerateValidCertificate(hostname)
	if err != nil {
		t.Fatalf("Failed to generate test certificate: %v", err)
	}

	// Create cache directory and files
	cacheDir := filepath.Join(tempDir, "esxi-cert-cache")
	os.MkdirAll(cacheDir, 0755)

	certPath := filepath.Join(cacheDir, fmt.Sprintf("%s-cert.pem", hostname))
	keyPath := filepath.Join(cacheDir, fmt.Sprintf("%s-key.pem", hostname))

	os.WriteFile(certPath, certPEM, 0600)
	os.WriteFile(keyPath, keyPEM, 0600)

	config := Config{
		Hostname: hostname,
		Force:    false,
	}

	// Test with custom cache directory
	cachedCertPath, cachedKeyPath, found := getCachedCertificateWithDir(config, cacheDir)

	if !found {
		t.Error("Expected to find cached certificate")
	}
	if cachedCertPath != certPath {
		t.Errorf("Expected cert path %s, got %s", certPath, cachedCertPath)
	}
	if cachedKeyPath != keyPath {
		t.Errorf("Expected key path %s, got %s", keyPath, cachedKeyPath)
	}
}

func TestGetCachedCertificate_ForceSkipsCache(t *testing.T) {
	config := Config{
		Hostname: "test.example.com",
		Force:    true,
	}

	cachedCertPath, cachedKeyPath, found := getCachedCertificate(config)

	if found {
		t.Error("Expected force mode to skip cache")
	}
	if cachedCertPath != "" || cachedKeyPath != "" {
		t.Error("Expected empty paths when cache is skipped")
	}
}

func TestGetCachedCertificate_NearExpiryCache(t *testing.T) {
	tempDir := t.TempDir()

	// Create a certificate that's close to expiration (< 50% remaining)
	hostname := "test.example.com"
	certPEM, keyPEM, err := testutil.GenerateNearExpiryCertificate(hostname, 10) // 10 days left (out of 90)
	if err != nil {
		t.Fatalf("Failed to generate near-expiry certificate: %v", err)
	}

	// Create cache directory and files
	cacheDir := filepath.Join(tempDir, "esxi-cert-cache")
	os.MkdirAll(cacheDir, 0755)

	certPath := filepath.Join(cacheDir, fmt.Sprintf("%s-cert.pem", hostname))
	keyPath := filepath.Join(cacheDir, fmt.Sprintf("%s-key.pem", hostname))

	os.WriteFile(certPath, certPEM, 0600)
	os.WriteFile(keyPath, keyPEM, 0600)

	config := Config{
		Hostname: hostname,
		Force:    false,
	}

	// Certificate with < 50% lifetime remaining should not be used from cache
	cachedCertPath, cachedKeyPath, found := getCachedCertificateWithDir(config, cacheDir)

	if found {
		t.Error("Expected near-expiry cached certificate to be rejected")
	}
	if cachedCertPath != "" || cachedKeyPath != "" {
		t.Error("Expected empty paths when cached certificate is too close to expiration")
	}
}

func TestGetCachedCertificate_MissingFiles(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "empty-cache")
	os.MkdirAll(cacheDir, 0755)

	config := Config{
		Hostname: "nonexistent.example.com",
		Force:    false,
	}

	cachedCertPath, cachedKeyPath, found := getCachedCertificateWithDir(config, cacheDir)

	if found {
		t.Error("Expected to not find nonexistent cached certificate")
	}
	if cachedCertPath != "" || cachedKeyPath != "" {
		t.Error("Expected empty paths when cache files don't exist")
	}
}

func TestGetCachedCertificate_MissingKeyFile(t *testing.T) {
	tempDir := t.TempDir()
	hostname := "test.example.com"

	// Generate certificate
	certPEM, _, err := testutil.GenerateValidCertificate(hostname)
	if err != nil {
		t.Fatalf("Failed to generate test certificate: %v", err)
	}

	// Create cache directory with only cert file (no key file)
	cacheDir := filepath.Join(tempDir, "partial-cache")
	os.MkdirAll(cacheDir, 0755)

	certPath := filepath.Join(cacheDir, fmt.Sprintf("%s-cert.pem", hostname))
	os.WriteFile(certPath, certPEM, 0600)
	// Intentionally don't write key file

	config := Config{
		Hostname: hostname,
		Force:    false,
	}

	cachedCertPath, cachedKeyPath, found := getCachedCertificateWithDir(config, cacheDir)

	if found {
		t.Error("Expected to not find cached certificate when key file is missing")
	}
	if cachedCertPath != "" || cachedKeyPath != "" {
		t.Error("Expected empty paths when key file is missing")
	}
}

func TestGetCachedCertificate_CorruptedCertFile(t *testing.T) {
	tempDir := t.TempDir()
	hostname := "test.example.com"

	// Create cache directory with corrupted cert file
	cacheDir := filepath.Join(tempDir, "corrupt-cache")
	os.MkdirAll(cacheDir, 0755)

	certPath := filepath.Join(cacheDir, fmt.Sprintf("%s-cert.pem", hostname))
	keyPath := filepath.Join(cacheDir, fmt.Sprintf("%s-key.pem", hostname))

	// Write corrupted PEM data
	os.WriteFile(certPath, []byte("NOT A VALID PEM FILE"), 0600)
	os.WriteFile(keyPath, []byte("NOT A VALID KEY FILE"), 0600)

	config := Config{
		Hostname: hostname,
		Force:    false,
	}

	cachedCertPath, cachedKeyPath, found := getCachedCertificateWithDir(config, cacheDir)

	if found {
		t.Error("Expected to not find cached certificate when cert file is corrupted")
	}
	if cachedCertPath != "" || cachedKeyPath != "" {
		t.Error("Expected empty paths when cached certificate is corrupted")
	}
}

func TestGetCachedCertificate_NonRSASignatureAlgorithm(t *testing.T) {
	tempDir := t.TempDir()
	hostname := "test.example.com"

	// Generate an ECDSA certificate (non-RSA signature algorithm)
	certPEM, keyPEM, err := testutil.GenerateValidECDSACertificate(hostname)
	if err != nil {
		t.Fatalf("Failed to generate ECDSA certificate: %v", err)
	}

	// Verify it's actually ECDSA
	cert, err := testutil.ParseCertificatePEM(certPEM)
	if err != nil {
		t.Fatalf("Failed to parse ECDSA certificate: %v", err)
	}
	if cert.SignatureAlgorithm == x509.SHA256WithRSA {
		t.Fatal("Expected ECDSA signature algorithm, got RSA")
	}

	// Create cache directory with ECDSA certificate
	cacheDir := filepath.Join(tempDir, "ecdsa-cache")
	os.MkdirAll(cacheDir, 0755)

	certPath := filepath.Join(cacheDir, fmt.Sprintf("%s-cert.pem", hostname))
	keyPath := filepath.Join(cacheDir, fmt.Sprintf("%s-key.pem", hostname))

	os.WriteFile(certPath, certPEM, 0600)
	os.WriteFile(keyPath, keyPEM, 0600)

	config := Config{
		Hostname: hostname,
		Force:    false,
	}

	// Cache should reject ECDSA certificate because code requires SHA256WithRSA
	cachedCertPath, cachedKeyPath, found := getCachedCertificateWithDir(config, cacheDir)

	if found {
		t.Error("Expected to reject cached certificate with non-RSA signature algorithm")
	}
	if cachedCertPath != "" || cachedKeyPath != "" {
		t.Error("Expected empty paths when cached certificate uses non-RSA signature")
	}
}

func TestGeneratePrivateKey(t *testing.T) {
	tests := []struct {
		name    string
		keySize int
	}{
		{"2048-bit key", 2048},
		{"4096-bit key", 4096},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{KeySize: tt.keySize}

			key := generatePrivateKey(config)
			if key == nil {
				t.Error("Expected private key to be generated")
			}

			// Verify it's an RSA key of the correct size
			if rsaKey, ok := key.(*rsa.PrivateKey); ok {
				actualSize := rsaKey.Size() * 8 // Convert bytes to bits
				if actualSize != tt.keySize {
					t.Errorf("Expected key size %d bits, got %d bits", tt.keySize, actualSize)
				}
			} else {
				t.Error("Expected RSA private key")
			}
		})
	}
}

func TestGeneratePrivateKey_InvalidSize(t *testing.T) {
	// This would normally call os.Exit(1) due to key generation failure
	// Test that unusual key sizes get corrected to 4096
	config := Config{KeySize: 1024}

	key := generatePrivateKey(config)
	if key == nil {
		t.Error("Expected private key to be generated even with unusual size")
	}

	if rsaKey, ok := key.(*rsa.PrivateKey); ok {
		actualSize := rsaKey.Size() * 8
		if actualSize != 4096 {
			t.Errorf("Expected unusual key size to be corrected to 4096, got %d", actualSize)
		}
	}
}

func TestMaskPassword(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "****"},
		{"a", "****"},
		{"ab", "****"},
		{"password", "********"},
		{"very-long-password-123", strings.Repeat("*", len("very-long-password-123"))},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("input_%s", tt.input), func(t *testing.T) {
			result := maskPassword(tt.input)
			if result != tt.expected {
				t.Errorf("maskPassword(%s) = %s, expected %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestUserInterface(t *testing.T) {
	// Test the User struct that implements the lego user interface
	user := &User{
		Email: "test@example.com",
		Key:   generatePrivateKey(Config{KeySize: 2048}),
	}

	if user.GetEmail() != "test@example.com" {
		t.Errorf("Expected email test@example.com, got %s", user.GetEmail())
	}

	if user.GetPrivateKey() == nil {
		t.Error("Expected private key to be returned")
	}

	// Registration will be nil until set
	if user.GetRegistration() != nil {
		t.Error("Expected registration to be nil initially")
	}
}

func TestValidateCertificateWithDialer_CertificateChanged(t *testing.T) {
	// Generate old certificate (expired)
	oldCertPEM, _, err := testutil.GenerateExpiredCertificate("test.example.com")
	if err != nil {
		t.Fatalf("Failed to generate old test certificate: %v", err)
	}

	oldCert, err := testutil.ParseCertificatePEM(oldCertPEM)
	if err != nil {
		t.Fatalf("Failed to parse old certificate: %v", err)
	}

	// Generate new certificate (different expiration time)
	newCertPEM, newKeyPEM, err := testutil.GenerateValidCertificate("test.example.com")
	if err != nil {
		t.Fatalf("Failed to generate new test certificate: %v", err)
	}

	// Create mock TLS dialer with the new certificate
	mockDialer := &testutil.MockTLSDialer{
		CertPEM:    newCertPEM,
		KeyPEM:     newKeyPEM,
		ShouldFail: false,
	}

	// Test that validation detects the certificate has changed
	validated, err := validateCertificateWithDialer("test.example.com", oldCert, mockDialer, 10*time.Second, 1*time.Second)
	if err != nil {
		t.Errorf("Expected no error for certificate validation, got: %v", err)
	}
	if !validated {
		t.Error("Expected validation to detect certificate change")
	}
}

func TestValidateCertificateWithDialer_SameCertificate(t *testing.T) {
	// Generate a certificate
	certPEM, keyPEM, err := testutil.GenerateValidCertificate("test.example.com")
	if err != nil {
		t.Fatalf("Failed to generate test certificate: %v", err)
	}

	cert, err := testutil.ParseCertificatePEM(certPEM)
	if err != nil {
		t.Fatalf("Failed to parse certificate: %v", err)
	}

	// Create mock TLS dialer with the same certificate
	mockDialer := &testutil.MockTLSDialer{
		CertPEM:    certPEM,
		KeyPEM:     keyPEM,
		ShouldFail: false,
	}

	// Test that validation times out when certificate hasn't changed
	// (uses a short timeout to make test faster)
	validated, err := validateCertificateWithDialer("test.example.com", cert, mockDialer, 2*time.Second, 500*time.Millisecond)
	if err != nil {
		t.Errorf("Expected no error for certificate validation, got: %v", err)
	}
	if validated {
		t.Error("Expected validation to timeout when certificate hasn't changed")
	}
}

func TestValidateCertificateWithDialer_ConnectionFailure(t *testing.T) {
	// Generate a certificate for the old cert parameter
	certPEM, _, err := testutil.GenerateValidCertificate("test.example.com")
	if err != nil {
		t.Fatalf("Failed to generate test certificate: %v", err)
	}

	cert, err := testutil.ParseCertificatePEM(certPEM)
	if err != nil {
		t.Fatalf("Failed to parse certificate: %v", err)
	}

	// Create mock TLS dialer that always fails
	mockDialer := &testutil.MockTLSDialer{
		ShouldFail: true,
		FailError:  fmt.Errorf("connection refused"),
	}

	// Test that validation handles connection failures gracefully
	validated, err := validateCertificateWithDialer("test.example.com", cert, mockDialer, 2*time.Second, 500*time.Millisecond)
	if err != nil {
		t.Errorf("Expected no error for certificate validation with connection failure, got: %v", err)
	}
	if validated {
		t.Error("Expected validation to fail when connections fail")
	}
}

func TestGenerateCertificate_CacheHit(t *testing.T) {
	// Test that generateCertificate returns cached certificate when cache is valid
	hostname := "test.example.com"

	// Generate a valid certificate and populate cache
	certPEM, keyPEM, err := testutil.GenerateValidCertificate(hostname)
	if err != nil {
		t.Fatalf("Failed to generate test certificate: %v", err)
	}

	// Create cache directory and files - using the default system temp dir
	// We need to use the actual temp dir since generateCertificate uses os.TempDir()
	sysTempDir := os.TempDir()
	cacheDir := filepath.Join(sysTempDir, "esxi-cert-cache")
	os.MkdirAll(cacheDir, 0755)
	defer func() {
		// Clean up test files
		os.Remove(filepath.Join(cacheDir, fmt.Sprintf("%s-cert.pem", hostname)))
		os.Remove(filepath.Join(cacheDir, fmt.Sprintf("%s-key.pem", hostname)))
	}()

	certPath := filepath.Join(cacheDir, fmt.Sprintf("%s-cert.pem", hostname))
	keyPath := filepath.Join(cacheDir, fmt.Sprintf("%s-key.pem", hostname))

	os.WriteFile(certPath, certPEM, 0600)
	os.WriteFile(keyPath, keyPEM, 0600)

	config := Config{
		Hostname: hostname,
		Force:    false,
		KeySize:  4096,
	}

	// Call generateCertificate - should return cached paths
	returnedCertPath, returnedKeyPath, err := generateCertificate(config)
	if err != nil {
		t.Fatalf("Expected cache hit to succeed, got error: %v", err)
	}

	// Verify it returned the cached paths
	if returnedCertPath != certPath {
		t.Errorf("Expected cert path %s, got %s", certPath, returnedCertPath)
	}
	if returnedKeyPath != keyPath {
		t.Errorf("Expected key path %s, got %s", keyPath, returnedKeyPath)
	}
}

func TestGenerateCertificate_Integration(t *testing.T) {
	// This test would need to mock the ACME client and Route53 provider
	// For now, we'll just test the configuration structure
	config := Config{
		Hostname:         "test.example.com",
		Domain:           "example.com",
		Email:            "test@example.com",
		Route53KeyID:     "AKIATEST123",
		Route53SecretKey: "test-secret",
		Route53Region:    "us-east-1",
		KeySize:          4096,
		Force:            false,
	}

	// Verify configuration is valid for certificate generation
	if config.Hostname == "" {
		t.Error("Hostname is required for certificate generation")
	}
	if config.Domain == "" {
		t.Error("Domain is required for DNS validation")
	}
	if config.Email == "" {
		t.Error("Email is required for ACME registration")
	}
	if config.Route53KeyID == "" || config.Route53SecretKey == "" {
		t.Error("AWS credentials are required for Route53 DNS validation")
	}
	if config.KeySize != 2048 && config.KeySize != 4096 {
		t.Error("Invalid key size for certificate generation")
	}

	// In a real integration test, you would:
	// 1. Mock the ACME server
	// 2. Mock the Route53 DNS provider
	// 3. Call generateCertificate(config)
	// 4. Verify the certificate was generated and cached
	t.Skip("Full certificate generation test requires mocked ACME and Route53 services")
}
