package hardware

import (
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"strings"
)

// ParseMAC normalizes and validates a MAC address
func ParseMAC(macAddr string) ([]byte, error) {
	macAddr = strings.ReplaceAll(macAddr, ":", "")
	macAddr = strings.ReplaceAll(macAddr, "-", "") // Handle 00-11-... format

	macBytes, err := hex.DecodeString(macAddr)
	if err != nil {
		return nil, fmt.Errorf("invalid MAC address format: %v", err)
	}
	if len(macBytes) != 6 {
		return nil, fmt.Errorf("invalid MAC address length: expected 6 bytes, got %d", len(macBytes))
	}
	return macBytes, nil
}

// broadcastFromIP derives the subnet broadcast address from a node IP.
// Assumes a /24 subnet (standard for homelabs). Falls back to limited broadcast.
func broadcastFromIP(ip string) string {
	parts := strings.Split(ip, ".")
	if len(parts) == 4 {
		parts[3] = "255"
		return strings.Join(parts, ".")
	}
	return "255.255.255.255"
}

// WakeOnLAN sends a magic packet to the specified MAC address.
// nodeIP is used to derive the subnet broadcast address (e.g. "192.168.1.101" → "192.168.1.255").
func WakeOnLAN(macAddr, nodeIP string) error {
	// 1. Parse MAC address
	macBytes, err := ParseMAC(macAddr)
	if err != nil {
		return err
	}

	// 2. Build Magic Packet (6x FF + 16x MAC)
	packet := make([]byte, 102)
	for i := 0; i < 6; i++ {
		packet[i] = 0xFF
	}
	for i := 0; i < 16; i++ {
		copy(packet[6+i*6:], macBytes)
	}

	// 3. Broadcast to derived subnet address
	broadcast := broadcastFromIP(nodeIP)
	addr, err := net.ResolveUDPAddr("udp", broadcast+":9")
	if err != nil {
		return fmt.Errorf("failed to resolve broadcast address: %v", err)
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return fmt.Errorf("failed to dial UDP: %v", err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			log.Printf("Warning: failed to close UDP connection: %v", err)
		}
	}()

	_, err = conn.Write(packet)
	if err != nil {
		return fmt.Errorf("failed to send magic packet: %v", err)
	}

	return nil
}
