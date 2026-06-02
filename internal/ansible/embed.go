package ansible

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"github.com/DannyStrelok/kuargogo/infrastructure"
)

// EnsurePlaybooksExtracted writes the embedded playbooks to a cache directory on disk.
// It then overlays any user-provided playbooks from ~/.kuargogo/playbooks.
// It returns the absolute path to the extracted playbooks directory.
func EnsurePlaybooksExtracted() (string, error) {
	usrHome, err := os.UserHomeDir()
	if err != nil {
		usrHome = os.TempDir()
	}

	baseDir := filepath.Join(usrHome, ".kuargogo")
	cacheDir := filepath.Join(baseDir, "playbooks_cache")
	userDir := filepath.Join(baseDir, "playbooks")

	// Ensure the cache dir exists
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create playbook cache dir: %w", err)
	}

	// Ensure the user override dir exists (as a hint for advanced users)
	if err := os.MkdirAll(userDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create user playbook dir: %w", err)
	}

	// Step 1: Walk the embedded FS and extract all core files
	err = fs.WalkDir(infrastructure.PlaybooksFS, "playbooks", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		outPath := filepath.Join(cacheDir, path)

		if d.IsDir() {
			return os.MkdirAll(outPath, 0755)
		}

		data, err := infrastructure.PlaybooksFS.ReadFile(path)
		if err != nil {
			return err
		}

		// Write embedded files
		return os.WriteFile(outPath, data, 0644)
	})

	if err != nil {
		return "", fmt.Errorf("failed to extract embedded playbooks: %w", err)
	}

	// Step 2: Overlay user files (if any exist in ~/.kuargogo/playbooks)
	targetPlaybooksDir := filepath.Join(cacheDir, "playbooks")
	if err := copyDir(userDir, targetPlaybooksDir); err != nil {
		return "", fmt.Errorf("failed to overlay user playbooks: %w", err)
	}

	return targetPlaybooksDir, nil
}

// ListAvailablePlaybooks returns a list of top-level playbooks and directories (like roles/)
// found in the embedded filesystem.
func ListAvailablePlaybooks() ([]string, error) {
	entries, err := fs.ReadDir(infrastructure.PlaybooksFS, "playbooks")
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded playbooks: %w", err)
	}

	var list []string
	for _, e := range entries {
		list = append(list, e.Name())
	}
	return list, nil
}

// ExportPlaybooks copies specific embedded playbooks or directories to the user's local ~/.kuargogo/playbooks folder.
// If overwrite is true, it replaces existing local files. If false, it skips them.
func ExportPlaybooks(selected []string, overwrite bool) (string, error) {
	usrHome, err := os.UserHomeDir()
	if err != nil {
		usrHome = os.TempDir()
	}

	userDir := filepath.Join(usrHome, ".kuargogo", "playbooks")
	if err := os.MkdirAll(userDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create user playbook dir: %w", err)
	}

	summary := ""
	for _, item := range selected {
		// IMPORTANT: Always use forward slashes for embed.FS, even on Windows.
		embeddedPath := path.Join("playbooks", item)

		err := fs.WalkDir(infrastructure.PlaybooksFS, embeddedPath, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			// embed.FS uses forward slashes internally. We need to normalize for path comparison.
			normalizedPath := filepath.FromSlash(p)
			relPath, _ := filepath.Rel("playbooks", normalizedPath)
			dstPath := filepath.Join(userDir, relPath)

			if d.IsDir() {
				return os.MkdirAll(dstPath, 0755)
			}

			// Check if file exists and handle overwrite
			if _, statErr := os.Stat(dstPath); statErr == nil && !overwrite {
				summary += fmt.Sprintf("⚠️  Skipped: %s (already exists)\n", relPath)
				return nil
			}

			data, err := infrastructure.PlaybooksFS.ReadFile(p)
			if err != nil {
				return err
			}

			if err := os.WriteFile(dstPath, data, 0644); err != nil {
				return err
			}
			summary += fmt.Sprintf("✅ Exported: %s\n", relPath)
			return nil
		})

		if err != nil {
			return summary, fmt.Errorf("failed to export '%s': %w", item, err)
		}
	}

	if summary == "" {
		summary = "No items selected or all items skipped."
	}

	return summary, nil
}

// copyDir recursively copies a directory tree, similar to cp -r.
// It overwrites existing files in the destination.
func copyDir(src string, dst string) error {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)

	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculate relative path from source root
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		// Skip the root dir itself
		if relPath == "." {
			return nil
		}

		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		// Ignore hidden files like .DS_Store or .git in the user's directory
		if filepath.Base(path)[0] == '.' {
			return nil
		}

		// Open source file
		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() { _ = srcFile.Close() }()

		// Create/truncate destination file
		dstFile, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
		if err != nil {
			return err
		}
		defer func() { _ = dstFile.Close() }()

		// Copy contents
		if _, err := io.Copy(dstFile, srcFile); err != nil {
			return err
		}

		return nil
	})
}
