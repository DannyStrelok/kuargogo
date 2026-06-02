package provision

import (
	"strings"
)

// PowerAction defines the type of power operation
type PowerAction string

const (
	PowerOff    PowerAction = "off"
	PowerReboot PowerAction = "reboot"
)

// RemotePowerControl executes shutdown or reboot via SSH
func (e *Executor) RemotePowerControl(targetIP string, port int, action PowerAction) (string, error) {
	sshCmd := "sudo shutdown -h now"
	if action == PowerReboot {
		sshCmd = "sudo reboot"
	}

	out, err := e.ExecuteCommand(targetIP, port, sshCmd)
	if err != nil {
		// Reboot/Shutdown might drop connection immediately — this is expected
		errMsg := err.Error()
		if strings.Contains(errMsg, "status 255") ||
			strings.Contains(errMsg, "connection lost") ||
			strings.Contains(errMsg, "connection reset") ||
			strings.Contains(errMsg, "broken pipe") ||
			strings.Contains(errMsg, "EOF") ||
			strings.Contains(errMsg, "connection closed") ||
			strings.Contains(errMsg, "reset by peer") ||
			strings.Contains(errMsg, "use of closed network connection") ||
			strings.Contains(errMsg, "i/o timeout") {
			return "Command sent (connection closed as expected).", nil
		}
		return out, err
	}
	return out, nil
}
