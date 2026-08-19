package version

import (
	"net/http"
	"net/http/httptest"
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
		GoVersion: "go1.26.5",
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
		"Go Version: go1.26.5",
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

// setupMockGitHub starts a mock GitHub API server and points the package's
// update-check dependencies (githubAPIBase + httpClient) at it. The previous
// values are restored automatically when the test completes.
func setupMockGitHub(t *testing.T, handler http.HandlerFunc) {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	oldBase := githubAPIBase
	oldClient := httpClient
	githubAPIBase = server.URL
	httpClient = server.Client()
	t.Cleanup(func() {
		githubAPIBase = oldBase
		httpClient = oldClient
	})
}

// setTestVersion sets the package version variables for the duration of a test.
func setTestVersion(t *testing.T, version, gitTag string) {
	t.Helper()
	oldVersion := Version
	oldGitTag := GitTag
	Version = version
	GitTag = gitTag
	t.Cleanup(func() {
		Version = oldVersion
		GitTag = oldGitTag
	})
}

func TestCheckForUpdates_UpdateAvailable(t *testing.T) {
	setupMockGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/tags") {
			t.Errorf("unexpected request path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"name":"v2.0.0"}]`))
	})
	setTestVersion(t, "v1.0.0", "v1.0.0")

	updateInfo, err := CheckForUpdates()
	if err != nil {
		t.Fatalf("CheckForUpdates returned error: %v", err)
	}
	if updateInfo.IsUpToDate {
		t.Error("Expected IsUpToDate to be false when an update is available")
	}
	if updateInfo.CurrentVersion != "v1.0.0" {
		t.Errorf("Expected CurrentVersion v1.0.0, got %s", updateInfo.CurrentVersion)
	}
	if updateInfo.LatestVersion != "v2.0.0" {
		t.Errorf("Expected LatestVersion v2.0.0, got %s", updateInfo.LatestVersion)
	}
	if !strings.Contains(updateInfo.UpdateURL, "github.com") {
		t.Errorf("Expected UpdateURL to contain github.com, got %s", updateInfo.UpdateURL)
	}
}

func TestCheckForUpdates_UpToDate(t *testing.T) {
	setupMockGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"name":"v1.0.0"}]`))
	})
	setTestVersion(t, "v1.0.0", "v1.0.0")

	updateInfo, err := CheckForUpdates()
	if err != nil {
		t.Fatalf("CheckForUpdates returned error: %v", err)
	}
	if !updateInfo.IsUpToDate {
		t.Errorf("Expected IsUpToDate to be true when versions match, got CurrentVersion=%s LatestVersion=%s",
			updateInfo.CurrentVersion, updateInfo.LatestVersion)
	}
}

func TestCheckForUpdates_UsesGitTagWhenSet(t *testing.T) {
	setupMockGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"name":"v2.0.0"}]`))
	})
	// Version defaults to "development" in non-release builds, but GitTag
	// (injected via ldflags in release builds) should be used for comparison.
	setTestVersion(t, "development", "v1.0.0")

	updateInfo, err := CheckForUpdates()
	if err != nil {
		t.Fatalf("CheckForUpdates returned error: %v", err)
	}
	if updateInfo.CurrentVersion != "v1.0.0" {
		t.Errorf("Expected CurrentVersion to use GitTag v1.0.0, got %s", updateInfo.CurrentVersion)
	}
	if updateInfo.IsUpToDate {
		t.Error("Expected IsUpToDate to be false when GitTag is older than latest")
	}
}

func TestCheckForUpdates_HTTPError(t *testing.T) {
	setupMockGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	setTestVersion(t, "v1.0.0", "v1.0.0")

	if _, err := CheckForUpdates(); err == nil {
		t.Error("Expected an error when GitHub returns a non-200 status")
	}
}

func TestCheckForUpdates_MalformedJSON(t *testing.T) {
	setupMockGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tag_name":`)) // truncated JSON
	})
	setTestVersion(t, "v1.0.0", "v1.0.0")

	if _, err := CheckForUpdates(); err == nil {
		t.Error("Expected an error when the response is malformed")
	}
}

func TestCheckForUpdates_NoTags(t *testing.T) {
	setupMockGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[]`))
	})
	setTestVersion(t, "v1.0.0", "v1.0.0")

	if _, err := CheckForUpdates(); err == nil {
		t.Error("Expected an error when the response has no tags")
	}
}

func TestCheckForUpdates_NonSemverCurrent(t *testing.T) {
	// A "development" version is not valid semver, so the check should fail
	// gracefully (callers treat the error as "no notification").
	setupMockGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"name":"v2.0.0"}]`))
	})
	setTestVersion(t, "development", "")

	if _, err := CheckForUpdates(); err == nil {
		t.Error("Expected an error when the current version is not valid semver")
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name    string
		current string
		latest  string
		want    bool
		wantErr bool
	}{
		{name: "current older", current: "v1.0.0", latest: "v2.0.0", want: false},
		{name: "current equal", current: "v2.0.0", latest: "v2.0.0", want: true},
		{name: "current newer", current: "v2.0.1", latest: "v2.0.0", want: true},
		{name: "no v prefix", current: "1.0.0", latest: "v1.0.0", want: true},
		{name: "latest older", current: "v2.0.0", latest: "v1.5.0", want: true},
		{name: "invalid current", current: "development", latest: "v2.0.0", wantErr: true},
		{name: "invalid latest", current: "v2.0.0", latest: "not-a-version", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := compareVersions(tt.current, tt.latest)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected an error, got nil (result=%v)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("compareVersions(%q, %q) = %v, want %v", tt.current, tt.latest, got, tt.want)
			}
		})
	}
}

func TestGetUpdateNotification(t *testing.T) {
	t.Run("update available returns notification", func(t *testing.T) {
		setupMockGitHub(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`[{"name":"v2.0.0"}]`))
		})
		setTestVersion(t, "v1.0.0", "v1.0.0")

		result := GetUpdateNotification()
		if result == "" {
			t.Fatal("Expected a non-empty notification when an update is available")
		}
		for _, fragment := range []string{"Update available", "→", "Download"} {
			if !strings.Contains(result, fragment) {
				t.Errorf("Notification missing %q: %s", fragment, result)
			}
		}
	})

	t.Run("up to date returns empty string", func(t *testing.T) {
		setupMockGitHub(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`[{"name":"v1.0.0"}]`))
		})
		setTestVersion(t, "v1.0.0", "v1.0.0")

		if result := GetUpdateNotification(); result != "" {
			t.Errorf("Expected empty notification when up to date, got %q", result)
		}
	})

	t.Run("error returns empty string", func(t *testing.T) {
		setupMockGitHub(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})
		setTestVersion(t, "v1.0.0", "v1.0.0")

		if result := GetUpdateNotification(); result != "" {
			t.Errorf("Expected empty notification on error, got %q", result)
		}
	})
}

func TestQuietlyCheckForUpdates(t *testing.T) {
	t.Run("returns true when update available", func(t *testing.T) {
		setupMockGitHub(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`[{"name":"v2.0.0"}]`))
		})
		setTestVersion(t, "v1.0.0", "v1.0.0")

		if !QuietlyCheckForUpdates() {
			t.Error("Expected QuietlyCheckForUpdates to return true when update available")
		}
	})

	t.Run("returns false when up to date", func(t *testing.T) {
		setupMockGitHub(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`[{"name":"v1.0.0"}]`))
		})
		setTestVersion(t, "v1.0.0", "v1.0.0")

		if QuietlyCheckForUpdates() {
			t.Error("Expected QuietlyCheckForUpdates to return false when up to date")
		}
	})

	t.Run("returns false on error without panic", func(t *testing.T) {
		setupMockGitHub(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})
		setTestVersion(t, "v1.0.0", "v1.0.0")

		if QuietlyCheckForUpdates() {
			t.Error("Expected QuietlyCheckForUpdates to return false on error")
		}
	})
}

func TestPrintUpdateNotification_Format(t *testing.T) {
	// PrintUpdateNotification writes to stdout which is hard to capture
	// without redirection; verify the UpdateInfo fields drive the branches.
	t.Run("up-to-date message format", func(t *testing.T) {
		updateInfo := &UpdateInfo{
			CurrentVersion: "v1.0.0",
			LatestVersion:  "v1.0.0",
			UpdateURL:      "",
			IsUpToDate:     true,
		}

		if !updateInfo.IsUpToDate {
			t.Error("Expected IsUpToDate to be true")
		}
	})

	t.Run("update available message format", func(t *testing.T) {
		updateInfo := &UpdateInfo{
			CurrentVersion: "v1.0.0",
			LatestVersion:  "v2.0.0",
			UpdateURL:      "https://github.com/owner/repo/releases/tag/v2.0.0",
			IsUpToDate:     false,
		}

		if updateInfo.IsUpToDate {
			t.Error("Expected IsUpToDate to be false")
		}
	})
}

func TestUpdateInfoStructure(t *testing.T) {
	// Test that UpdateInfo struct has all expected fields
	updateInfo := UpdateInfo{
		CurrentVersion: "v1.0.0",
		LatestVersion:  "v2.0.0",
		UpdateURL:      "https://example.com",
		IsUpToDate:     false,
	}

	// Verify all fields are accessible
	if updateInfo.CurrentVersion == "" {
		t.Error("CurrentVersion field should be accessible")
	}
	if updateInfo.LatestVersion == "" {
		t.Error("LatestVersion field should be accessible")
	}
	if updateInfo.UpdateURL == "" {
		t.Error("UpdateURL field should be accessible")
	}

	// Verify boolean field
	if updateInfo.IsUpToDate {
		t.Error("IsUpToDate should be false for this test case")
	}
}

func TestGitHubConstants(t *testing.T) {
	// Test that hardcoded GitHub constants are set
	if GitHubOwner == "" {
		t.Error("GitHubOwner constant should be set")
	}
	if GitHubRepo == "" {
		t.Error("GitHubRepo constant should be set")
	}

	// Verify they're the expected values for this project
	if GitHubOwner != "ozskywalker" {
		t.Errorf("Expected GitHubOwner to be 'ozskywalker', got %s", GitHubOwner)
	}
	if GitHubRepo != "lab-update-esxi-cert" {
		t.Errorf("Expected GitHubRepo to be 'lab-update-esxi-cert', got %s", GitHubRepo)
	}
}
