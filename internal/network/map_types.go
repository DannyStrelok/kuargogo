package network

// PortMapEntry represents a single row in the network map
type PortMapEntry struct {
	PortID          string
	PortName        string
	IsUp            bool
	Speed           Speed
	ConnectedMAC    string
	NodeName        string
	NodeIP          string
	NodeRole        string
	IsUnknownDevice bool
}

// NetworkMap holds the complete view of the switch
type NetworkMap struct {
	Hostname string
	Model    string
	Ports    []PortMapEntry
}
