package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/DannyStrelok/kuargogo/internal/hardware"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/spf13/cobra"
)

var envCmd = &cobra.Command{
	Use:   "env [status]",
	Short: "Check Rack Environment (Temp/Humidity)",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			_ = cmd.Help()
			return
		}

		if args[0] != "status" {
			fmt.Println("Unknown subcommand.")
			return
		}

		fmt.Println("Querying Rack Environment Sensors...")

		if DryRun {
			fmt.Println("[DRY-RUN] Simulated Reading:")
			fmt.Println("  Temperature: 24.5 °C")
			fmt.Println("  Humidity:    45.0 %")
			fmt.Println("  Fans:        AUTO (PWM 35%)")
			return
		}

		// Connect to MQTT
		mqttConfig := config.GetConfig().MQTT
		client, err := hardware.NewClient(mqttConfig.Broker, "kuargogo-env", mqttConfig.Username, string(mqttConfig.Password), false)
		if err != nil {
			fmt.Printf("Error connecting to MQTT: %v\n", err)
			return
		}
		defer client.Disconnect(250)

		// Subscribe and Wait
		topic := fmt.Sprintf("%s/env/status", mqttConfig.TopicPrefix)
		msgChan := make(chan []byte)

		token := client.Subscribe(topic, 0, func(c mqtt.Client, m mqtt.Message) {
			msgChan <- m.Payload()
		})
		token.Wait()

		select {
		case payload := <-msgChan:
			var data map[string]interface{}
			if err := json.Unmarshal(payload, &data); err != nil {
				fmt.Printf("Error parsing sensor data: %v\nRaw: %s\n", err, string(payload))
				return
			}

			// Pretty Print
			fmt.Println("---------------------------")
			if t, ok := data["temp"]; ok {
				fmt.Printf("  Temperature: %.1f °C\n", t)
			}
			if h, ok := data["humidity"]; ok {
				fmt.Printf("  Humidity:    %.1f %%\n", h)
			}
			if f, ok := data["fans"]; ok {
				fmt.Printf("  Fans (RPM):  %v\n", f)
			}
			fmt.Println("---------------------------")

		case <-time.After(3 * time.Second):
			fmt.Println("Timeout: No sensor data received from Infrastructure Manager.")
			fmt.Println("Check if the RPi daemon is running.")
		}
	},
}

func init() {
	rootCmd.AddCommand(envCmd)
}
