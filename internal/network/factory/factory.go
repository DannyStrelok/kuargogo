package factory

import (
	"fmt"

	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/DannyStrelok/kuargogo/internal/network"
	"github.com/DannyStrelok/kuargogo/internal/network/simulated"
	"github.com/DannyStrelok/kuargogo/internal/network/tplink"
)

// NewManager creates a configured Network Manager with the appropriate driver
func NewManager(cfg config.Network, layout config.NetworkLayout) (*network.Manager, error) {
	if cfg.SwitchIP == "" && cfg.Driver == "" {
		return nil, fmt.Errorf("network is not configured")
	}

	var driver network.SwitchController

	switch cfg.Driver {
	case "tplink":
		driver = tplink.NewSG108E(cfg.SwitchIP, cfg.User, string(cfg.Password))
	case "simulated":
		driver = simulated.NewSwitch()
	default:
		// Fallback logic matches CLI behavior
		if cfg.SwitchIP != "" {
			driver = tplink.NewSG108E(cfg.SwitchIP, cfg.User, string(cfg.Password))
		} else {
			return nil, fmt.Errorf("unknown network driver: %s", cfg.Driver)
		}
	}

	return network.NewManager(driver, layout), nil
}
