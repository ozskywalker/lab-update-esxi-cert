package testutil

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"golang.org/x/crypto/ssh"
)

// MockSTSClient implements a mock STS client for testing AWS credential validation
type MockSTSClient struct {
	ShouldFail bool
	Identity   *sts.GetCallerIdentityOutput
}

// GetCallerIdentity mocks the STS GetCallerIdentity call
func (m *MockSTSClient) GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	if m.ShouldFail {
		return nil, fmt.Errorf("the security token included in the request is invalid")
	}

	if m.Identity != nil {
		return m.Identity, nil
	}

	// Default successful response
	return &sts.GetCallerIdentityOutput{
		Account: aws.String("123456789012"),
		Arn:     aws.String("arn:aws:iam::123456789012:user/test-user"),
		UserId:  aws.String("AIDACKCEVSQ6C2EXAMPLE"),
	}, nil
}

// MockTLSServer creates a mock TLS server for certificate testing
type MockTLSServer struct {
	listener net.Listener
	CertPEM  []byte
	KeyPEM   []byte
}

// NewMockTLSServer creates a new mock TLS server with the given certificate
func NewMockTLSServer(certPEM, keyPEM []byte) (*MockTLSServer, error) {
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}

	// Create a listener on a random port
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, err
	}

	// Create TLS config with the certificate
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	// Create TLS listener
	tlsListener := tls.NewListener(listener, tlsConfig)

	// Create HTTP server
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Mock ESXi Server"))
		}),
	}

	mockServer := &MockTLSServer{
		listener: tlsListener,
		CertPEM:  certPEM,
		KeyPEM:   keyPEM,
	}

	// Start server in goroutine
	go func() {
		server.Serve(tlsListener)
	}()

	return mockServer, nil
}

// GetHostPort returns the host:port for connections
func (m *MockTLSServer) GetHostPort() string {
	return m.listener.Addr().String()
}

// Close stops the mock server
func (m *MockTLSServer) Close() {
	if m.listener != nil {
		m.listener.Close()
	}
}

// MockSSHServer provides a mock SSH server for testing certificate uploads
type MockSSHServer struct {
	listener     net.Listener
	hostKey      ssh.Signer
	Commands     []string
	Files        map[string][]byte
	ShouldFail   bool
	FailCommands []string
}

// NewMockSSHServer creates a new mock SSH server
func NewMockSSHServer() (*MockSSHServer, error) {
	// Generate host key
	hostKey, err := generateSSHHostKey()
	if err != nil {
		return nil, err
	}

	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, err
	}

	mock := &MockSSHServer{
		listener: listener,
		hostKey:  hostKey,
		Commands: make([]string, 0),
		Files:    make(map[string][]byte),
	}

	// Configure SSH server
	config := &ssh.ServerConfig{
		PasswordCallback: func(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			// Accept any password for testing
			return nil, nil
		},
		KeyboardInteractiveCallback: func(conn ssh.ConnMetadata, challenge ssh.KeyboardInteractiveChallenge) (*ssh.Permissions, error) {
			// Accept any interactive auth for testing
			return nil, nil
		},
	}
	config.AddHostKey(hostKey)

	// Start accepting connections
	go mock.acceptConnections(config)

	return mock, nil
}

// GetHostPort returns the host:port for SSH connections
func (m *MockSSHServer) GetHostPort() string {
	return m.listener.Addr().String()
}

// Close stops the mock SSH server
func (m *MockSSHServer) Close() {
	if m.listener != nil {
		m.listener.Close()
	}
}

// acceptConnections handles incoming SSH connections
func (m *MockSSHServer) acceptConnections(config *ssh.ServerConfig) {
	for {
		conn, err := m.listener.Accept()
		if err != nil {
			return // Server closed
		}

		go m.handleConnection(conn, config)
	}
}

// handleConnection handles a single SSH connection
func (m *MockSSHServer) handleConnection(conn net.Conn, config *ssh.ServerConfig) {
	defer conn.Close()

	sshConn, chans, reqs, err := ssh.NewServerConn(conn, config)
	if err != nil {
		return
	}
	defer sshConn.Close()

	// Handle global requests
	go ssh.DiscardRequests(reqs)

	// Handle channels
	for newChannel := range chans {
		if newChannel.ChannelType() != "session" {
			newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}

		channel, requests, err := newChannel.Accept()
		if err != nil {
			continue
		}

		go m.handleSession(channel, requests)
	}
}

// handleSession handles SSH session requests
func (m *MockSSHServer) handleSession(channel ssh.Channel, requests <-chan *ssh.Request) {
	defer channel.Close()

	for req := range requests {
		switch req.Type {
		case "exec":
			command := string(req.Payload[4:]) // Skip the length prefix
			m.Commands = append(m.Commands, command)

			// Check if this command should fail
			shouldFail := m.ShouldFail
			for _, failCmd := range m.FailCommands {
				if strings.Contains(command, failCmd) {
					shouldFail = true
					break
				}
			}

			if shouldFail {
				req.Reply(false, nil)
				channel.SendRequest("exit-status", false, []byte{0, 0, 0, 1}) // Exit code 1
			} else {
				req.Reply(true, nil)
				m.handleCommand(channel, command)
				channel.SendRequest("exit-status", false, []byte{0, 0, 0, 0}) // Exit code 0
			}
			return

		default:
			req.Reply(false, nil)
		}
	}
}

// handleCommand processes mock SSH commands
func (m *MockSSHServer) handleCommand(channel ssh.Channel, command string) {
	if strings.HasPrefix(command, "cat >") {
		// Handle file writes - in a real implementation we'd read from stdin
		// For testing, we'll just acknowledge the command
		return
	}

	if strings.HasPrefix(command, "ls -la") {
		// Mock ls output
		output := "-rw-r--r-- 1 root root 1234 Jan 01 12:00 rui.crt\n-rw------- 1 root root 1679 Jan 01 12:00 rui.key\n"
		channel.Write([]byte(output))
		return
	}

	// For other commands, just acknowledge
}

// generateSSHHostKey generates an SSH host key for the mock server
func generateSSHHostKey() (ssh.Signer, error) {
	// For simplicity, we'll just return an error if we can't generate a key
	// In a real implementation, you'd generate an actual key
	return nil, fmt.Errorf("SSH host key generation not implemented in mock")
}

// MockACMEServer provides a mock ACME server for testing certificate generation
type MockACMEServer struct {
	server *httptest.Server
}

// NewMockACMEServer creates a new mock ACME server
func NewMockACMEServer() *MockACMEServer {
	mux := http.NewServeMux()

	// Mock ACME directory endpoint
	mux.HandleFunc("/directory", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// In a real implementation, you'd marshal the directory
		w.Write([]byte(`{"newAccount":"/acme/new-account","newOrder":"/acme/new-order"}`))
	})

	// Mock other ACME endpoints as needed
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	server := httptest.NewServer(mux)

	return &MockACMEServer{
		server: server,
	}
}

// GetURL returns the mock ACME server URL
func (m *MockACMEServer) GetURL() string {
	return m.server.URL
}

// Close stops the mock ACME server
func (m *MockACMEServer) Close() {
	m.server.Close()
}

// MockTLSDialer implements the TLSDialer interface for testing
type MockTLSDialer struct {
	CertPEM    []byte
	KeyPEM     []byte
	ShouldFail bool
	FailError  error
}

// Dial implements TLSDialer interface with mock behavior
func (m *MockTLSDialer) Dial(network, addr string, config *tls.Config) (*tls.Conn, error) {
	if m.ShouldFail {
		if m.FailError != nil {
			return nil, m.FailError
		}
		return nil, fmt.Errorf("mock TLS dial failure")
	}

	// Create a mock TLS connection with our test certificate
	serverConn, clientConn := net.Pipe()

	// Set up the server side with our test certificate
	go func() {
		defer serverConn.Close()
		if m.CertPEM != nil && m.KeyPEM != nil {
			cert, err := tls.X509KeyPair(m.CertPEM, m.KeyPEM)
			if err != nil {
				return
			}

			tlsConfig := &tls.Config{
				Certificates: []tls.Certificate{cert},
			}

			tlsConn := tls.Server(serverConn, tlsConfig)
			defer tlsConn.Close()

			// Perform handshake
			err = tlsConn.Handshake()
			if err != nil {
				return
			}

			// Keep connection open for a short time
			time.Sleep(10 * time.Millisecond)
		}
	}()

	// Return client side as TLS connection
	tlsConn := tls.Client(clientConn, config)

	// Perform handshake with timeout for faster tests
	err := tlsConn.Handshake()
	if err != nil {
		clientConn.Close()
		return nil, err
	}

	return tlsConn, nil
}

// WaitForPort waits for a port to become available (helper for testing)
func WaitForPort(address string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", address, time.Second)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("port %s not available after %v", address, timeout)
}
