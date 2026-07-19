package updater

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/blang/semver"
	"github.com/minio/selfupdate"
)

// ReleaseInfo holds information about a discovered update
type ReleaseInfo struct {
	Version      semver.Version
	ReleaseNotes string
	AssetURL     string
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type githubRelease struct {
	TagName string        `json:"tag_name"`
	Body    string        `json:"body"`
	Assets  []githubAsset `json:"assets"`
}

// CheckUpdate checks if there is a newer version available than currentVersion.
// Returns details of the update, valid=true if found, or an error.
func CheckUpdate(currentVersion string, repoSlug string) (*ReleaseInfo, bool, error) {
	if currentVersion == "dev" {
		return nil, false, nil // Dev versions don't update
	}

	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repoSlug), nil)
	if err != nil {
		return nil, false, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "token "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("github api returned status %s", resp.Status)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, false, fmt.Errorf("failed to decode release info: %w", err)
	}

	// Clean up TagName to parse semver (e.g. "v1.2.3" -> "1.2.3")
	cleanTag := strings.TrimPrefix(release.TagName, "v")
	latestVersion, err := semver.Parse(cleanTag)
	if err != nil {
		return nil, false, fmt.Errorf("invalid latest version %q: %w", release.TagName, err)
	}

	currentSemver, err := semver.Parse(strings.TrimPrefix(currentVersion, "v"))
	if err != nil {
		return nil, false, fmt.Errorf("invalid current version %q: %w", currentVersion, err)
	}

	if latestVersion.LTE(currentSemver) {
		return nil, false, nil
	}

	// Match asset for the current OS and architecture
	assetURL, _ := matchAsset(release.Assets)
	if assetURL == "" {
		return nil, false, fmt.Errorf("no matching binary found for OS %s and Arch %s in latest release", runtime.GOOS, runtime.GOARCH)
	}

	return &ReleaseInfo{
		Version:      latestVersion,
		ReleaseNotes: release.Body,
		AssetURL:     assetURL,
	}, true, nil
}

// PerformUpdate downloads and applies the update from the given AssetURL.
func PerformUpdate(assetURL string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not locate executable path: %w", err)
	}

	client := &http.Client{
		Timeout: 2 * time.Minute,
	}

	req, err := http.NewRequest("GET", assetURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create download request: %w", err)
	}

	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "token "+token)
	}

	resp, clientErr := client.Do(req)
	if clientErr != nil {
		return fmt.Errorf("failed to download update: %w", clientErr)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %s", resp.Status)
	}

	// Decompress the asset
	binaryReader, err := decompress(resp.Body, filepath.Base(assetURL))
	if err != nil {
		return fmt.Errorf("decompression failed: %w", err)
	}

	// Apply update using minio/selfupdate
	err = selfupdate.Apply(binaryReader, selfupdate.Options{
		TargetPath: exe,
	})
	if err != nil {
		return fmt.Errorf("failed to apply update: %w", err)
	}

	return nil
}

func matchAsset(assets []githubAsset) (string, string) {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	suffix := fmt.Sprintf("%s_%s", goos, goarch)
	if goarch == "arm" {
		suffix = fmt.Sprintf("%s_armv7", goos) // GoReleaser default for 32-bit ARM
	}

	for _, asset := range assets {
		name := strings.ToLower(asset.Name)
		if strings.Contains(name, "kuargogo") && strings.Contains(name, suffix) {
			return asset.BrowserDownloadURL, asset.Name
		}
	}
	return "", ""
}

func decompress(r io.Reader, filename string) (io.Reader, error) {
	filenameLower := strings.ToLower(filename)
	if strings.HasSuffix(filenameLower, ".tar.gz") || strings.HasSuffix(filenameLower, ".tgz") {
		gr, err := gzip.NewReader(r)
		if err != nil {
			return nil, err
		}
		tr := tar.NewReader(gr)
		for {
			header, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, err
			}
			baseName := filepath.Base(header.Name)
			if baseName == "kgg" || baseName == "kgg.exe" {
				// We must read it into memory because the tar reader is sequential,
				// and returning a pointer to the reader while iterating could close or move past it.
				var buf bytes.Buffer
				if _, err := io.Copy(&buf, tr); err != nil {
					return nil, err
				}
				return bytes.NewReader(buf.Bytes()), nil
			}
		}
		return nil, fmt.Errorf("binary 'kgg' not found in tarball")
	} else if strings.HasSuffix(filenameLower, ".zip") {
		data, err := io.ReadAll(r)
		if err != nil {
			return nil, err
		}
		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return nil, err
		}
		for _, f := range zr.File {
			baseName := filepath.Base(f.Name)
			if baseName == "kgg" || baseName == "kgg.exe" {
				rc, err := f.Open()
				if err != nil {
					return nil, err
				}
				var buf bytes.Buffer
				if _, err := io.Copy(&buf, rc); err != nil {
					_ = rc.Close()
					return nil, err
				}
				_ = rc.Close()
				return bytes.NewReader(buf.Bytes()), nil
			}
		}
		return nil, fmt.Errorf("binary 'kgg' not found in zip archive")
	}
	// Raw binary
	return r, nil
}
