package bot

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/devinjeon/claude-discord-bot/internal/imaging"
	"github.com/devinjeon/claude-discord-bot/internal/tmux"
)

// SlashCommands defines all available slash commands.
var SlashCommands = []*discordgo.ApplicationCommand{
	{
		Name:        "claude-now",
		Description: "Show current terminal output of the Claude tmux session",
	},
	{
		Name:        "claude-screenshot",
		Description: "Take a system screenshot and send it",
	},
	{
		Name:        "claude-restart",
		Description: "Fully reset and restart the Claude tmux session",
	},
	{
		Name:        "claude-usage",
		Description: "Check Claude Code usage stats",
	},
	{
		Name:        "claude-export",
		Description: "Run /export in the Claude tmux session",
	},
	{
		Name:        "claude-model",
		Description: "Run /model in the Claude tmux session",
	},
	{
		Name:        "claude-login",
		Description: "Run /login in the Claude tmux session",
	},
	{
		Name:        "claude-logout",
		Description: "Run /logout in the Claude tmux session",
	},
	{
		Name:        "claude-compact",
		Description: "Run /compact in the Claude tmux session",
	},
	{
		Name:        "claude-clear",
		Description: "Run /clear in the Claude tmux session",
	},
	{
		Name:        "claude-skills",
		Description: "Run /skills in the Claude tmux session",
	},
	{
		Name:        "claude-sendkey",
		Description: "Send a key or text to the Claude tmux session",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "key",
				Description:  "Key to send (1-9, Escape, Enter, or any text)",
				Required:     true,
				Autocomplete: true,
			},
		},
	},
}

// CommandHandler provides command implementations.
type CommandHandler struct {
	Tmux      tmux.Config
	ChannelID string
}

// SendDelayedNow waits 1 second, then captures and sends the current tmux output.
func (h *CommandHandler) SendDelayedNow(s *discordgo.Session) {
	time.Sleep(1 * time.Second)
	output, err := h.Tmux.CapturePane()
	if err != nil {
		return
	}
	if output == "" {
		output = "(empty)"
	}
	s.ChannelMessageSend(h.ChannelID, formatCodeBlock(output, 1900))
}

// HandleNow captures and sends the current tmux pane output.
func (h *CommandHandler) HandleNow(s *discordgo.Session) {
	output, err := h.Tmux.CapturePane()
	if err != nil {
		s.ChannelMessageSend(h.ChannelID, fmt.Sprintf("tmux capture failed: %v", err))
		return
	}
	if output == "" {
		output = "(empty)"
	}
	s.ChannelMessageSend(h.ChannelID, formatCodeBlock(output, 1900))
}

// HandleScreenshot takes a screenshot and sends it.
func (h *CommandHandler) HandleScreenshot(s *discordgo.Session) {
	jpegFile, cleanup, err := imaging.CaptureScreenshot(70)
	if err != nil {
		s.ChannelMessageSend(h.ChannelID, fmt.Sprintf("screenshot failed: %v", err))
		return
	}
	defer cleanup()

	f, err := os.Open(jpegFile)
	if err != nil {
		s.ChannelMessageSend(h.ChannelID, fmt.Sprintf("failed to open screenshot: %v", err))
		return
	}
	defer f.Close()

	if _, err := s.ChannelFileSend(h.ChannelID, "screenshot.jpg", f); err != nil {
		s.ChannelMessageSend(h.ChannelID, fmt.Sprintf("failed to send screenshot: %v", err))
		return
	}
	log.Println("[screenshot] sent successfully")
}

// HandleRestart kills the tmux session (LaunchAgent will restart it).
func (h *CommandHandler) HandleRestart(s *discordgo.Session) {
	log.Println("[restart] killing tmux session")
	s.ChannelMessageSend(h.ChannelID, "Restarting claude-channel session...")

	if err := h.Tmux.KillSession(); err != nil {
		log.Printf("[restart] kill-session failed: %v", err)
		s.ChannelMessageSend(h.ChannelID, fmt.Sprintf("Restart failed: %v", err))
		return
	}
	s.ChannelMessageSend(h.ChannelID, "claude-channel session terminated. LaunchAgent will restart it automatically.")
}

// HandleUsage sends /usage to tmux, captures the output, and sends it.
func (h *CommandHandler) HandleUsage(s *discordgo.Session) {
	log.Println("[usage] sending /usage to tmux")

	if err := h.Tmux.SendText("/usage"); err != nil {
		s.ChannelMessageSend(h.ChannelID, fmt.Sprintf("Failed to send usage command: %v", err))
		return
	}
	time.Sleep(100 * time.Millisecond)
	h.Tmux.SendKeys("Enter")

	output, err := h.waitForUsageOutput()
	if err != nil {
		s.ChannelMessageSend(h.ChannelID, fmt.Sprintf("tmux capture failed: %v", err))
		return
	}

	h.Tmux.SendKeys("Escape")

	trimmed := extractUsageBlock(output)
	s.ChannelMessageSend(h.ChannelID, formatCodeBlock(trimmed, 1900))
	log.Println("[usage] sent")
}

// HandleNowSlash handles the /claude-now slash command.
func (h *CommandHandler) HandleNowSlash(s *discordgo.Session, i *discordgo.Interaction) {
	output, err := h.Tmux.CapturePane()
	if err != nil {
		s.InteractionResponseEdit(i, &discordgo.WebhookEdit{Content: strPtr(fmt.Sprintf("tmux capture failed: %v", err))})
		return
	}
	if output == "" {
		output = "(empty)"
	}
	msg := formatCodeBlock(output, 1900)
	s.InteractionResponseEdit(i, &discordgo.WebhookEdit{Content: &msg})
}

// HandleScreenshotSlash handles the /claude-screenshot slash command.
func (h *CommandHandler) HandleScreenshotSlash(s *discordgo.Session, i *discordgo.Interaction) {
	jpegFile, cleanup, err := imaging.CaptureScreenshot(70)
	if err != nil {
		s.InteractionResponseEdit(i, &discordgo.WebhookEdit{Content: strPtr(fmt.Sprintf("screenshot failed: %v", err))})
		return
	}
	defer cleanup()

	f, err := os.Open(jpegFile)
	if err != nil {
		s.InteractionResponseEdit(i, &discordgo.WebhookEdit{Content: strPtr(fmt.Sprintf("failed to open: %v", err))})
		return
	}
	defer f.Close()

	s.InteractionResponseEdit(i, &discordgo.WebhookEdit{
		Content: strPtr("screenshot:"),
		Files:   []*discordgo.File{{Name: "screenshot.jpg", Reader: f}},
	})
	log.Println("[slash-screenshot] sent successfully")
}

// HandleRestartSlash handles the /claude-restart slash command.
func (h *CommandHandler) HandleRestartSlash(s *discordgo.Session, i *discordgo.Interaction) {
	log.Println("[slash-restart] killing tmux session")
	if err := h.Tmux.KillSession(); err != nil {
		s.InteractionResponseEdit(i, &discordgo.WebhookEdit{Content: strPtr(fmt.Sprintf("Restart failed: %v", err))})
		return
	}
	s.InteractionResponseEdit(i, &discordgo.WebhookEdit{
		Content: strPtr("claude-channel session terminated. LaunchAgent will restart it automatically."),
	})
}

// HandleUsageSlash handles the /claude-usage slash command.
func (h *CommandHandler) HandleUsageSlash(s *discordgo.Session, i *discordgo.Interaction) {
	log.Println("[slash-usage] sending /usage to tmux")

	if err := h.Tmux.SendText("/usage"); err != nil {
		s.InteractionResponseEdit(i, &discordgo.WebhookEdit{Content: strPtr(fmt.Sprintf("Failed to send usage command: %v", err))})
		return
	}
	time.Sleep(100 * time.Millisecond)
	h.Tmux.SendKeys("Enter")

	output, err := h.waitForUsageOutput()
	if err != nil {
		s.InteractionResponseEdit(i, &discordgo.WebhookEdit{Content: strPtr(fmt.Sprintf("Capture failed: %v", err))})
		return
	}

	h.Tmux.SendKeys("Escape")

	trimmed := extractUsageBlock(output)
	trimmed = strings.ReplaceAll(trimmed, "```", "` ` `")
	msg := fmt.Sprintf("```\n%s\n```", trimmed)
	if _, err := s.InteractionResponseEdit(i, &discordgo.WebhookEdit{Content: &msg}); err != nil {
		log.Printf("[slash-usage] edit error: %v", err)
	}
	log.Println("[slash-usage] sent")
}

// sendSlashCommand sends a slash command to the tmux session and reports the result.
func (h *CommandHandler) sendSlashCommand(name string, s *discordgo.Session) {
	log.Printf("[%s] sending /%s to tmux", name, name)
	if err := h.Tmux.SendText("/" + name); err != nil {
		s.ChannelMessageSend(h.ChannelID, fmt.Sprintf("Failed to send /%s: %v", name, err))
		return
	}
	time.Sleep(100 * time.Millisecond)
	h.Tmux.SendKeys("Enter")
	s.ChannelMessageSend(h.ChannelID, fmt.Sprintf("Sent /%s to Claude session.", name))
	log.Printf("[%s] sent", name)
}

// sendSlashCommandSlash is the slash-command variant of sendSlashCommand.
func (h *CommandHandler) sendSlashCommandSlash(name string, s *discordgo.Session, i *discordgo.Interaction) {
	log.Printf("[slash-%s] sending /%s to tmux", name, name)
	if err := h.Tmux.SendText("/" + name); err != nil {
		s.InteractionResponseEdit(i, &discordgo.WebhookEdit{Content: strPtr(fmt.Sprintf("Failed to send /%s: %v", name, err))})
		return
	}
	time.Sleep(100 * time.Millisecond)
	h.Tmux.SendKeys("Enter")
	s.InteractionResponseEdit(i, &discordgo.WebhookEdit{Content: strPtr(fmt.Sprintf("Sent /%s to Claude session.", name))})
	log.Printf("[slash-%s] sent", name)
}

func (h *CommandHandler) HandleExport(s *discordgo.Session)      { h.sendSlashCommand("export", s) }
func (h *CommandHandler) HandleModel(s *discordgo.Session)       { h.sendSlashCommand("model", s) }
func (h *CommandHandler) HandleLogin(s *discordgo.Session)       { h.sendSlashCommand("login", s) }
func (h *CommandHandler) HandleLogout(s *discordgo.Session)      { h.sendSlashCommand("logout", s) }
func (h *CommandHandler) HandleCompact(s *discordgo.Session)     { h.sendSlashCommand("compact", s) }
func (h *CommandHandler) HandleClear(s *discordgo.Session)       { h.sendSlashCommand("clear", s) }
func (h *CommandHandler) HandleSkills(s *discordgo.Session) {
	log.Println("[skills] sending /skills to tmux")

	if err := h.Tmux.SendText("/skills"); err != nil {
		s.ChannelMessageSend(h.ChannelID, fmt.Sprintf("Failed to send skills command: %v", err))
		return
	}
	time.Sleep(100 * time.Millisecond)
	h.Tmux.SendKeys("Enter")
	time.Sleep(2 * time.Second)

	output, err := h.Tmux.CapturePane()
	if err != nil {
		s.ChannelMessageSend(h.ChannelID, fmt.Sprintf("tmux capture failed: %v", err))
		return
	}

	h.Tmux.SendKeys("Escape")

	s.ChannelMessageSend(h.ChannelID, formatCodeBlock(output, 1900))
	log.Println("[skills] sent")
}

func (h *CommandHandler) HandleExportSlash(s *discordgo.Session, i *discordgo.Interaction)  { h.sendSlashCommandSlash("export", s, i) }
func (h *CommandHandler) HandleModelSlash(s *discordgo.Session, i *discordgo.Interaction)   { h.sendSlashCommandSlash("model", s, i) }
func (h *CommandHandler) HandleLoginSlash(s *discordgo.Session, i *discordgo.Interaction)   { h.sendSlashCommandSlash("login", s, i) }
func (h *CommandHandler) HandleLogoutSlash(s *discordgo.Session, i *discordgo.Interaction)  { h.sendSlashCommandSlash("logout", s, i) }
func (h *CommandHandler) HandleCompactSlash(s *discordgo.Session, i *discordgo.Interaction) { h.sendSlashCommandSlash("compact", s, i) }
func (h *CommandHandler) HandleClearSlash(s *discordgo.Session, i *discordgo.Interaction)   { h.sendSlashCommandSlash("clear", s, i) }
func (h *CommandHandler) HandleSkillsSlash(s *discordgo.Session, i *discordgo.Interaction) {
	log.Println("[slash-skills] sending /skills to tmux")

	if err := h.Tmux.SendText("/skills"); err != nil {
		s.InteractionResponseEdit(i, &discordgo.WebhookEdit{Content: strPtr(fmt.Sprintf("Failed to send skills command: %v", err))})
		return
	}
	time.Sleep(100 * time.Millisecond)
	h.Tmux.SendKeys("Enter")
	time.Sleep(2 * time.Second)

	output, err := h.Tmux.CapturePane()
	if err != nil {
		s.InteractionResponseEdit(i, &discordgo.WebhookEdit{Content: strPtr(fmt.Sprintf("Capture failed: %v", err))})
		return
	}

	h.Tmux.SendKeys("Escape")

	trimmed := strings.ReplaceAll(output, "```", "` ` `")
	msg := fmt.Sprintf("```\n%s\n```", trimmed)
	if _, err := s.InteractionResponseEdit(i, &discordgo.WebhookEdit{Content: &msg}); err != nil {
		log.Printf("[slash-skills] edit error: %v", err)
	}
	log.Println("[slash-skills] sent")
}

// sendkeyChoices are the predefined autocomplete suggestions for /claude-sendkey.
var sendkeyChoices = []*discordgo.ApplicationCommandOptionChoice{
	{Name: "1", Value: "1"},
	{Name: "2", Value: "2"},
	{Name: "3", Value: "3"},
	{Name: "4", Value: "4"},
	{Name: "5", Value: "5"},
	{Name: "6", Value: "6"},
	{Name: "7", Value: "7"},
	{Name: "8", Value: "8"},
	{Name: "9", Value: "9"},
	{Name: "Escape", Value: "Escape"},
	{Name: "Enter", Value: "Enter"},
	{Name: "Tab", Value: "Tab"},
	{Name: "Up", Value: "Up"},
	{Name: "Down", Value: "Down"},
}

// HandleSendkeyAutocomplete responds with filtered autocomplete suggestions.
func (h *CommandHandler) HandleSendkeyAutocomplete(s *discordgo.Session, i *discordgo.Interaction) {
	data := i.ApplicationCommandData()
	var query string
	for _, opt := range data.Options {
		if opt.Name == "key" {
			query = strings.ToLower(opt.StringValue())
		}
	}

	var filtered []*discordgo.ApplicationCommandOptionChoice
	for _, c := range sendkeyChoices {
		if query == "" || strings.Contains(strings.ToLower(c.Name), query) {
			filtered = append(filtered, c)
		}
	}

	s.InteractionRespond(i, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{Choices: filtered},
	})
}

// HandleSendkeySlash handles the /claude-sendkey slash command.
func (h *CommandHandler) HandleSendkeySlash(s *discordgo.Session, i *discordgo.Interaction) {
	data := i.ApplicationCommandData()
	var key string
	for _, opt := range data.Options {
		if opt.Name == "key" {
			key = opt.StringValue()
		}
	}

	log.Printf("[sendkey] key=%q", key)

	switch key {
	case "Escape", "Enter", "Tab", "Up", "Down":
		h.Tmux.SendKeys(key)
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		h.Tmux.SendKeys(key)
	default:
		// Freeform text input
		if err := h.Tmux.SendText(key); err != nil {
			s.InteractionResponseEdit(i, &discordgo.WebhookEdit{Content: strPtr(fmt.Sprintf("Failed to send text: %v", err))})
			return
		}
		time.Sleep(100 * time.Millisecond)
		h.Tmux.SendKeys("Enter")
	}

	s.InteractionResponseEdit(i, &discordgo.WebhookEdit{Content: strPtr(fmt.Sprintf("Sent key: %s", key))})
	go h.SendDelayedNow(s)
}

func strPtr(s string) *string { return &s }

func formatCodeBlock(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "```", "` ` `")
	return fmt.Sprintf("```\n%s\n```", truncate(s, maxLen))
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return "..." + s[len(s)-maxLen+3:]
}

// waitForUsageOutput polls tmux until usage data is loaded (contains "Current session")
// or times out after 10 seconds.
func (h *CommandHandler) waitForUsageOutput() (string, error) {
	const (
		pollInterval = 500 * time.Millisecond
		timeout      = 10 * time.Second
	)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		time.Sleep(pollInterval)
		output, err := h.Tmux.CapturePane()
		if err != nil {
			return "", err
		}
		if strings.Contains(output, "Current session") {
			return output, nil
		}
	}
	// Return whatever we have after timeout
	return h.Tmux.CapturePane()
}

// extractUsageBlock extracts the usage info block from tmux output.
func extractUsageBlock(output string) string {
	lines := strings.Split(output, "\n")
	startIdx := -1
	endIdx := len(lines)
	for i, line := range lines {
		if strings.Contains(line, "Current session") {
			startIdx = i
		}
		if strings.Contains(line, "Esc to cancel") {
			endIdx = i + 1
		}
	}
	if startIdx == -1 {
		return output
	}
	return strings.Join(lines[startIdx:endIdx], "\n")
}
