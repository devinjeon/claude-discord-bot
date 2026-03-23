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
		Description: "Claude tmux 세션의 현재 터미널 상태를 확인합니다",
	},
	{
		Name:        "claude-screenshot",
		Description: "시스템 스크린샷을 찍어서 전송합니다",
	},
	{
		Name:        "claude-restart",
		Description: "Claude tmux 세션을 재시작합니다",
	},
	{
		Name:        "claude-usage",
		Description: "Claude Code 사용량을 확인합니다",
	},
}

// CommandHandler provides command implementations.
type CommandHandler struct {
	Tmux      tmux.Config
	ChannelID string
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
	s.ChannelMessageSend(h.ChannelID, "claude-channel 세션을 재시작합니다...")

	if err := h.Tmux.KillSession(); err != nil {
		log.Printf("[restart] kill-session failed: %v", err)
		s.ChannelMessageSend(h.ChannelID, fmt.Sprintf("재시작 실패: %v", err))
		return
	}
	s.ChannelMessageSend(h.ChannelID, "claude-channel 세션 종료 완료. LaunchAgent가 자동으로 재시작합니다.")
}

// HandleUsage sends /usage to tmux, captures the output, and sends it.
func (h *CommandHandler) HandleUsage(s *discordgo.Session) {
	log.Println("[usage] sending /usage to tmux")

	if err := h.Tmux.SendText("/usage"); err != nil {
		s.ChannelMessageSend(h.ChannelID, fmt.Sprintf("usage 명령 전송 실패: %v", err))
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
		s.InteractionResponseEdit(i, &discordgo.WebhookEdit{Content: strPtr(fmt.Sprintf("재시작 실패: %v", err))})
		return
	}
	s.InteractionResponseEdit(i, &discordgo.WebhookEdit{
		Content: strPtr("claude-channel 세션 종료 완료. LaunchAgent가 자동으로 재시작합니다."),
	})
}

// HandleUsageSlash handles the /claude-usage slash command.
func (h *CommandHandler) HandleUsageSlash(s *discordgo.Session, i *discordgo.Interaction) {
	log.Println("[slash-usage] sending /usage to tmux")

	if err := h.Tmux.SendText("/usage"); err != nil {
		s.InteractionResponseEdit(i, &discordgo.WebhookEdit{Content: strPtr(fmt.Sprintf("usage 전송 실패: %v", err))})
		return
	}
	time.Sleep(100 * time.Millisecond)
	h.Tmux.SendKeys("Enter")
	time.Sleep(2 * time.Second)

	output, err := h.Tmux.CapturePane()
	if err != nil {
		s.InteractionResponseEdit(i, &discordgo.WebhookEdit{Content: strPtr(fmt.Sprintf("capture 실패: %v", err))})
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
