package bot

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/devinjeon/claude-discord-bot/internal/tmux"
)

// Config holds the bot configuration.
type Config struct {
	Token        string
	ChannelID    string
	GuildID      string
	Tmux         tmux.Config
	PollInterval time.Duration
}

// Bot is the main Discord bot instance.
type Bot struct {
	cfg     Config
	session *discordgo.Session
	state   *InteractionState
	cmd     *CommandHandler
	poller  *Poller
}

// New creates a new Bot instance.
func New(cfg Config) (*Bot, error) {
	dg, err := discordgo.New("Bot " + cfg.Token)
	if err != nil {
		return nil, fmt.Errorf("create discord session: %w", err)
	}

	state := &InteractionState{}
	cmd := &CommandHandler{
		Tmux:      cfg.Tmux,
		ChannelID: cfg.ChannelID,
	}

	b := &Bot{
		cfg:     cfg,
		session: dg,
		state:   state,
		cmd:     cmd,
	}

	b.poller = &Poller{
		Session:   dg,
		Tmux:      cfg.Tmux,
		State:     state,
		ChannelID: cfg.ChannelID,
		Interval:  cfg.PollInterval,
	}

	b.registerHandlers()
	return b, nil
}

// Run opens the Discord connection, registers commands, and blocks until done is closed.
func (b *Bot) Run(done <-chan struct{}) error {
	b.session.Identify.Intents = discordgo.IntentsGuildMessages |
		discordgo.IntentsMessageContent |
		discordgo.IntentsGuildMessageReactions

	if err := b.session.Open(); err != nil {
		return fmt.Errorf("connect to discord: %w", err)
	}
	defer b.session.Close()

	b.registerSlashCommands()

	log.Printf("claude-bot running (channel: %s)", b.cfg.ChannelID)

	go b.poller.Run(done)

	<-done
	log.Println("Shutting down")
	return nil
}

func (b *Bot) registerHandlers() {
	// Text messages
	b.session.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		if m.Author.ID == s.State.User.ID || m.ChannelID != b.cfg.ChannelID {
			return
		}

		log.Printf("[msg] author=%s content=%q", m.Author.Username, m.Content)
		content := strings.TrimSpace(m.Content)

		// Check if we're waiting for text input
		if b.state.CheckAndClearTextWait() {
			log.Printf("[text] received text input: %q", content)
			s.MessageReactionAdd(m.ChannelID, m.ID, "✅")

			b.cfg.Tmux.SendKeys("Tab")
			time.Sleep(200 * time.Millisecond)
			if err := b.cfg.Tmux.SendText(content); err != nil {
				log.Printf("[text] send text failed: %v", err)
				s.ChannelMessageSend(b.cfg.ChannelID, fmt.Sprintf("텍스트 입력 실패: %v", err))
				return
			}
			time.Sleep(100 * time.Millisecond)
			b.cfg.Tmux.SendKeys("Enter")

			log.Printf("[text] sent text + Enter")
			s.ChannelMessageSend(b.cfg.ChannelID, fmt.Sprintf("텍스트 입력 완료: %s", content))
			return
		}

		// Message-based commands
		switch content {
		case "/claude-now":
			s.MessageReactionAdd(m.ChannelID, m.ID, "✅")
			b.cmd.HandleNow(s)
		case "/claude-screenshot":
			s.MessageReactionAdd(m.ChannelID, m.ID, "✅")
			b.cmd.HandleScreenshot(s)
		case "/claude-restart":
			s.MessageReactionAdd(m.ChannelID, m.ID, "✅")
			b.cmd.HandleRestart(s)
		case "/claude-usage":
			s.MessageReactionAdd(m.ChannelID, m.ID, "✅")
			b.cmd.HandleUsage(s)
		}
	})

	// Emoji reactions for choice selection
	b.session.AddHandler(func(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
		if r.UserID == s.State.User.ID || r.ChannelID != b.cfg.ChannelID {
			return
		}

		if !b.state.IsActiveMessage(r.MessageID) {
			return
		}

		if r.Emoji.Name == escEmoji {
			log.Println("[choice] user selected Esc (cancel)")
			b.state.ClearChoice()
			b.cfg.Tmux.SendKeys("Escape")
			s.ChannelMessageSend(b.cfg.ChannelID, "Esc 전송 완료 (취소)")
			return
		}

		selectedNum := -1
		for i, emoji := range numberEmojis {
			if r.Emoji.Name == emoji {
				selectedNum = i + 1
				break
			}
		}

		b.state.mu.Lock()
		numChoices := b.state.numChoices
		b.state.mu.Unlock()

		if selectedNum < 1 || selectedNum > numChoices {
			return
		}

		log.Printf("[choice] user selected option %d", selectedNum)
		b.state.ClearChoice()

		for i := 1; i < selectedNum; i++ {
			b.cfg.Tmux.SendKeys("Down")
			time.Sleep(100 * time.Millisecond)
		}
		b.cfg.Tmux.SendKeys("Enter")

		log.Printf("[choice] sent keys for option %d (Down x%d + Enter)", selectedNum, selectedNum-1)
		s.ChannelMessageSend(b.cfg.ChannelID, fmt.Sprintf("Option %d 선택 완료", selectedNum))

		go b.poller.CheckForTextInput()
	})

	// Slash command interactions
	b.session.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.Type != discordgo.InteractionApplicationCommand {
			return
		}

		log.Printf("[slash] command=%s", i.ApplicationCommandData().Name)

		switch i.ApplicationCommandData().Name {
		case "claude-now":
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			})
			b.cmd.HandleNowSlash(s, i.Interaction)

		case "claude-screenshot":
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			})
			go b.cmd.HandleScreenshotSlash(s, i.Interaction)

		case "claude-restart":
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			})
			go b.cmd.HandleRestartSlash(s, i.Interaction)

		case "claude-usage":
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			})
			go b.cmd.HandleUsageSlash(s, i.Interaction)
		}
	})
}

func (b *Bot) registerSlashCommands() {
	// Remove old commands
	existingCmds, err := b.session.ApplicationCommands(b.session.State.User.ID, b.cfg.GuildID)
	if err != nil {
		log.Printf("[slash] failed to list guild commands: %v", err)
	}
	for _, cmd := range existingCmds {
		if err := b.session.ApplicationCommandDelete(b.session.State.User.ID, b.cfg.GuildID, cmd.ID); err != nil {
			log.Printf("[slash] failed to delete guild command /%s: %v", cmd.Name, err)
		} else {
			log.Printf("[slash] removed old guild command: /%s", cmd.Name)
		}
	}
	globalCmds, err := b.session.ApplicationCommands(b.session.State.User.ID, "")
	if err != nil {
		log.Printf("[slash] failed to list global commands: %v", err)
	}
	for _, cmd := range globalCmds {
		if err := b.session.ApplicationCommandDelete(b.session.State.User.ID, "", cmd.ID); err != nil {
			log.Printf("[slash] failed to delete global command /%s: %v", cmd.Name, err)
		} else {
			log.Printf("[slash] removed global command: /%s", cmd.Name)
		}
	}

	// Register new commands
	for _, cmd := range SlashCommands {
		if _, err := b.session.ApplicationCommandCreate(b.session.State.User.ID, b.cfg.GuildID, cmd); err != nil {
			log.Printf("[slash] failed to register /%s: %v", cmd.Name, err)
		} else {
			log.Printf("[slash] registered: /%s", cmd.Name)
		}
	}
}
