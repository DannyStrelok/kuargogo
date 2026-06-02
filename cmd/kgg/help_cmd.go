package main

import (
	"fmt"
	"os"

	"charm.land/glamour/v2"
	"github.com/DannyStrelok/kuargogo/internal/help"
	"github.com/spf13/cobra"
)

var helpCmd = &cobra.Command{
	Use:   "help [topic]",
	Short: "Show documentation and guides",
	Long:  `Show documentation, guides, and reference material for the Kuargogo.`,
	Run: func(cmd *cobra.Command, args []string) {
		svc := help.NewService()

		if len(args) == 0 {
			// List topics
			fmt.Println("Available Help Topics:")
			fmt.Println("----------------------")
			for _, t := range svc.GetTopics() {
				fmt.Printf("%-20s %s\n", t.ID, t.Description)
			}
			fmt.Println("\nUsage: kgg help <topic_id>")
			return
		}

		topicID := args[0]
		content, err := svc.GetContent(topicID)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		// Render Markdown
		if isTerminal() {
			out, err := glamour.Render(content, "dark")
			if err != nil {
				// Fallback to plain text if render fails
				fmt.Println(content)
			} else {
				fmt.Print(out)
			}
		} else {
			fmt.Println(content)
		}
	},
}

func init() {
	rootCmd.AddCommand(helpCmd)
}

func isTerminal() bool {
	o, _ := os.Stdout.Stat()
	return (o.Mode() & os.ModeCharDevice) == os.ModeCharDevice
}
