//go:build linux

package discovery

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/mdlayher/ethernet"
	"github.com/mdlayher/lldp"
	"github.com/mdlayher/packet"
)

// DiscoverLLDP Listens for LLDP frames on all interfaces.
// This requires NET_RAW capabilities (usually root).
func DiscoverLLDP() ([]LLDPDevice, error) {
	// 1. Identify interfaces
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to list interfaces: %w", err)
	}

	var devices []LLDPDevice

	type result struct {
		dev LLDPDevice
		err error
	}
	resChan := make(chan result)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	activeListeners := 0

	for _, iface := range ifaces {
		// filter out loopback and down
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		activeListeners++
		go func(ifi net.Interface) {
			dev, err := listenLLDP(ctx, &ifi)
			if err != nil {
				resChan <- result{err: err}
				return
			}
			resChan <- result{dev: dev}
		}(iface)
	}

	// Collect
	for i := 0; i < activeListeners; i++ {
		res := <-resChan
		if res.err == nil && res.dev.ChassisID != "" {
			devices = append(devices, res.dev)
		}
	}

	return devices, nil
}

func listenLLDP(ctx context.Context, iface *net.Interface) (LLDPDevice, error) {
	// Open raw socket using mdlayher/packet
	c, err := packet.Listen(iface, packet.Raw, 0x88cc, nil)
	if err != nil {
		return LLDPDevice{}, err
	}
	defer func() {
		err = c.Close()
		if err != nil {
			log.Printf("failed to close socket: %v\n", err)
		}
	}()

	// Set read deadline from context
	if d, ok := ctx.Deadline(); ok {
		if err := c.SetReadDeadline(d); err != nil {
			return LLDPDevice{}, fmt.Errorf("failed to set read deadline: %w", err)
		}
	}

	var dev LLDPDevice

	buf := make([]byte, 1500) // MTU
	for {
		n, _, err := c.ReadFrom(buf)
		if err != nil {
			return LLDPDevice{}, err
		}

		var f ethernet.Frame
		if err := f.UnmarshalBinary(buf[:n]); err != nil {
			continue
		}

		var l lldp.Frame
		if err := l.UnmarshalBinary(f.Payload); err != nil {
			continue
		}

		// Iterate over Optional TLVs
		// In mdlayher/lldp, TLV is a struct { Type, Length, Value }
		for _, tlv := range l.Optional {
			// Constants based on IEEE 802.1AB
			// 4 = Port Description
			// 5 = System Name
			// 6 = System Description
			switch tlv.Type {
			case 5: // System Name
				dev.SysName = string(tlv.Value)
			case 4: // Port Description
				// dev.PortDesc = string(tlv.Value)
			case 6: // System Description
				// dev.SysDesc = string(tlv.Value)
			}
		}

		// UnmarshalBinary in mdlayher/lldp usually populates ChassisID and PortID fields
		// if they are present as Mandatory TLVs.
		if l.ChassisID != nil && dev.ChassisID == "" {
			dev.ChassisID = string(l.ChassisID.ID)
		}
		if l.PortID != nil && dev.PortID == "" {
			dev.PortID = string(l.PortID.ID)
		}

		if dev.ChassisID != "" {
			return dev, nil
		}
	}
}
