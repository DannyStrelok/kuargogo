package simulated

import (
	"testing"

	"github.com/DannyStrelok/kuargogo/internal/network"
)

func TestSimulatedSwitchState(t *testing.T) {
	sw := NewSwitch()

	// Connect first
	if err := sw.Connect(); err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer func() { _ = sw.Close() }()

	// Initial check
	status, err := sw.GetStatus()
	if err != nil {
		t.Fatalf("failed to get status: %v", err)
	}

	// Find port 8 and check it's up and default VLAN 1
	var port8 *network.PortStatus
	for i := range status.Ports {
		if status.Ports[i].ID == "port_8" {
			port8 = &status.Ports[i]
			break
		}
	}
	if port8 == nil {
		t.Fatal("port_8 not found in switch status")
	}
	if !port8.IsUp {
		t.Error("expected port_8 to be initially UP")
	}
	if len(port8.VLANs) == 0 || port8.VLANs[0] != 1 {
		t.Errorf("expected port_8 to be initially in VLAN 1, got %v", port8.VLANs)
	}

	// Change state to DOWN
	if err := sw.SetPortState("port_8", false); err != nil {
		t.Fatalf("failed to set port state: %v", err)
	}

	status, err = sw.GetStatus()
	if err != nil {
		t.Fatalf("failed to get status: %v", err)
	}
	port8 = nil
	for i := range status.Ports {
		if status.Ports[i].ID == "port_8" {
			port8 = &status.Ports[i]
			break
		}
	}
	if port8.IsUp {
		t.Error("expected port_8 to be DOWN after SetPortState(false)")
	}

	// Change VLAN to 666
	if err := sw.SetPortVLAN("port_8", 666); err != nil {
		t.Fatalf("failed to set port VLAN: %v", err)
	}

	status, err = sw.GetStatus()
	if err != nil {
		t.Fatalf("failed to get status: %v", err)
	}
	port8 = nil
	for i := range status.Ports {
		if status.Ports[i].ID == "port_8" {
			port8 = &status.Ports[i]
			break
		}
	}
	if len(port8.VLANs) == 0 || port8.VLANs[0] != 666 {
		t.Errorf("expected port_8 to be in VLAN 666, got %v", port8.VLANs)
	}
}
