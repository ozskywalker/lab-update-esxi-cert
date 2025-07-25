package version

import (
	"fmt"
	"log"
	"time"

	"github.com/tcnksm/go-latest"
)

// UpdateInfo contains information about available updates
type UpdateInfo struct {
	CurrentVersion string
	LatestVersion  string
	UpdateURL      string
	IsUpToDate     bool
}

// CheckForUpdates checks if there's a newer version available on GitHub
func CheckForUpdates(owner, repo string) (*UpdateInfo, error) {
	if owner == "" || repo == "" {
		return nil, fmt.Errorf("owner and repo must be specified for update checks")
	}

	// Create GitHub tag checker
	githubTag := &latest.GithubTag{
		Owner:      owner,
		Repository: repo,
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
		IsUpToDate:     res.Outdated == false,
	}

	return updateInfo, nil
}

// PrintUpdateNotification prints a user-friendly update notification
func (u *UpdateInfo) PrintUpdateNotification() {
	if u.IsUpToDate {
		fmt.Printf("✓ You are running the latest version (%s)\n", u.CurrentVersion)
		return
	}

	fmt.Printf("📦 Update available: %s → %s\n", u.CurrentVersion, u.LatestVersion)
	fmt.Printf("   Download: %s\n", u.UpdateURL)
	fmt.Println("   Run with --version to see current version details")
}

// QuietlyCheckForUpdates performs an update check without user interaction
// Returns true if an update is available, false otherwise
func QuietlyCheckForUpdates(owner, repo string) bool {
	updateInfo, err := CheckForUpdates(owner, repo)
	if err != nil {
		// Log the error but don't interrupt the user
		log.Printf("[DEBUG] Update check failed: %v", err)
		return false
	}

	return !updateInfo.IsUpToDate
}
