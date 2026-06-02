package inventory

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/DannyStrelok/kuargogo/internal/config"
)

// NodeEntry extends the basic config.Node with dynamic status information
type NodeEntry struct {
	config.Node
	IsOnline bool
}

// GetInventory retrieves all nodes from config and checks their online status concurrently
func GetInventory() []NodeEntry {
	cfg := config.GetConfig()
	nodes := cfg.Nodes
	entries := make([]NodeEntry, len(nodes))

	var wg sync.WaitGroup
	wg.Add(len(nodes))

	// Get common SSH port from config
	sshPort := cfg.SSH.Port
	if sshPort == 0 {
		sshPort = 22
	}

	for i, n := range nodes {
		go func(i int, node config.Node) {
			defer wg.Done()

			// Try to connect to the SSH port with a short timeout
			address := net.JoinHostPort(node.IP, fmt.Sprintf("%d", sshPort))
			conn, err := net.DialTimeout("tcp", address, 2*time.Second)

			isOnline := false
			if err == nil {
				isOnline = true
				_ = conn.Close()
			}

			entries[i] = NodeEntry{
				Node:     node,
				IsOnline: isOnline,
			}
		}(i, n)
	}

	wg.Wait()
	return entries
}
