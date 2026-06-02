package network

import "time"

// Speed represents network link speed
type Speed string

const (
	Speed10M     Speed = "10Mbps"
	Speed100M    Speed = "100Mbps"
	Speed1G      Speed = "1Gbps"
	Speed2_5G    Speed = "2.5Gbps"
	Speed10G     Speed = "10Gbps"
	SpeedDown    Speed = "Down"
	SpeedUnknown Speed = "Unknown"
)

// PortStatus represents the real-time status of a switch port
type PortStatus struct {
	ID           string // "port_1"
	Name         string // "Port 1" or "RPi"
	IsUp         bool
	Speed        Speed
	Duplex       string // "Full", "Half"
	VLANs        []int
	BytesIn      uint64
	BytesOut     uint64
	Errors       uint64
	ConnectedMAC string // Detected MAC address (LLDP/FDB)
}

// SwitchStatus holds the aggregate status of the switch
type SwitchStatus struct {
	Hostname    string
	IP          string
	Model       string
	Uptime      time.Duration
	Ports       []PortStatus
	Temperature float64 // If available
}
