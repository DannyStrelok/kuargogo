package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/DannyStrelok/kuargogo/internal/ai"
	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/DannyStrelok/kuargogo/internal/hardware"
	"github.com/spf13/cobra"
)

// getClient helper to reuse client creation logic
func getClient() (ai.Client, error) {
	cfg := config.GetConfig().AI
	return ai.NewClient(cfg, config.IsDryRun())
}

var aiCmd = &cobra.Command{
	Use:   "ai [prompt]",
	Short: "Interact with local AI models",
	Long:  `Send prompts to the Ollama service running on the GPU-enabled node.`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) > 0 {
			runGenerate(cmd, args)
			return
		}
		_ = cmd.Help()
	},
}

func runGenerate(cmd *cobra.Command, args []string) {
	prompt := strings.Join(args, " ")
	model, _ := cmd.Flags().GetString("model")

	client, err := getClient()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Visual Feedback: AI Processing (Purple)
	mqttConfig := config.GetConfig().MQTT
	hwClient, hwErr := hardware.NewClient(mqttConfig.Broker, "kuargogo-ai", mqttConfig.Username, string(mqttConfig.Password), DryRun)
	if hwErr == nil {
		defer hwClient.Disconnect(250)
		if err := hwClient.SetColor("purple", "breathing", mqttConfig.TopicPrefix, "global"); err != nil {
			log.Printf("Warning: failed to set AI LED color: %v\n", err)
		}
		defer func() {
			if err := hwClient.SetColor("green", "static", mqttConfig.TopicPrefix, "global"); err != nil {
				log.Printf("Warning: failed to reset AI LED color: %v\n", err)
			}
		}()
	}

	fmt.Printf("asking %s...\n", model)
	err = client.Generate(prompt)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		if strings.Contains(err.Error(), "connection refused") {
			fmt.Println("Tip: Ensure Ollama is running and accessible on port 11434.")
		}
	}
}

var aiStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show running AI models and VRAM usage",
	Run: func(cmd *cobra.Command, args []string) {
		client, err := getClient()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		fmt.Println("Fetching AI Status...")
		models, err := client.ListRunning()
		if err != nil {
			fmt.Printf("Error fetching status: %v\n", err)
			return
		}

		if len(models) == 0 {
			fmt.Println("No models currently loaded in memory.")
			return
		}

		fmt.Printf("%-20s | %-15s\n", "Model", "VRAM Usage")
		fmt.Println(strings.Repeat("-", 38))
		for _, m := range models {
			vramMB := float64(m.SizeVRAM) / 1024 / 1024
			fmt.Printf("%-20s | %.0f MB\n", m.Name, vramMB)
		}
	},
}

var aiPullCmd = &cobra.Command{
	Use:   "pull [model_name]",
	Short: "Download a model to the AI node",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		modelName := args[0]
		client, err := getClient()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		fmt.Printf("Pulling model '%s'...\n", modelName)
		err = client.Pull(modelName, func(status string, total, completed int64) {
			if total > 0 {
				percent := float64(completed) / float64(total) * 100
				fmt.Printf("\r%-15s [%.1f%%]     ", status, percent)
			} else {
				fmt.Printf("\r%-15s          ", status)
			}
		})
		fmt.Println()
		if err != nil {
			fmt.Printf("Error pulling model: %v\n", err)
		} else {
			fmt.Println("Pull complete.")
		}
	},
}

var aiChatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Start an interactive chat session",
	Run: func(cmd *cobra.Command, args []string) {
		client, err := getClient()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		model, _ := cmd.Flags().GetString("model")

		// Visual Feedback: AI Session (Purple)
		mqttConfig := config.GetConfig().MQTT
		hwClient, hwErr := hardware.NewClient(mqttConfig.Broker, "kuargogo-chat", mqttConfig.Username, string(mqttConfig.Password), DryRun)
		if hwErr == nil {
			defer hwClient.Disconnect(250)
			_ = hwClient.SetColor("purple", "static", mqttConfig.TopicPrefix, "global")
			defer func() { _ = hwClient.SetColor("green", "static", mqttConfig.TopicPrefix, "global") }()
		}

		fmt.Printf("Starting chat with %s. Type 'exit' to quit.\n", model)
		reader := bufio.NewReader(os.Stdin)

		for {
			fmt.Print(">>> ")
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)

			if input == "exit" || input == "quit" {
				break
			}
			if input == "" {
				continue
			}

			err := client.Generate(input)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			}
		}
	},
}

func init() {
	aiCmd.PersistentFlags().StringP("model", "m", "llama3", "Ollama model to use")
	aiCmd.AddCommand(aiStatusCmd)
	aiCmd.AddCommand(aiPullCmd)
	aiCmd.AddCommand(aiChatCmd)
	aiCmd.AddCommand(aiSkillCmd)
	aiCmd.AddCommand(aiInterpretCmd)
	aiCmd.AddCommand(aiExplainCmd)
	rootCmd.AddCommand(aiCmd)
}

var aiInterpretCmd = &cobra.Command{
	Use:     "interpret [message]",
	Aliases: []string{"intent"},
	Short:   "Convert natural language into a structured JSON intent",
	Args:    cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		message := strings.Join(args, " ")
		client, err := getClient()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		// Silence usual generation output to only get JSON in stdout
		client.SetOutput(os.Stderr)

		intent, err := ai.Interpret(client, message)
		if err != nil {
			fmt.Printf("Error during interpretation: %v\n", err)
			return
		}

		jsonData, _ := json.MarshalIndent(intent, "", "  ")
		fmt.Println(string(jsonData))
	},
}

var aiSkillCmd = &cobra.Command{
	Use:     "generate-skill",
	Aliases: []string{"skill"},
	Short:   "Generate machine-readable context for AI agents",
	Long:    `Create a skill.md file in ~/.kuargogo/ to help AI agents understand the cluster structure and available tools.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Generating AI Skill Context...")
		path, err := ai.GenerateSkillMD()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Printf("Successfully generated skill context at: %s\n", path)
	},
}

var aiExplainCmd = &cobra.Command{
	Use:     "explain [text]",
	Aliases: []string{"analyze"},
	Short:   "Explain a log message or incident using AI",
	Long:    `Send a log snippet or error message to the AI for a Senior SRE-level analysis and solution suggestion.`,
	Run: func(cmd *cobra.Command, args []string) {
		var text string
		if len(args) > 0 {
			text = strings.Join(args, " ")
		} else {
			// Read from stdin
			scanner := bufio.NewScanner(os.Stdin)
			var sb strings.Builder
			for scanner.Scan() {
				sb.WriteString(scanner.Text() + "\n")
			}
			text = sb.String()
		}

		if strings.TrimSpace(text) == "" {
			fmt.Println("Error: No text provided for explanation.")
			return
		}

		client, err := getClient()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		prompt := fmt.Sprintf("You are a Senior SRE Expert. Analyze the following log snippet or infrastructure incident, identify the root cause, and suggest concise actionable solutions:\n\n%s", text)

		// Visual Feedback: AI Processing (Purple)
		mqttConfig := config.GetConfig().MQTT
		hwClient, hwErr := hardware.NewClient(mqttConfig.Broker, "kuargogo-explain", mqttConfig.Username, string(mqttConfig.Password), DryRun)
		if hwErr == nil {
			defer hwClient.Disconnect(250)
			_ = hwClient.SetColor("purple", "breathing", mqttConfig.TopicPrefix, "global")
			defer func() { _ = hwClient.SetColor("green", "static", mqttConfig.TopicPrefix, "global") }()
		}

		err = client.Generate(prompt)
		if err != nil {
			fmt.Printf("Error during AI analysis: %v\n", err)
		}
	},
}
