package simulated

import (
	"fmt"
	"time"

	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/DannyStrelok/kuargogo/internal/network"
)

type SimulatedSwitch struct {
	connected  bool
	portStates map[string]bool
	portVlans  map[string]int
}

func NewSwitch() network.SwitchController {
	return &SimulatedSwitch{
		portStates: map[string]bool{
			"port_1": true,
			"port_2": true,
			"port_3": false,
			"port_4": false,
			"port_5": false,
			"port_6": false,
			"port_7": false,
			"port_8": true,
		},
		portVlans: map[string]int{
			"port_1": 1,
			"port_2": 1,
			"port_3": 1,
			"port_4": 1,
			"port_5": 1,
			"port_6": 1,
			"port_7": 1,
			"port_8": 1,
		},
	}
}

func (s *SimulatedSwitch) Connect() error {
	s.connected = true
	return nil
}

func (s *SimulatedSwitch) Close() error {
	s.connected = false
	return nil
}

func (s *SimulatedSwitch) ensureInitialized() {
	if s.portStates == nil {
		s.portStates = map[string]bool{
			"port_1": true,
			"port_2": true,
			"port_3": false,
			"port_4": false,
			"port_5": false,
			"port_6": false,
			"port_7": false,
			"port_8": true,
		}
	}
	if s.portVlans == nil {
		s.portVlans = map[string]int{
			"port_1": 1,
			"port_2": 1,
			"port_3": 1,
			"port_4": 1,
			"port_5": 1,
			"port_6": 1,
			"port_7": 1,
			"port_8": 1,
		}
	}
}

func (s *SimulatedSwitch) getSpeed(portID string) network.Speed {
	if !s.portStates[portID] {
		return network.SpeedDown
	}
	if portID == "port_8" {
		return network.Speed100M
	}
	return network.Speed1G
}

func (s *SimulatedSwitch) GetStatus() (*network.SwitchStatus, error) {
	if !s.connected {
		return nil, fmt.Errorf("not connected")
	}

	s.ensureInitialized()

	ports := []network.PortStatus{
		{ID: "port_1", Name: "Port 1", IsUp: s.portStates["port_1"], Speed: s.getSpeed("port_1"), Duplex: "Full", ConnectedMAC: "b8:27:eb:00:00:01", VLANs: []int{s.portVlans["port_1"]}},
		{ID: "port_2", Name: "Port 2", IsUp: s.portStates["port_2"], Speed: s.getSpeed("port_2"), Duplex: "Full", ConnectedMAC: "98:90:96:00:00:02", VLANs: []int{s.portVlans["port_2"]}},
		{ID: "port_3", Name: "Port 3", IsUp: s.portStates["port_3"], Speed: s.getSpeed("port_3"), VLANs: []int{s.portVlans["port_3"]}},
		{ID: "port_4", Name: "Port 4", IsUp: s.portStates["port_4"], Speed: s.getSpeed("port_4"), VLANs: []int{s.portVlans["port_4"]}},
		{ID: "port_5", Name: "Port 5", IsUp: s.portStates["port_5"], Speed: s.getSpeed("port_5"), VLANs: []int{s.portVlans["port_5"]}},
		{ID: "port_6", Name: "Port 6", IsUp: s.portStates["port_6"], Speed: s.getSpeed("port_6"), VLANs: []int{s.portVlans["port_6"]}},
		{ID: "port_7", Name: "Port 7", IsUp: s.portStates["port_7"], Speed: s.getSpeed("port_7"), VLANs: []int{s.portVlans["port_7"]}},
		{ID: "port_8", Name: "Port 8", IsUp: s.portStates["port_8"], Speed: s.getSpeed("port_8"), Duplex: "Full", ConnectedMAC: "aa:bb:cc:dd:ee:ff", VLANs: []int{s.portVlans["port_8"]}},
	}

	return &network.SwitchStatus{
		Hostname: "Simulated-Switch",
		IP:       "192.168.0.1",
		Model:    "Virt-SG108E",
		Uptime:   24 * time.Hour,
		Ports:    ports,
	}, nil
}

func (s *SimulatedSwitch) ApplyConfig(layout config.NetworkLayout) error {
	if !s.connected {
		return fmt.Errorf("not connected")
	}
	time.Sleep(50 * time.Millisecond)
	return nil
}

func (s *SimulatedSwitch) GetMACTable() (map[string]string, error) {
	if !s.connected {
		return nil, fmt.Errorf("not connected")
	}
	return map[string]string{
		"port_1": "b8:27:eb:00:00:01",
		"port_2": "98:90:96:00:00:02",
		"port_8": "aa:bb:cc:dd:ee:ff",
	}, nil
}

func (s *SimulatedSwitch) Reboot() error {
	if !s.connected {
		return fmt.Errorf("not connected")
	}
	time.Sleep(100 * time.Millisecond)
	return nil
}

func (s *SimulatedSwitch) SetPortState(portID string, isUp bool) error {
	if !s.connected {
		return fmt.Errorf("not connected")
	}
	s.ensureInitialized()
	s.portStates[portID] = isUp
	return nil
}

func (s *SimulatedSwitch) SetPortVLAN(portID string, vlanID int) error {
	if !s.connected {
		return fmt.Errorf("not connected")
	}
	s.ensureInitialized()
	s.portVlans[portID] = vlanID
	return nil
}
