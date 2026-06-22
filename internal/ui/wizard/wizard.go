package wizard

import (
	"fmt"
	"io"
	"math/rand"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/DannyStrelok/kuargogo/internal/config"
	"github.com/DannyStrelok/kuargogo/internal/ui"
	"github.com/DannyStrelok/kuargogo/internal/ui/engine"
)

type saveSuccessMsg struct{}

type WizardModel struct {
	form        *huh.Form
	width       int
	confirmSave bool // Show final save dialog
	theme       engine.Theme

	// Form Bound Variables
	clusterName  string
	tgToken      string
	tgAdminID    string
	sshUser      string
	switchDriver string
	switchIP     string
	switchUser   string
	switchPass   string
	mqttBroker   string
	mqttTopic    string
	masterPass   string
	syncProvider string
	syncS3Url    string
	syncS3Bucket string
	syncS3Prefix string
	syncS3Access string
	syncS3Secret string
	syncS3Region string
}

func NewWizardModel(theme engine.Theme) *WizardModel {
	m := &WizardModel{
		theme:        theme,
		clusterName:  randomFunnyHomelabName(),
		sshUser:      "kgg-admin", // Default to the new kgg-admin user!
		mqttTopic:    "kgg",
		switchDriver: "none",
		syncProvider: "none",
	}

	// Build the single unified Form with structured groups
	m.form = huh.NewForm(
		// Group 1: General Info
		huh.NewGroup(
			huh.NewInput().
				Title("Cluster Context Name").
				Description("A unique identifier for this homelab environment").
				Validate(huh.ValidateNotEmpty()).
				Value(&m.clusterName),
			huh.NewInput().
				Title("Telegram Bot Token").
				Description("Optional: For system alerts and remote control (from @BotFather)").
				Value(&m.tgToken),
			huh.NewInput().
				Title("Telegram Admin ID").
				Description("Optional: Your numeric user ID (from @userinfobot)").
				Validate(func(s string) error {
					if s == "" {
						return nil
					}
					var id int
					_, err := fmt.Sscanf(s, "%d", &id)
					if err != nil {
						return fmt.Errorf("must be a number")
					}
					return nil
				}).
				Value(&m.tgAdminID),
		).Title("1. General Setup"),

		// Group 2: SSH Defaults
		huh.NewGroup(
			huh.NewInput().
				Title("Default SSH User").
				Description("Username for cluster operations").
				Validate(huh.ValidateNotEmpty()).
				Value(&m.sshUser),
		).Title("2. Node Connection"),

		// Group 3: Switch Control Option
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Enable Switch Management?").
				Description("Control smart switch features like VLANs or ports").
				Options(
					huh.NewOption("Disabled / Manual", "none"),
					huh.NewOption("TP-Link SG108E", "tplink"),
					huh.NewOption("Simulated (Testing)", "simulated"),
				).
				Value(&m.switchDriver),
		).Title("3. Switch Management Option"),

		// Group 3b: Switch Details (Conditional)
		huh.NewGroup(
			huh.NewInput().
				Title("Switch IP Address").
				Validate(func(s string) error {
					if m.switchDriver != "none" && s == "" {
						return fmt.Errorf("IP address is required for driver '%s'", m.switchDriver)
					}
					return nil
				}).
				Value(&m.switchIP),
			huh.NewInput().
				Title("Switch Username").
				Value(&m.switchUser),
			huh.NewInput().
				Title("Switch Password").
				EchoMode(huh.EchoModePassword).
				Value(&m.switchPass),
		).
			Title("3.1 Switch Credentials").
			WithHideFunc(func() bool { return m.switchDriver == "none" }),

		// Group 4: MQTT Telemetry
		huh.NewGroup(
			huh.NewInput().
				Title("MQTT Broker Host (Pi IP)").
				Description("Optional: IP of Mosquitto broker for telemetry and WoL").
				Value(&m.mqttBroker),
			huh.NewInput().
				Title("MQTT Topic Prefix").
				Description("Topic suffix for internal alerts").
				Value(&m.mqttTopic),
		).Title("4. Hardware & Telemetry"),

		// Group 5: Vault Passphrase
		huh.NewGroup(
			huh.NewInput().
				Title("Master Passphrase").
				Description("Optional: Encrypts secrets (API tokens/passwords) at rest in kuargogo.yaml").
				EchoMode(huh.EchoModePassword).
				Value(&m.masterPass),
		).
			Title("5. Security & Vault").
			WithHideFunc(func() bool {
				pass, err := config.GetMasterKey()
				return err == nil && pass != ""
			}),

		// Group 6: Cloud Resilient Sync Option
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Enable Cloud Backup Sync?").
				Description("Upload encrypted configs automatically to S3/R2 compatible storage").
				Options(
					huh.NewOption("Disabled", "none"),
					huh.NewOption("Enabled (S3-Compatible)", "s3"),
				).
				Value(&m.syncProvider),
		).
			Title("6. Cloud Resilient Backup Option").
			WithHideFunc(func() bool {
				syncProv := config.RootConfigGetSync().Provider
				return syncProv != "" && syncProv != "none"
			}),

		// Group 6b: Cloud Backup Details (Conditional)
		huh.NewGroup(
			huh.NewInput().
				Title("S3 Endpoint URL").
				Description("e.g. https://<account_id>.r2.cloudflarestorage.com").
				Validate(func(s string) error {
					if m.syncProvider == "s3" && s == "" {
						return fmt.Errorf("endpoint is required for cloud sync")
					}
					return nil
				}).
				Value(&m.syncS3Url),
			huh.NewInput().
				Title("S3 Bucket Name").
				Validate(func(s string) error {
					if m.syncProvider == "s3" && s == "" {
						return fmt.Errorf("bucket name is required")
					}
					return nil
				}).
				Value(&m.syncS3Bucket),
			huh.NewInput().
				Title("S3 Folder Prefix").
				Description("Optional: e.g., 'homelab-kgg/'").
				Value(&m.syncS3Prefix),
			huh.NewInput().
				Title("S3 Access Key ID").
				Validate(func(s string) error {
					if m.syncProvider == "s3" && s == "" {
						return fmt.Errorf("access key is required")
					}
					return nil
				}).
				Value(&m.syncS3Access),
			huh.NewInput().
				Title("S3 Secret Access Key").
				EchoMode(huh.EchoModePassword).
				Validate(func(s string) error {
					if m.syncProvider == "s3" && s == "" {
						return fmt.Errorf("secret access key is required")
					}
					return nil
				}).
				Value(&m.syncS3Secret),
			huh.NewInput().
				Title("S3 Region").
				Description("Use 'auto' for Cloudflare R2").
				Value(&m.syncS3Region),
		).
			Title("6.1 S3 Credentials").
			WithHideFunc(func() bool {
				syncProv := config.RootConfigGetSync().Provider
				if syncProv != "" && syncProv != "none" {
					return true
				}
				return m.syncProvider == "none"
			}),
	)

	return m
}

func (m *WizardModel) Init() tea.Cmd {
	return m.form.Init()
}

func (m *WizardModel) ApplyTheme(t engine.Theme) {
	m.theme = t
}

func (m *WizardModel) MouseMode() tea.MouseMode {
	return tea.MouseModeNone
}

func (m *WizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case saveSuccessMsg:
		return m, tea.Quit

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.form.WithWidth(msg.Width - 10)
		newForm, cmd := m.form.Update(msg)
		if f, ok := newForm.(*huh.Form); ok {
			m.form = f
		}
		return m, cmd

	case tea.KeyPressMsg:
		if m.confirmSave {
			switch msg.String() {
			case "y", "Y", "enter":
				m.confirmSave = false
				return m, m.save()
			case "n", "N", "esc":
				m.confirmSave = false
				m.form.State = huh.StateNormal
				return m, nil
			}
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			return m, engine.Pop()
		}
	}

	var cmd tea.Cmd
	newForm, cmd := m.form.Update(msg)
	if f, ok := newForm.(*huh.Form); ok {
		m.form = f

		// Trigger save confirmation if the entire form completes
		if f.State == huh.StateCompleted {
			if m.validate() {
				m.confirmSave = true
			} else {
				// Revert form state to continue editing if validation failed
				f.State = huh.StateNormal
			}
		}
	}

	return m, cmd
}

func (m *WizardModel) validate() bool {
	if m.clusterName == "" || m.sshUser == "" {
		return false
	}
	if m.switchDriver != "none" && m.switchIP == "" {
		return false
	}
	if m.syncProvider == "s3" {
		if m.syncS3Url == "" || m.syncS3Bucket == "" || m.syncS3Access == "" || m.syncS3Secret == "" {
			return false
		}
	}
	return true
}

func (m *WizardModel) save() tea.Cmd {
	return func() tea.Msg {
		cfg := config.ClusterConfig{
			Telegram: config.Telegram{
				BotToken: config.Secret(m.tgToken),
			},
			Network: config.Network{
				SwitchIP: m.switchIP,
				Driver:   m.switchDriver,
				User:     m.switchUser,
				Password: config.Secret(m.switchPass),
			},
			MQTT: config.MQTT{
				Broker:      m.mqttBroker,
				TopicPrefix: m.mqttTopic,
			},
			SSH: config.SSH{
				PrivateKeyPath: fmt.Sprintf("~/.ssh/kgg_%s_id", m.clusterName),
				Port:           22,
			},
		}

		if m.tgAdminID != "" {
			_, _ = fmt.Sscanf(m.tgAdminID, "%d", &cfg.Telegram.AdminID)
		}

		config.AddContext(m.clusterName, cfg)

		if m.syncProvider == "s3" {
			config.RootConfigSetS3(config.Backup{
				S3Url:       m.syncS3Url,
				S3Bucket:    m.syncS3Bucket,
				S3Prefix:    m.syncS3Prefix,
				S3AccessKey: config.Secret(m.syncS3Access),
				S3SecretKey: config.Secret(m.syncS3Secret),
				S3Region:    m.syncS3Region,
			})
		}

		if m.masterPass != "" {
			if err := config.UnlockConfig(m.masterPass); err != nil {
				return engine.ErrorMsg{Err: err}
			}
		}

		if err := config.SaveConfig(); err != nil {
			return engine.ErrorMsg{Err: err}
		}
		return saveSuccessMsg{}
	}
}

func (m *WizardModel) View() tea.View {
	docBorderColor := m.theme.AccentColor()
	if m.form.State == huh.StateCompleted || m.confirmSave {
		docBorderColor = m.theme.SuccessColor()
	}

	docStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder(), true, true, true, true).
		BorderForeground(docBorderColor).
		Background(m.theme.SurfaceColor()).
		Padding(1, 4).
		Width(m.width - 6)

	formView := ""
	if m.confirmSave {
		successText := lipgloss.NewStyle().Foreground(m.theme.SuccessColor()).Bold(true).Render("✓ SETUP VALIDADO")
		promptText := lipgloss.NewStyle().Foreground(m.theme.PrimaryColor()).Bold(true).Render("¿Deseas guardar y aplicar esta configuración en kuargogo.yaml?")
		optionsText := lipgloss.NewStyle().Foreground(m.theme.MutedColor()).Render("(enter / y) Guardar y Aplicar  |  (esc / n) Cancelar y Editar")

		formView = fmt.Sprintf("\n\n  %s\n\n  %s\n\n  %s\n\n", successText, promptText, optionsText)
	} else {
		formView = m.form.View()
	}

	paperContent := docStyle.Render(formView)

	footerStyle := lipgloss.NewStyle().Foreground(m.theme.MutedColor()).Padding(1, 2)
	footer := footerStyle.Render("Tab / Shift+Tab: Moverse entre campos | Enter: Confirmar / Siguiente Sección | Esc: Volver")

	content := lipgloss.JoinVertical(lipgloss.Left,
		"\n  "+lipgloss.NewStyle().Foreground(m.theme.AccentColor()).Bold(true).Render("🛠️ KUARGOGO SETUP WIZARD")+"\n",
		paperContent,
		footer,
	)

	return tea.NewView(content)
}

func (m *WizardModel) Title() string         { return "Setup Wizard" }
func (m *WizardModel) Icon() string          { return "🛠️" }
func (m *WizardModel) ShowBreadcrumbs() bool { return true }

func Run(out io.Writer) error {
	theme := &ui.KGGTheme{}
	theme.SetDark(true) // Maintain dark theme for elite look
	wModel := NewWizardModel(theme)
	uiEngine := engine.NewEngine(wModel, theme, "kuargogo")

	p := tea.NewProgram(uiEngine)
	_, err := p.Run()
	return err
}

func randomFunnyHomelabName() string {
	adjectives := []string{
		"sleepy", "angry", "happy", "sad", "furioso", "overengineered", "unstable", "quantum",
		"cursed", "haunted", "noisy", "redundant", "self-hosted",
		"dockerized", "guapo", "kubernetes", "chistoso", "baremetal", "laggy", "epic", "perezoso",
	}

	nouns := []string{
		"potato", "cluster", "torero", "arguiñano", "server", "lab", "rack",
		"node", "instance", "daemon", "cebollero", "container", "router",
		"gateway", "proxy", "cache", "vm", "firewall", "caja",
	}

	suffixes := []string{
		"of-doom", "from-hell", "del-infierno", "3000", "mk2", "deluxe",
		"ultimate", "reborn", "original", "v2", "x", "prime",
		"edition", "lab", "serrano", "core", "edge", "hub", "edition",
	}

	return fmt.Sprintf(
		"the-%s-%s-%s",
		adjectives[rand.Intn(len(adjectives))],
		nouns[rand.Intn(len(nouns))],
		suffixes[rand.Intn(len(suffixes))],
	)
}
