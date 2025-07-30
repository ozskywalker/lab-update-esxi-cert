package version

import (
	"fmt"
	"log"
	"time"

	"github.com/tcnksm/go-latest"
)

// Constants for hardcoded repository information
const (
	GitHubOwner = "ozskywalker"
	GitHubRepo  = "lab-update-esxi-cert"
)

// UpdateInfo contains information about available updates
type UpdateInfo struct {
	CurrentVersion string
	LatestVersion  string
	UpdateURL      string
	IsUpToDate     bool
}

// CheckForUpdates checks if there's a newer version available on GitHub
// Uses hardcoded repository information
func CheckForUpdates() (*UpdateInfo, error) {
	// Create GitHub tag checker with hardcoded repo info
	githubTag := &latest.GithubTag{
		Owner:      GitHubOwner,
		Repository: GitHubRepo,
	}

	// Get current version info
	current := Get()
	currentVer := current.Version
	if current.GitTag != "" {
		currentVer = current.GitTag
	}

	// Check for updates with timeout
	done := make(chan bool, 1)
	var res *latest.CheckResponse
	var err error

	go func() {
		res, err = latest.Check(githubTag, currentVer)
		done <- true
	}()

	// Wait for result with timeout
	select {
	case <-done:
		if err != nil {
			return nil, fmt.Errorf("failed to check for updates: %v", err)
		}
	case <-time.After(10 * time.Second):
		return nil, fmt.Errorf("update check timed out")
	}

	// Build update info
	updateInfo := &UpdateInfo{
		CurrentVersion: currentVer,
		LatestVersion:  res.Current,
		UpdateURL:      res.Meta.URL,
		IsUpToDate:     !res.Outdated,
	}

	return updateInfo, nil
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
