package version

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/hashicorp/go-version"
)

// Constants for hardcoded repository information
const (
	GitHubOwner = "ozskywalker"
	GitHubRepo  = "lab-update-esxi-cert"

	// updateCheckTimeout bounds the GitHub API request. The previous
	// implementation (tcnksm/go-latest) blocked until a goroutine-based timeout;
	// we now enforce it directly on the HTTP client and request context.
	updateCheckTimeout = 10 * time.Second
)

// httpClient is the HTTP client used for update checks. It is a package
// variable so tests can inject a client pointed at a mock GitHub server.
var httpClient = &http.Client{Timeout: updateCheckTimeout}

// githubAPIBase is the base URL for the GitHub API. It is a package variable so
// tests can redirect the update check to a mock server.
var githubAPIBase = "https://api.github.com"

// UpdateInfo contains information about available updates
type UpdateInfo struct {
	CurrentVersion string
	LatestVersion  string
	UpdateURL      string
	IsUpToDate     bool
}

// githubTag is the subset of a GitHub "tags" response we need.
type githubTag struct {
	Name string `json:"name"`
}

// CheckForUpdates checks if there's a newer version available on GitHub
// by querying the GitHub releases API for the hardcoded repository.
func CheckForUpdates() (*UpdateInfo, error) {
	return checkForUpdatesWithClient(httpClient)
}

// checkForUpdatesWithClient performs the update check using the provided HTTP
// client. Splitting the HTTP client out keeps the comparison logic unit-testable
// against a mock GitHub server.
//
// We use the "/tags" endpoint (newest first) rather than "/releases/latest"
// because this project releases as drafts (goreleaser `release.draft: true`); a
// draft is not returned by "/releases/latest" until it is published. Querying
// tags preserves the previous behavior of advertising updates as soon as a tag
// exists, including drafts.
func checkForUpdatesWithClient(client *http.Client) (*UpdateInfo, error) {
	endpoint := fmt.Sprintf("%s/repos/%s/%s/tags", githubAPIBase, GitHubOwner, GitHubRepo)

	ctx, cancel := context.WithTimeout(context.Background(), updateCheckTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build update check request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to check for updates: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("update check failed with status %s", resp.Status)
	}

	var tags []githubTag
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return nil, fmt.Errorf("failed to parse update check response: %w", err)
	}
	if len(tags) == 0 || tags[0].Name == "" {
		return nil, fmt.Errorf("update check returned no tags")
	}
	latest := tags[0].Name

	// Resolve the version we are currently running. A release build injects
	// GitTag via ldflags; otherwise fall back to the Version variable.
	info := Get()
	current := info.Version
	if info.GitTag != "" {
		current = info.GitTag
	}
	if current == "" {
		current = "development"
	}

	isUpToDate, err := compareVersions(current, latest)
	if err != nil {
		return nil, fmt.Errorf("failed to compare versions: %w", err)
	}

	updateURL := fmt.Sprintf("https://github.com/%s/%s/releases/tag/%s", GitHubOwner, GitHubRepo, latest)

	return &UpdateInfo{
		CurrentVersion: current,
		LatestVersion:  latest,
		UpdateURL:      updateURL,
		IsUpToDate:     isUpToDate,
	}, nil
}

// compareVersions reports whether the running version is at least as new as the
// latest released version, using semantic version comparison (the "v" prefix is
// accepted by hashicorp/go-version).
func compareVersions(current, latest string) (bool, error) {
	curVer, err := version.NewVersion(current)
	if err != nil {
		return false, fmt.Errorf("current version %q is not valid semver: %v", current, err)
	}
	latestVer, err := version.NewVersion(latest)
	if err != nil {
		return false, fmt.Errorf("latest version %q is not valid semver: %v", latest, err)
	}
	return curVer.GreaterThanOrEqual(latestVer), nil
}

// GetUpdateNotification returns a single-line update notification string
// Returns empty string if up-to-date or check fails
func GetUpdateNotification() string {
	updateInfo, err := CheckForUpdates()
	if err != nil {
		// Silently fail - don't interrupt normal operation
		return ""
	}

	if updateInfo.IsUpToDate {
		return ""
	}

	return fmt.Sprintf("📦 Update available: %s → %s - Download: %s",
		updateInfo.CurrentVersion, updateInfo.LatestVersion, updateInfo.UpdateURL)
}

// PrintUpdateNotification prints a user-friendly update notification
func (u *UpdateInfo) PrintUpdateNotification() {
	if u.IsUpToDate {
		fmt.Printf("✓ You are running the latest version (%s)\n", u.CurrentVersion)
		return
	}

	fmt.Printf("📦 Update available: %s → %s\n", u.CurrentVersion, u.LatestVersion)
	fmt.Printf("   Download: %s\n", u.UpdateURL)
}

// QuietlyCheckForUpdates performs an update check without user interaction
// Returns true if an update is available, false otherwise
func QuietlyCheckForUpdates() bool {
	updateInfo, err := CheckForUpdates()
	if err != nil {
		// Log the error but don't interrupt the user
		log.Printf("[DEBUG] Update check failed: %v", err)
		return false
	}

	return !updateInfo.IsUpToDate
}
