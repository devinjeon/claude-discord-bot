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

		// Message-based commands (non-command messages are handled by claude-channel plugin)
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
		case "/claude-export":
			s.MessageReactionAdd(m.ChannelID, m.ID, "✅")
			b.cmd.HandleExport(s)
		case "/claude-model":
			s.MessageReactionAdd(m.ChannelID, m.ID, "✅")
			b.cmd.HandleModel(s)
		case "/claude-login":
			s.MessageReactionAdd(m.ChannelID, m.ID, "✅")
			b.cmd.HandleLogin(s)
		case "/claude-logout":
			s.MessageReactionAdd(m.ChannelID, m.ID, "✅")
			b.cmd.HandleLogout(s)
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
			s.ChannelMessageSend(b.cfg.ChannelID, "Sent Esc (cancelled)")
			go b.cmd.SendDelayedNow(s)
			return
		}

		if r.Emoji.Name == enterEmoji && b.state.IsConfirm() {
			log.Println("[choice] user selected Enter (confirm)")
			b.state.ClearChoice()
			b.cfg.Tmux.SendKeys("Enter")
			s.ChannelMessageSend(b.cfg.ChannelID, "Sent Enter (confirmed)")
			go b.cmd.SendDelayedNow(s)
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
		s.ChannelMessageSend(b.cfg.ChannelID, fmt.Sprintf("Selected option %d", selectedNum))
		go b.cmd.SendDelayedNow(s)
		go b.poller.CheckForTextInput()
	})

	// Slash command interactions
	b.session.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.Type == discordgo.InteractionApplicationCommandAutocomplete {
			if i.ApplicationCommandData().Name == "claude-sendkey" {
				b.cmd.HandleSendkeyAutocomplete(s, i.Interaction)
			}
			return
		}

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

		case "claude-export":
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			})
			go b.cmd.HandleExportSlash(s, i.Interaction)

		case "claude-model":
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			})
			go b.cmd.HandleModelSlash(s, i.Interaction)

		case "claude-login":
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			})
			go b.cmd.HandleLoginSlash(s, i.Interaction)

		case "claude-logout":
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			})
			go b.cmd.HandleLogoutSlash(s, i.Interaction)

		case "claude-sendkey":
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			})
			go b.cmd.HandleSendkeySlash(s, i.Interaction)
		}
	})
}

func (b *Bot) registerSlashCommands() {
	// Clean up global commands
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

	// Bulk overwrite guild commands (single API call)
	registered, err := b.session.ApplicationCommandBulkOverwrite(b.session.State.User.ID, b.cfg.GuildID, SlashCommands)
	if err != nil {
		log.Printf("[slash] bulk overwrite failed: %v", err)
		return
	}
	for _, cmd := range registered {
		log.Printf("[slash] registered: /%s", cmd.Name)
	}
}
