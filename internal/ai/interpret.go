package ai

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Intent represents a classified user action from natural language.
type Intent struct {
	Action string            `json:"action"` // e.g., "health", "reboot", "shutdown", "wol", "status", "logs", "info"
	Target string            `json:"target"` // e.g., "node-1", "master", "all"
	Params map[string]string `json:"params"` // e.g., {"service": "kgg-agent"}
}

// Interpret uses the AI client to convert a natural language message into a structured Intent.
func Interpret(client Client, message string) (*Intent, error) {
	systemPrompt := `You are an intent classifier for 'kggargogo', a homelab management tool.
Convert the user's natural language request into a strictly formatted JSON object.
Allowed Actions:
- "health": General cluster health check or "how is everything".
- "status": Quick status of nodes or hardware.
- "reboot": Reboot a specific node.
- "shutdown": Power off a specific node.
- "wol": Wake up a node via Wake-on-LAN.
- "logs": Check logs for a node or service.
- "info": Get hardware information for a node.
- "unknown": If the request doesn't match any above.

Rules:
1. Output ONLY a valid JSON object. No preamble, no explanation.
2. The JSON must have: "action", "target", and "params" (object).
3. If no target is specified, use "all" or empty string as appropriate.
4. If the user asks for a specific node, put its name in "target".

Example Input: "Reboot master-1"
Example Output: {"action": "reboot", "target": "master-1", "params": {}}

Example Input: "How is the health of the cluster?"
Example Output: {"action": "health", "target": "all", "params": {}}`

	fullPrompt := fmt.Sprintf("%s\n\nUser Message: \"%s\"\n\nJSON Intent:", systemPrompt, message)

	var aiOutput strings.Builder
	client.SetOutput(&aiOutput)

	if err := client.Generate(fullPrompt); err != nil {
		return nil, fmt.Errorf("AI generation failed: %w", err)
	}

	// Extract JSON from potential markdown blocks
	rawJSON := aiOutput.String()
	rawJSON = strings.TrimSpace(rawJSON)
	rawJSON = strings.TrimPrefix(rawJSON, "```json")
	rawJSON = strings.TrimPrefix(rawJSON, "```")
	rawJSON = strings.TrimSuffix(rawJSON, "```")
	rawJSON = strings.TrimSpace(rawJSON)

	var intent Intent
	if err := json.Unmarshal([]byte(rawJSON), &intent); err != nil {
		return nil, fmt.Errorf("failed to parse AI intent JSON: %w (raw: %s)", err, rawJSON)
	}

	return &intent, nil
}
