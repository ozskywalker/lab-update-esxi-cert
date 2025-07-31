package integration

import (
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// MockSSHServer provides a more complete SSH server implementation for integration testing
type MockSSHServer struct {
	listener     net.Listener
	hostKey      ssh.Signer
	commands     []string
	files        map[string][]byte
	shouldFail   bool
	failCommands []string
	users        map[string]string // username -> password
}

// NewMockSSHServer creates a new mock SSH server for ESXi integration testing
func NewMockSSHServer() (*MockSSHServer, error) {
	// Generate host key for SSH server
	hostKey, err := generateEd25519HostKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate host key: %v", err)
	}

	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, fmt.Errorf("failed to create listener: %v", err)
	}

	server := &MockSSHServer{
		listener: listener,
		hostKey:  hostKey,
		commands: make([]string, 0),
		files:    make(map[string][]byte),
		users:    make(map[string]string),
	}

	// Add default ESXi user
	server.users["root"] = "password"

	// Start accepting connections
	go server.serve()

	return server, nil
}

// AddUser adds a user/password combination for authentication
func (s *MockSSHServer) AddUser(username, password string) {
	s.users[username] = password
}

// SetFailCommands sets commands that should fail
func (s *MockSSHServer) SetFailCommands(commands []string) {
	s.failCommands = commands
}

// GetAddress returns the server address
func (s *MockSSHServer) GetAddress() string {
	return s.listener.Addr().String()
}

// Close stops the SSH server
func (s *MockSSHServer) Close() error {
	return s.listener.Close()
}

// GetExecutedCommands returns all commands that were executed
func (s *MockSSHServer) GetExecutedCommands() []string {
	return s.commands
}

// GetUploadedFiles returns all files that were uploaded
func (s *MockSSHServer) GetUploadedFiles() map[string][]byte {
	return s.files
}

// serve handles incoming SSH connections
func (s *MockSSHServer) serve() {
	config := &ssh.ServerConfig{
		PasswordCallback: func(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			expectedPassword, exists := s.users[conn.User()]
			if !exists {
				return nil, fmt.Errorf("user %s not found", conn.User())
			}
			if string(password) != expectedPassword {
				return nil, fmt.Errorf("invalid password for user %s", conn.User())
			}
			return nil, nil
		},
		KeyboardInteractiveCallback: func(conn ssh.ConnMetadata, challenge ssh.KeyboardInteractiveChallenge) (*ssh.Permissions, error) {
			// Accept any keyboard interactive auth for testing
			return nil, nil
		},
	}
	config.AddHostKey(s.hostKey)

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return // Server closed
		}

		go s.handleConnection(conn, config)
	}
}

// handleConnection handles a single SSH connection
func (s *MockSSHServer) handleConnection(conn net.Conn, config *ssh.ServerConfig) {
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

		go s.handleSession(channel, requests)
	}
}

// handleSession handles SSH session requests
func (s *MockSSHServer) handleSession(channel ssh.Channel, requests <-chan *ssh.Request) {
	defer channel.Close()

	for req := range requests {
		switch req.Type {
		case "exec":
			// Extract command from payload
			if len(req.Payload) < 4 {
				req.Reply(false, nil)
				continue
			}

			commandLen := int(req.Payload[3])
			if len(req.Payload) < 4+commandLen {
				req.Reply(false, nil)
				continue
			}

			command := string(req.Payload[4 : 4+commandLen])
			s.commands = append(s.commands, command)

			// Check if this command should fail
			shouldFail := s.shouldFail
			for _, failCmd := range s.failCommands {
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
				s.executeCommand(channel, command)
				channel.SendRequest("exit-status", false, []byte{0, 0, 0, 0}) // Exit code 0
			}
			return

		case "shell":
			req.Reply(true, nil)
			// Handle interactive shell - not needed for our tests
			return

		default:
			req.Reply(false, nil)
		}
	}
}

// executeCommand simulates command execution
func (s *MockSSHServer) executeCommand(channel ssh.Channel, command string) {
	switch {
	case strings.HasPrefix(command, "cat >"):
		// Handle file upload
		s.handleFileUpload(channel, command)

	case strings.HasPrefix(command, "ls -la"):
		// Mock directory listing
		output := "-rw-r--r-- 1 root root 1234 Jan 01 12:00 rui.crt\n-rw------- 1 root root 1679 Jan 01 12:00 rui.key\n"
		channel.Write([]byte(output))

	case strings.Contains(command, "cp -f"):
		// Mock file copy (backup)
		// No output needed

	case strings.Contains(command, "chmod"):
		// Mock permission change
		// No output needed

	case strings.Contains(command, "chown"):
		// Mock ownership change
		// No output needed

	case strings.Contains(command, "/etc/init.d/hostd restart"):
		// Mock hostd service restart
		channel.Write([]byte("Restarting hostd: [  OK  ]\n"))

	case strings.Contains(command, "/etc/init.d/vpxa restart"):
		// Mock vpxa service restart (may fail on standalone hosts)
		if strings.Contains(command, "vpxa") {
			// Simulate occasional failure for vpxa on standalone hosts
			channel.Write([]byte("vpxa: not running\n"))
		}

	default:
		// Default success for unknown commands
		channel.Write([]byte("OK\n"))
	}
}

// handleFileUpload simulates file upload via cat
func (s *MockSSHServer) handleFileUpload(channel ssh.Channel, command string) {
	// Extract filename from "cat > /path/to/file"
	parts := strings.Fields(command)
	if len(parts) != 3 || parts[1] != ">" {
		return
	}
	filename := parts[2]

	// Read data from stdin (in a real implementation)
	// For testing, we'll just store that the file was "uploaded"
	s.files[filename] = []byte("mock file content")
}

// TestSSHConnection tests basic SSH connectivity to mock ESXi server
func TestSSHConnection(t *testing.T) {
	// Start mock SSH server
	server, err := NewMockSSHServer()
	if err != nil {
		t.Fatalf("Failed to start mock SSH server: %v", err)
	}
	defer server.Close()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Configure SSH client
	config := &ssh.ClientConfig{
		User: "root",
		Auth: []ssh.AuthMethod{
			ssh.Password("password"),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}

	// Connect to mock server
	client, err := ssh.Dial("tcp", server.GetAddress(), config)
	if err != nil {
		t.Fatalf("Failed to connect to mock SSH server: %v", err)
	}
	defer client.Close()

	// Test executing a simple command
	session, err := client.NewSession()
	if err != nil {
		t.Fatalf("Failed to create SSH session: %v", err)
	}
	defer session.Close()

	// Execute a test command
	output, err := session.CombinedOutput("ls -la /etc/vmware/ssl/")
	if err != nil {
		t.Fatalf("Failed to execute command: %v", err)
	}

	// Verify output
	if !strings.Contains(string(output), "rui.crt") {
		t.Errorf("Expected output to contain rui.crt, got: %s", string(output))
	}

	// Verify command was recorded
	commands := server.GetExecutedCommands()
	if len(commands) != 1 {
		t.Errorf("Expected 1 command to be executed, got %d", len(commands))
	}
	if !strings.Contains(commands[0], "ls -la") {
		t.Errorf("Expected ls command, got: %s", commands[0])
	}
}

// TestSSHFileUpload tests file upload simulation
func TestSSHFileUpload(t *testing.T) {
	server, err := NewMockSSHServer()
	if err != nil {
		t.Fatalf("Failed to start mock SSH server: %v", err)
	}
	defer server.Close()

	time.Sleep(100 * time.Millisecond)

	config := &ssh.ClientConfig{
		User: "root",
		Auth: []ssh.AuthMethod{
			ssh.Password("password"),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}

	client, err := ssh.Dial("tcp", server.GetAddress(), config)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	// Test file upload simulation
	session, err := client.NewSession()
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	defer session.Close()

	// Simulate uploading a certificate file
	testContent := "test certificate content"
	session.Stdin = strings.NewReader(testContent)

	err = session.Run("cat > /etc/vmware/ssl/rui.crt")
	if err != nil {
		t.Fatalf("Failed to upload file: %v", err)
	}

	// Verify file was "uploaded"
	files := server.GetUploadedFiles()
	if content, exists := files["/etc/vmware/ssl/rui.crt"]; !exists {
		t.Error("Expected certificate file to be uploaded")
	} else if string(content) != "mock file content" {
		t.Errorf("Expected mock content, got: %s", string(content))
	}
}

// TestSSHServiceManagement tests ESXi service restart commands
func TestSSHServiceManagement(t *testing.T) {
	server, err := NewMockSSHServer()
	if err != nil {
		t.Fatalf("Failed to start mock SSH server: %v", err)
	}
	defer server.Close()

	time.Sleep(100 * time.Millisecond)

	config := &ssh.ClientConfig{
		User: "root",
		Auth: []ssh.AuthMethod{
			ssh.Password("password"),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}

	client, err := ssh.Dial("tcp", server.GetAddress(), config)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	// Test hostd service restart
	session, err := client.NewSession()
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	output, err := session.CombinedOutput("/etc/init.d/hostd restart")
	session.Close()
	if err != nil {
		t.Fatalf("Failed to restart hostd: %v", err)
	}

	if !strings.Contains(string(output), "OK") {
		t.Errorf("Expected successful hostd restart, got: %s", string(output))
	}

	// Test vpxa service restart (may fail)
	session, err = client.NewSession()
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	_, err = session.CombinedOutput("/etc/init.d/vpxa restart")
	session.Close()
	// vpxa restart may fail on standalone hosts - that's expected

	// Verify commands were executed
	commands := server.GetExecutedCommands()
	hostdFound := false
	vpxaFound := false

	for _, cmd := range commands {
		if strings.Contains(cmd, "hostd restart") {
			hostdFound = true
		}
		if strings.Contains(cmd, "vpxa restart") {
			vpxaFound = true
		}
	}

	if !hostdFound {
		t.Error("Expected hostd restart command to be executed")
	}
	if !vpxaFound {
		t.Error("Expected vpxa restart command to be executed")
	}
}

// TestSSHAuthenticationFailure tests authentication failure handling
func TestSSHAuthenticationFailure(t *testing.T) {
	server, err := NewMockSSHServer()
	if err != nil {
		t.Fatalf("Failed to start mock SSH server: %v", err)
	}
	defer server.Close()

	time.Sleep(100 * time.Millisecond)

	// Try to connect with wrong password
	config := &ssh.ClientConfig{
		User: "root",
		Auth: []ssh.AuthMethod{
			ssh.Password("wrong-password"),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}

	_, err = ssh.Dial("tcp", server.GetAddress(), config)
	if err == nil {
		t.Error("Expected authentication to fail with wrong password")
	}

	// Try to connect with non-existent user
	config.User = "nonexistent"
	config.Auth = []ssh.AuthMethod{ssh.Password("password")}

	_, err = ssh.Dial("tcp", server.GetAddress(), config)
	if err == nil {
		t.Error("Expected authentication to fail with non-existent user")
	}
}

// TestSSHCommandFailure tests command failure simulation
func TestSSHCommandFailure(t *testing.T) {
	server, err := NewMockSSHServer()
	if err != nil {
		t.Fatalf("Failed to start mock SSH server: %v", err)
	}
	defer server.Close()

	// Configure server to fail specific commands
	server.SetFailCommands([]string{"hostd restart"})

	time.Sleep(100 * time.Millisecond)

	config := &ssh.ClientConfig{
		User: "root",
		Auth: []ssh.AuthMethod{
			ssh.Password("password"),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}

	client, err := ssh.Dial("tcp", server.GetAddress(), config)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	// Try to restart hostd (should fail)
	session, err := client.NewSession()
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	err = session.Run("/etc/init.d/hostd restart")
	session.Close()

	if err == nil {
		t.Error("Expected hostd restart command to fail")
	}
}

// Helper function to generate Ed25519 host key for SSH server
func generateEd25519HostKey() (ssh.Signer, error) {
	// For testing, we'll use a dummy key
	// In production, you'd generate a real Ed25519 key
	privateKeyBytes := []byte(`-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
QyNTUxOQAAACDjU4tMZrBXLx5OUvKrqy2nHPnFZtzFgLSCEj1hN5nXVwAAAJjNHWOczR1j
nAAAAAtzc2gtZWQyNTUxOQAAACDjU4tMZrBXLx5OUvKrqy2nHPnFZtzFgLSCEj1hN5nXVw
AAAECEHiWtNDe4N8LZq7k7pP7K8L0tYlmJD5pF7LNLCJkE43E+NTi0xmsFcvHk5S8qurL
acc+cVm3MWAtIISPWE3mddXAAAAEGF6Z1JCZjhzaGlAY2l0YWRlbHMAAAAAQg==
-----END OPENSSH PRIVATE KEY-----`)

	signer, err := ssh.ParsePrivateKey(privateKeyBytes)
	if err != nil {
		// If parsing fails, generate a simple key for testing
		return generateSimpleHostKey()
	}

	return signer, nil
}

// generateSimpleHostKey generates a simple RSA key for testing if Ed25519 fails
func generateSimpleHostKey() (ssh.Signer, error) {
	// This is a simplified implementation for testing
	// In practice, you'd generate a proper host key
	return nil, fmt.Errorf("host key generation not implemented - using mock")
}
