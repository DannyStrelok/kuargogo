package ai

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"

	"github.com/DannyStrelok/kuargogo/internal/config"
)

// Client defines the interface for AI providers
type Client interface {
	Generate(prompt string) error
	ListRunning() ([]RunningModel, error)
	Pull(model string, onProgress func(string, int64, int64)) error
	SetOutput(w io.Writer)
}

// RunningModel represents a model loaded in memory
type RunningModel struct {
	Name     string `json:"name"`
	SizeVRAM int64  `json:"size_vram"`
}

// NewClient returns the appropriate AI client based on configuration
func NewClient(cfg config.AIConfig, dryRun bool) (Client, error) {
	switch cfg.Provider {
	case "ollama":
		endpoint := cfg.Endpoint
		if endpoint == "" {
			// Fallback to auto-discovery if endpoint is missing for Ollama
			gpuNode := config.FindGPUNodes()
			if len(gpuNode) > 0 {
				endpoint = fmt.Sprintf("http://%s:11434", gpuNode[0].IP)
			} else {
				return nil, fmt.Errorf("ollama provider selected but no endpoint configured and no GPU node found")
			}
		}
		return &OllamaClient{
			BaseURL:       endpoint,
			Model:         cfg.Model,
			AnonymizeLogs: cfg.AnonymizeLogs,
			DryRun:        dryRun,
			Output:        os.Stdout,
		}, nil
	case "openai-compatible", "openai":
		client := &OpenAIClient{
			BaseURL:       cfg.Endpoint,
			APIKey:        string(cfg.APIKey),
			Model:         cfg.Model,
			AnonymizeLogs: cfg.AnonymizeLogs,
			DryRun:        dryRun,
			Output:        os.Stdout,
		}
		return client, nil
	default:
		return nil, fmt.Errorf("unsupported AI provider: %s", cfg.Provider)
	}
}

// OllamaClient handles interaction with Ollama
type OllamaClient struct {
	BaseURL       string
	Model         string
	AnonymizeLogs bool
	DryRun        bool
	Output        io.Writer
}

func (c *OllamaClient) Generate(prompt string) error {
	if c.AnonymizeLogs {
		prompt = Anonymize(prompt)
	}

	if c.DryRun {
		_, _ = fmt.Fprintf(c.Output, "[DRY-RUN] Sending prompt to %s (Ollama - Model: %s):\n%s\n", c.BaseURL, c.Model, prompt)
		_, _ = fmt.Fprintln(c.Output, "\n[DRY-RUN] Response: (Simulated Ollama response)")
		return nil
	}

	reqBody := struct {
		Model  string `json:"model"`
		Prompt string `json:"prompt"`
	}{
		Model:  c.Model,
		Prompt: prompt,
	}
	jsonData, _ := json.Marshal(reqBody)

	resp, err := http.Post(c.BaseURL+"/api/generate", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to connect to Ollama: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		var chunk struct {
			Response string `json:"response"`
			Done     bool   `json:"done"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &chunk); err != nil {
			continue
		}
		_, _ = fmt.Fprint(c.Output, chunk.Response)
		if chunk.Done {
			_, _ = fmt.Fprintln(c.Output)
		}
	}
	return scanner.Err()
}

func (c *OllamaClient) SetOutput(w io.Writer) {
	c.Output = w
}

func (c *OllamaClient) ListRunning() ([]RunningModel, error) {
	if c.DryRun {
		return []RunningModel{{Name: c.Model, SizeVRAM: 4000000000}}, nil
	}
	resp, err := http.Get(c.BaseURL + "/api/ps")
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Models []RunningModel `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Models, nil
}

func (c *OllamaClient) Pull(model string, onProgress func(string, int64, int64)) error {
	if c.DryRun {
		if onProgress != nil {
			onProgress("success", 100, 100)
		}
		return nil
	}
	reqBody := struct {
		Name   string `json:"name"`
		Stream bool   `json:"stream"`
	}{Name: model, Stream: true}
	jsonData, _ := json.Marshal(reqBody)

	resp, err := http.Post(c.BaseURL+"/api/pull", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		var chunk struct {
			Status    string `json:"status"`
			Total     int64  `json:"total"`
			Completed int64  `json:"completed"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &chunk); err != nil {
			continue
		}
		if onProgress != nil {
			onProgress(chunk.Status, chunk.Total, chunk.Completed)
		}
	}
	return scanner.Err()
}

// OpenAIClient handles interaction with OpenAI-compatible APIs
type OpenAIClient struct {
	BaseURL       string
	APIKey        string
	Model         string
	AnonymizeLogs bool
	DryRun        bool
	Output        io.Writer
}

func (c *OpenAIClient) Generate(prompt string) error {
	if c.AnonymizeLogs {
		prompt = Anonymize(prompt)
	}

	if c.DryRun {
		_, _ = fmt.Fprintf(c.Output, "[DRY-RUN] Sending prompt to %s (OpenAI - Model: %s):\n%s\n", c.BaseURL, c.Model, prompt)
		return nil
	}

	// Basic OpenAI Chat Completion implementation
	reqBody := struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}{
		Model: c.Model,
		Messages: []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{{Role: "user", Content: prompt}},
	}
	jsonData, _ := json.Marshal(reqBody)

	url := c.BaseURL
	if url == "" {
		url = "https://api.openai.com/v1"
	}
	req, _ := http.NewRequest("POST", url+"/chat/completions", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}
	if result.Error.Message != "" {
		return fmt.Errorf("openai error: %s", result.Error.Message)
	}
	if len(result.Choices) > 0 {
		_, _ = fmt.Fprintln(c.Output, result.Choices[0].Message.Content)
	}
	return nil
}

func (c *OpenAIClient) SetOutput(w io.Writer) {
	c.Output = w
}

func (c *OpenAIClient) ListRunning() ([]RunningModel, error) {
	return nil, fmt.Errorf("ListRunning is not supported by OpenAI provider")
}

func (c *OpenAIClient) Pull(model string, onProgress func(string, int64, int64)) error {
	return fmt.Errorf("Pull is not supported by OpenAI provider")
}

func Anonymize(text string) string {
	// Simple IP regex for demonstration
	ipRegex := `\b(?:\d{1,3}\.){3}\d{1,3}\b`
	re := regexp.MustCompile(ipRegex)
	return re.ReplaceAllString(text, "[IP_ANONYMIZED]")
}

