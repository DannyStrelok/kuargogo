package notify

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/DannyStrelok/kuargogo/internal/ansible"
	"github.com/DannyStrelok/kuargogo/internal/config"
)

// TelegramNotifier sends messages via Telegram Bot API.
type TelegramNotifier struct {
	BotToken string
	AdminID  int
	Output   io.Writer
	DryRun   bool
}

// NewTelegramNotifier creates a notifier from active config.
func NewTelegramNotifier() *TelegramNotifier {
	cfg := config.GetConfig()
	return &TelegramNotifier{
		BotToken: string(cfg.Telegram.BotToken),
		AdminID:  cfg.Telegram.AdminID,
		Output:   os.Stdout,
	}
}

// IsConfigured returns true if Telegram credentials are set.
func (t *TelegramNotifier) IsConfigured() bool {
	return t.BotToken != "" && t.AdminID != 0
}

// SendMessage sends a text message to the configured admin.
func (t *TelegramNotifier) SendMessage(message string) error {
	if !t.IsConfigured() {
		return fmt.Errorf("telegram not configured: set bot_token and admin_id in config")
	}

	if t.DryRun {
		if _, err := fmt.Fprintln(t.Output, "[DRY RUN] Would send Telegram message:"); err != nil {
			return fmt.Errorf("failed to write to output: %w", err)
		}
		if _, err := fmt.Fprintln(t.Output, message); err != nil {
			return fmt.Errorf("failed to write to output: %w", err)
		}
		return nil
	}

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.BotToken)

	resp, err := http.PostForm(apiURL, url.Values{
		"chat_id":    {fmt.Sprintf("%d", t.AdminID)},
		"text":       {message},
		"parse_mode": {"Markdown"},
	})
	if err != nil {
		return fmt.Errorf("telegram request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API returned status %d", resp.StatusCode)
	}

	return nil
}

// NotifyAnsibleResult formats and sends an Ansible execution result.
func (t *TelegramNotifier) NotifyAnsibleResult(result *ansible.Result) error {
	status := "✅ SUCCESS"
	if !result.Success {
		status = "❌ FAILED"
	}

	message := fmt.Sprintf(
		"*🔧 Kuargogo Ansible Report*\n\n"+
			"📋 *Playbook:* `%s`\n"+
			"🏁 *Status:* %s\n"+
			"⏱️ *Duration:* %s\n"+
			"🕐 *Time:* %s",
		result.Playbook,
		status,
		result.Duration.Round(time.Second),
		time.Now().Format("15:04:05 02-Jan-2006"),
	)

	if !result.Success && result.Stderr != "" {
		// Truncate stderr if too long for Telegram
		stderr := result.Stderr
		if len(stderr) > 500 {
			stderr = stderr[:500] + "..."
		}
		message += fmt.Sprintf("\n\n*Error:*\n```\n%s\n```", stderr)
	}

	return t.SendMessage(message)
}

// NotifySimple sends a simple status notification.
func (t *TelegramNotifier) NotifySimple(title string, success bool, details string) error {
	status := "✅"
	if !success {
		status = "❌"
	}

	message := fmt.Sprintf(
		"*%s %s*\n\n%s\n\n🕐 %s",
		status,
		title,
		details,
		time.Now().Format("15:04:05"),
	)

	return t.SendMessage(message)
}
