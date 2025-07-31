package testutil

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"time"
)

// GenerateTestCertificate creates a self-signed certificate for testing
func GenerateTestCertificate(hostname string, notBefore, notAfter time.Time) (certPEM, keyPEM []byte, err error) {
	// Generate private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization:  []string{"Test Org"},
			Country:       []string{"US"},
			Province:      []string{"Test State"},
			Locality:      []string{"Test City"},
			StreetAddress: []string{"Test Street"},
			PostalCode:    []string{"12345"},
			CommonName:    hostname,
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:              []string{hostname, "localhost"},
	}

	// Create certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, nil, err
	}

	// Encode certificate
	certPEM = pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})

	// Encode private key
	keyPEM = pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	return certPEM, keyPEM, nil
}

// GenerateExpiredCertificate creates a certificate that has already expired
func GenerateExpiredCertificate(hostname string) (certPEM, keyPEM []byte, err error) {
	notBefore := time.Now().Add(-365 * 24 * time.Hour) // 1 year ago
	notAfter := time.Now().Add(-1 * 24 * time.Hour)    // 1 day ago
	return GenerateTestCertificate(hostname, notBefore, notAfter)
}

// GenerateNearExpiryCertificate creates a certificate that expires soon
func GenerateNearExpiryCertificate(hostname string, daysUntilExpiry int) (certPEM, keyPEM []byte, err error) {
	notBefore := time.Now().Add(-80 * 24 * time.Hour) // 80 days ago (typical Let's Encrypt duration is 90 days)
	notAfter := time.Now().Add(time.Duration(daysUntilExpiry) * 24 * time.Hour)
	return GenerateTestCertificate(hostname, notBefore, notAfter)
}

// GenerateValidCertificate creates a certificate that has plenty of time left
func GenerateValidCertificate(hostname string) (certPEM, keyPEM []byte, err error) {
	notBefore := time.Now().Add(-24 * time.Hour) // 1 day ago
	notAfter := time.Now().Add(60 * 24 * time.Hour) // 60 days from now
	return GenerateTestCertificate(hostname, notBefore, notAfter)
}

// StartMockTLSServer starts a TLS server with the given certificate for testing
func StartMockTLSServer(certPEM, keyPEM []byte) (*tls.Config, func(), error) {
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, nil, err
	}

	config := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	// Return config and a no-op cleanup function
	cleanup := func() {}
	
	return config, cleanup, nil
}

// ParseCertificatePEM parses a PEM-encoded certificate for testing
func ParseCertificatePEM(certPEM []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM certificate")
	}
	return x509.ParseCertificate(block.Bytes)
}