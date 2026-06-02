//go:build !linux

package discovery

import (
	"fmt"
)

// DiscoverLLDP is a stub for non-Linux platforms where raw sockets for LLDP might be difficult or unsupported.
func DiscoverLLDP() ([]LLDPDevice, error) {
	return []LLDPDevice{}, fmt.Errorf("LLDP discovery is only supported on Linux")
}
