package updater

import (
	"fmt"
	"os"

	"github.com/blang/semver"
	"github.com/rhysd/go-github-selfupdate/selfupdate"
)

// ReleaseInfo holds information about a discovered update
type ReleaseInfo struct {
	Version      semver.Version
	ReleaseNotes string
	AssetURL     string
}

// CheckUpdate checks if there is a newer version available than currentVersion.
// Returns details of the update, valid=true if found, or an error.
func CheckUpdate(currentVersion string, repoSlug string) (*ReleaseInfo, bool, error) {
	if currentVersion == "dev" {
		return nil, false, nil // Dev versions don't update
	}

	latest, found, err := selfupdate.DetectLatest(repoSlug)
	if err != nil {
		return nil, false, fmt.Errorf("detect failed: %w", err)
	}

	v, err := semver.Parse(currentVersion)
	if err != nil {
		// If current version is not semantic, we assume it's dev or broken
		// But "dev" is handled above. If it's malformed, we might just fail.
		return nil, false, fmt.Errorf("invalid current version: %w", err)
	}

	if !found || latest.Version.LTE(v) {
		return nil, false, nil
	}

	return &ReleaseInfo{
		Version:      latest.Version,
		ReleaseNotes: latest.ReleaseNotes,
		AssetURL:     latest.AssetURL,
	}, true, nil
}

// PerformUpdate downloads and applies the update from the given AssetURL.
func PerformUpdate(assetURL string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not locate executable path: %w", err)
	}

	if err := selfupdate.UpdateTo(assetURL, exe); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}
	return nil
}
