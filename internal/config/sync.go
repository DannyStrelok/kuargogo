package config

import (
	"fmt"
)

// SyncProvider defines the interface for cloud synchronization backends.
type SyncProvider interface {
	// Push uploads encrypted configuration data.
	Push(data []byte) error
	// Pull downloads the latest encrypted configuration data.
	Pull() ([]byte, error)
	// Logout clears the provider's local session.
	Logout() error
}

// GetSyncProvider returns the active sync provider based on configuration.
func GetSyncProvider() (SyncProvider, error) {
	sync := RootConfigGetSync()
	providerType := sync.Provider

	switch providerType {
	case "s3":
		return NewS3Provider(), nil
	case "":
		return nil, fmt.Errorf("no sync provider configured")
	default:
		return nil, fmt.Errorf("unknown sync provider: %s", providerType)
	}
}
