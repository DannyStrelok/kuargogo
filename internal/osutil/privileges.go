package osutil

import (
	"errors"
	"fmt"
)

// ErrElevationTriggered is returned when the process attempts to elevate privileges
// and needs to exit so the new elevated process can take over.
var ErrElevationTriggered = errors.New("privilege elevation triggered")

// IsAdmin checks if the current process has administrative/root privileges.
// Implementation varies by platform in privileges_windows.go and privileges_unix.go.
func IsAdmin() bool {
	return isAdmin()
}

// Elevate attempts to restart the current process with administrative privileges.
// On Windows, it triggers a UAC prompt. On Unix, it uses sudo.
func Elevate() error {
	return elevate()
}

// EnsureAdmin checks for administrative privileges and attempts to elevate if missing.
// If elevation is triggered, it returns ErrElevationTriggered.
func EnsureAdmin() error {
	if IsAdmin() {
		return nil
	}

	if err := Elevate(); err != nil {
		return fmt.Errorf("failed to elevate privileges: %w", err)
	}

	// Elevate successful (process restarted), return sentinel error
	return ErrElevationTriggered
}
