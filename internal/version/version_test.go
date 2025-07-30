package version

import (
	"strings"
	"testing"
)

func TestGet(t *testing.T) {
	info := Get()

	if info == nil {
		t.Fatal("Get() returned nil")
	}

	// Test that all fields are present
	if info.Version == "" {
		t.Error("Version should not be empty")
	}

	if info.GoVersion == "" {
		t.Error("GoVersion should not be empty")
	}

	if info.Compiler == "" {
		t.Error("Compiler should not be empty")
	}

	if info.Platform == "" {
		t.Error("Platform should not be empty")
	}

	// Test platform format
	if !strings.Contains(info.Platform, "/") {
		t.Errorf("Platform should contain '/', got: %s", info.Platform)
	}
}

func TestVersionInfo_String(t *testing.T) {
	tests := []struct {
		name     string
		version  VersionInfo
		expected string
	}{
		{
			name: "with git tag",
			version: VersionInfo{
				Version:   "v1.0.0",
				GitCommit: "abcd1234567890",
				GitTag:    "v1.0.0",
			},
			expected: "v1.0.0 (abcd1234)",
		},
		{
			name: "with version no tag",
			version: VersionInfo{
				Version:   "v1.0.0",
				GitCommit: "abcd1234567890",
				GitTag:    "",
			},
			expected: "v1.0.0 (abcd1234)",
		},
		{
			name: "development version",
			version: VersionInfo{
				Version:   "development",
				GitCommit: "abcd1234567890",
				GitTag:    "",
			},
			expected: "development (abcd1234)",
		},
		{
			name: "short commit",
			version: VersionInfo{
				Version:   "v1.0.0",
				GitCommit: "abc123",
				GitTag:    "",
			},
			expected: "v1.0.0 (abc123)",
		},
		{
			name: "no commit",
			version: VersionInfo{
				Version:   "v1.0.0",
				GitCommit: "",
				GitTag:    "",
			},
			expected: "v1.0.0 (unknown)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.version.String()
			if result != tt.expected {
				t.Errorf("String() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestVersionInfo_Detailed(t *testing.T) {
	version := VersionInfo{
		Version:   "v1.0.0",
		GitCommit: "abcd1234567890",
		GitTag:    "v1.0.0",
		BuildDate: "2024-01-01T00:00:00Z",
		GoVersion: "go1.24.0",
		Compiler:  "gc",
		Platform:  "linux/amd64",
	}

	detailed := version.Detailed()

	// Check that all fields are present in the detailed output
	expectedFields := []string{
		"Version:    v1.0.0",
		"Git Commit: abcd1234567890",
		"Git Tag:    v1.0.0",
		"Build Date: 2024-01-01T00:00:00Z",
		"Go Version: go1.24.0",
		"Compiler:   gc",
		"Platform:   linux/amd64",
	}

	for _, field := range expectedFields {
		if !strings.Contains(detailed, field) {
			t.Errorf("Detailed() missing field: %s", field)
		}
	}
}

func TestBuildTimeVariables(t *testing.T) {
	// Test that build-time variables have default values
	if Version == "" {
		t.Error("Version variable should have a default value")
	}

	if BuildDate == "" {
		t.Error("BuildDate variable should have a default value")
	}

	// GitCommit and GitTag can be empty in development builds
}
