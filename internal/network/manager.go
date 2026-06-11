package network

import (
	"fmt"
	"log"

	"github.com/DannyStrelok/kuargogo/internal/config"
)

// SwitchController is the interface that all switch drivers must implement
type SwitchController interface {
	// Connect establishes session with the switch
	Connect() error
	// Close terminates the session
	Close() error

	// GetStatus returns the current state of ports and hardware
	GetStatus() (*SwitchStatus, error)

	// ApplyConfig enforces the declared network layout (VLANs, etc.)
	ApplyConfig(layout config.NetworkLayout) error

	// GetMACTable returns the Forwarding Database (FDB) - Port to MAC mapping
	GetMACTable() (map[string]string, error)

	// Reboot restarts the switch
	Reboot() error

	// SetPortState modifies the state (UP/DOWN) of a specific port
	SetPortState(portID string, isUp bool) error
	// SetPortVLAN binds a specific port to a VLAN ID
	SetPortVLAN(portID string, vlanID int) error
}

// Manager handles high-level network operations
type Manager struct {
	layout config.NetworkLayout
	driver SwitchController
}

// NewManager creates a new network manager with the provided driver
func NewManager(driver SwitchController, layout config.NetworkLayout) *Manager {
	return &Manager{
		layout: layout,
		driver: driver,
	}
}

// Proxy Methods to Driver

func (m *Manager) GetStatus() (*SwitchStatus, error) {
	if err := m.driver.Connect(); err != nil {
		return nil, err
	}
	defer func() {
		if err := m.driver.Close(); err != nil {
			log.Printf("Warning: failed to close switch driver: %v", err)
		}
	}()
	return m.driver.GetStatus()
}

func (m *Manager) ApplyConfig(layout config.NetworkLayout) error {
	if err := m.driver.Connect(); err != nil {
		return err
	}
	defer func() {
		if err := m.driver.Close(); err != nil {
			log.Printf("Warning: failed to close switch driver: %v", err)
		}
	}()
	return m.driver.ApplyConfig(layout)
}

func (m *Manager) Reboot() error {
	if err := m.driver.Connect(); err != nil {
		return err
	}
	defer func() {
		if err := m.driver.Close(); err != nil {
			log.Printf("Warning: failed to close switch driver: %v", err)
		}
	}()
	return m.driver.Reboot()
}

func (m *Manager) SetPortState(portID string, isUp bool) error {
	if err := m.driver.Connect(); err != nil {
		return err
	}
	defer func() {
		if err := m.driver.Close(); err != nil {
			log.Printf("Warning: failed to close switch driver: %v", err)
		}
	}()
	return m.driver.SetPortState(portID, isUp)
}

func (m *Manager) SetPortVLAN(portID string, vlanID int) error {
	if err := m.driver.Connect(); err != nil {
		return err
	}
	defer func() {
		if err := m.driver.Close(); err != nil {
			log.Printf("Warning: failed to close switch driver: %v", err)
		}
	}()
	return m.driver.SetPortVLAN(portID, vlanID)
}

// ValidatePhysicalConnections checks if the devices connected to ports match the inventory
func (m *Manager) ValidatePhysicalConnections(nodes []config.Node) ([]string, error) {
	if err := m.driver.Connect(); err != nil {
		return nil, err
	}
	defer func() {
		if err := m.driver.Close(); err != nil {
			log.Printf("Warning: failed to close switch driver: %v", err)
		}
	}()

	macTable, err := m.driver.GetMACTable()
	if err != nil {
		return nil, fmt.Errorf("failed to get MAC table: %w", err)
	}

	var violations []string

	// 1. Build Node Lookup Map
	nodeByMAC := make(map[string]config.Node)
	for _, n := range nodes {
		// Normalize MAC? Assuming lowercase for now as per yaml
		nodeByMAC[n.MAC] = n
	}

	// 2. Iterate connected devices
	for port, mac := range macTable {
		if node, exists := nodeByMAC[mac]; exists {
			// Found a known node
			// Check if this port is part of our known layout?
			// For now, just report where they are.
			// fmt.Printf("DEBUG: Found %s on %s\n", node.Name, port)
			_ = node
		} else {
			violations = append(violations, fmt.Sprintf("Unknown device (MAC: %s) found on %s", mac, port))
		}
	}

	// 3. Reverse Check: Are all nodes connected?
	// (Optional, maybe for 'status' not 'validate')

	return violations, nil
}

// GetNetworkMap returns a consolidated view of switch ports and connected inventory nodes
func (m *Manager) GetNetworkMap(nodes []config.Node) (*NetworkMap, error) {
	if err := m.driver.Connect(); err != nil {
		return nil, err
	}
	defer func() {
		if err := m.driver.Close(); err != nil {
			log.Printf("Warning: failed to close switch driver: %v", err)
		}
	}()

	// 1. Get raw status (Up/Down/Speed)
	status, err := m.driver.GetStatus()
	if err != nil {
		return nil, fmt.Errorf("failed to get switch status: %w", err)
	}

	// 2. Get MAC table (Port -> MAC)
	// Note: tplink driver scaffolding for GetStatus might return empty Ports,
	// and GetMACTable returns empty map.
	// We need to merge them.
	macTable, err := m.driver.GetMACTable()
	if err != nil {
		return nil, fmt.Errorf("failed to get MAC table: %w", err)
	}

	// 3. Build Node Lookup
	nodeByMAC := make(map[string]config.Node)
	for _, n := range nodes {
		nodeByMAC[n.MAC] = n
	}

	// 4. Build Map
	netMap := &NetworkMap{
		Hostname: status.Hostname,
		Model:    status.Model,
		Ports:    make([]PortMapEntry, 0, len(status.Ports)),
	}

	// If status.Ports is empty (driver not fully implemented), we can iterate raw MAC table or return empty.
	// For now, let's assume status.Ports is populated (Simulated driver does).

	// Create a map to quickly look up mac by port from the macTable
	macByPort := make(map[string]string)
	for port, mac := range macTable {
		macByPort[port] = mac
	}

	for _, p := range status.Ports {
		entry := PortMapEntry{
			PortID:   p.ID,
			PortName: p.Name,
			IsUp:     p.IsUp,
			Speed:    p.Speed,
		}

		// Check raw port info for MAC first (LLDP/internal), fallback to FDB table
		detectedMAC := p.ConnectedMAC
		if detectedMAC == "" {
			detectedMAC = macByPort[p.ID]
		}
		entry.ConnectedMAC = detectedMAC

		if detectedMAC != "" {
			if node, ok := nodeByMAC[detectedMAC]; ok {
				entry.NodeName = node.Name
				entry.NodeIP = node.IP
				entry.NodeRole = node.Role
			} else {
				entry.IsUnknownDevice = true
			}
		}

		netMap.Ports = append(netMap.Ports, entry)
	}

	return netMap, nil
}
