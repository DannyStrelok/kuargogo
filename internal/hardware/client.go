package hardware

import (
	"fmt"
	"io"
	"os"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Client wraps the Paho MQTT client
type Client struct {
	mqtt.Client
	DryRun bool
	Output io.Writer // Destination for DryRun messages (defaults to os.Stdout)
}

// NewClient creates and connects a new MQTT client
func NewClient(broker, clientID, username, password string, dryRun bool) (*Client, error) {
	if dryRun {
		return &Client{DryRun: true, Output: os.Stdout}, nil
	}

	opts := mqtt.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetClientID(clientID)
	if username != "" {
		opts.SetUsername(username)
		opts.SetPassword(password)
	}
	opts.SetKeepAlive(2 * time.Second)
	opts.SetPingTimeout(1 * time.Second)

	c := mqtt.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}

	return &Client{Client: c, Output: os.Stdout}, nil
}

// BlinkLED sends a message to blink the LED of a specific node
// Topic: kgg/homelab/control/{node}/locate
// Payload: {"action": "blink", "duration": "5s"}
func (c *Client) BlinkLED(nodeName, topicPrefix string) error {
	topic := fmt.Sprintf("%s/control/%s/locate", topicPrefix, nodeName)
	payload := `{"action": "blink", "duration": "5s"}`

	if c.DryRun {
		_, _ = fmt.Fprintf(c.Output, "[DRY-RUN] Would publish to %s: %s\n", topic, payload)
		return nil
	}

	token := c.Publish(topic, 0, false, payload)
	token.Wait()
	return token.Error()
}

// SetColor changes the LED ring color and effect
// Topic: kgg/homelab/control/lighting/{target}
// Payload: {"color": "red", "effect": "breathing"}
func (c *Client) SetColor(color, effect, topicPrefix, target string) error {
	topic := fmt.Sprintf("%s/control/lighting/%s", topicPrefix, target)
	payload := fmt.Sprintf(`{"color": "%s", "effect": "%s"}`, color, effect)

	if c.DryRun {
		_, _ = fmt.Fprintf(c.Output, "[DRY-RUN] Would publish to %s: %s\n", topic, payload)
		return nil
	}

	token := c.Publish(topic, 0, false, payload)
	token.Wait()
	return token.Error()
}

// SetFanSpeed controls the rack fans
// Topic: kgg/homelab/control/fans/set
// Payload: {"mode": "manual", "speed": 50}
func (c *Client) SetFanSpeed(mode string, speed int, topicPrefix string) error {
	topic := fmt.Sprintf("%s/control/fans/set", topicPrefix)
	payload := fmt.Sprintf(`{"mode": "%s", "speed": %d}`, mode, speed)

	if c.DryRun {
		_, _ = fmt.Fprintf(c.Output, "[DRY-RUN] Would publish to %s: %s\n", topic, payload)
		return nil
	}

	token := c.Publish(topic, 0, false, payload)
	token.Wait()
	return token.Error()
}
