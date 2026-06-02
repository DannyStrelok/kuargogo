package simulated

import (
	"fmt"
	"time"

	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/DannyStrelok/kuargogo/internal/network"
)

type SimulatedSwitch struct {
	connected bool
}

func NewSwitch() network.SwitchController {
	return &SimulatedSwitch{}
}

func (s *SimulatedSwitch) Connect() error {
	s.connected = true
	return nil
}

func (s *SimulatedSwitch) Close() error {
	s.connected = false
	return nil
}

func (s *SimulatedSwitch) GetStatus() (*network.SwitchStatus, error) {
	if !s.connected {
		return nil, fmt.Errorf("not connected")
	}
	return &network.SwitchStatus{
		Hostname: "Simulated-Switch",
		IP:       "192.168.0.1",
		Model:    "Virt-SG108E",
		Uptime:   24 * time.Hour,
		Ports: []network.PortStatus{
			{ID: "port_1", Name: "Port 1", IsUp: true, Speed: network.Speed1G, Duplex: "Full", ConnectedMAC: "b8:27:eb:00:00:01"},
			{ID: "port_2", Name: "Port 2", IsUp: true, Speed: network.Speed1G, Duplex: "Full", ConnectedMAC: "98:90:96:00:00:02"},
			{ID: "port_3", Name: "Port 3", IsUp: false, Speed: network.SpeedDown},
			{ID: "port_4", Name: "Port 4", IsUp: false, Speed: network.SpeedDown},
			{ID: "port_5", Name: "Port 5", IsUp: false, Speed: network.SpeedDown},
			{ID: "port_6", Name: "Port 6", IsUp: false, Speed: network.SpeedDown},
			{ID: "port_7", Name: "Port 7", IsUp: false, Speed: network.SpeedDown},
			{ID: "port_8", Name: "Port 8", IsUp: true, Speed: network.Speed100M, Duplex: "Full", ConnectedMAC: "aa:bb:cc:dd:ee:ff"}, // Unknown Device
		},
	}, nil
}

func (s *SimulatedSwitch) ApplyConfig(layout config.NetworkLayout) error {
	if !s.connected {
		return fmt.Errorf("not connected")
	}
	// Simulate applying config
	time.Sleep(500 * time.Millisecond)
	return nil
}

func (s *SimulatedSwitch) GetMACTable() (map[string]string, error) {
	if !s.connected {
		return nil, fmt.Errorf("not connected")
	}
	// Return a simulated MAC table: PortID -> MAC
	return map[string]string{
		"port_1": "b8:27:eb:00:00:01", // Match RPi
		"port_2": "98:90:96:00:00:02", // Match HP
		"port_8": "aa:bb:cc:dd:ee:ff", // Unknown Device
	}, nil
}

func (s *SimulatedSwitch) Reboot() error {
	if !s.connected {
		return fmt.Errorf("not connected")
	}
	time.Sleep(1 * time.Second)
	return nil
}
