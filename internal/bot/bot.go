package bot

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/devinjeon/claude-discord-bot/internal/tmux"
)

// InstanceConfig holds the configuration for a single channel-tmux instance.
type InstanceConfig struct {
	Name      string
	ChannelID string
	Tmux      tmux.Config
}

// Config holds the bot configuration.
type Config struct {
	Token        string
	GuildID      string
	Instances    []InstanceConfig
	PollInterval time.Duration
}

// Instance groups the per-channel state, command handler, and poller.
type Instance struct {
	Config InstanceConfig
	State  *InteractionState
	Cmd    *CommandHandler
	Poller *Poller
	cancel chan struct{} // per-instance cancel channel
}

// Bot is the main Discord bot instance.
type Bot struct {
	cfg       Config
	session   *discordgo.Session
	mu        sync.RWMutex
	instances map[string]*Instance // channelID -> Instance
}

// New creates a new Bot instance.
func New(cfg Config) (*Bot, error) {
	dg, err := discordgo.New("Bot " + cfg.Token)
	if err != nil {
		return nil, fmt.Errorf("create discord session: %w", err)
	}

	b := &Bot{
		cfg:       cfg,
		session:   dg,
		instances: make(map[string]*Instance),
	}

	for _, ic := range cfg.Instances {
		b.addInstance(ic, dg)
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

	log.Printf("claude-bot running (%d instances)", len(b.instances))

	b.mu.RLock()
	for _, inst := range b.instances {
		go inst.Poller.Run(inst.cancel)
	}
	b.mu.RUnlock()

	<-done
	log.Println("Shutting down")

	// Stop all instance pollers
	b.mu.RLock()
	for _, inst := range b.instances {
		select {
		case <-inst.cancel:
		default:
			close(inst.cancel)
		}
	}
	b.mu.RUnlock()

	return nil
}

func (b *Bot) getInstance(channelID string) *Instance {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.instances[channelID]
}

// addInstance creates and registers a new instance (caller must not hold b.mu).
func (b *Bot) addInstance(ic InstanceConfig, session *discordgo.Session) *Instance {
	state := &InteractionState{}
	cmd := &CommandHandler{
		Tmux:      ic.Tmux,
		ChannelID: ic.ChannelID,
	}
	cancel := make(chan struct{})
	poller := &Poller{
		Session:   session,
		Tmux:      ic.Tmux,
		State:     state,
		ChannelID: ic.ChannelID,
		Interval:  b.cfg.PollInterval,
	}
	inst := &Instance{
		Config: ic,
		State:  state,
		Cmd:    cmd,
		Poller: poller,
		cancel: cancel,
	}
	b.instances[ic.ChannelID] = inst
	log.Printf("[init] instance %q -> channel %s -> tmux %s", ic.Name, ic.ChannelID, ic.Tmux.Session)
	return inst
}

// Reload re-reads instance configs and updates the instances map.
// It stops pollers for removed instances and starts pollers for new ones.
func (b *Bot) Reload(newConfigs []InstanceConfig) {
	b.mu.Lock()

	// Find removed instances
	newSet := make(map[string]bool, len(newConfigs))
	for _, ic := range newConfigs {
		newSet[ic.ChannelID] = true
	}

	var toStop []*Instance
	for chID, inst := range b.instances {
		if !newSet[chID] {
			toStop = append(toStop, inst)
			delete(b.instances, chID)
		}
	}

	// Find new instances
	var toStart []*Instance
	for _, ic := range newConfigs {
		if _, exists := b.instances[ic.ChannelID]; !exists {
			inst := b.addInstance(ic, b.session)
			toStart = append(toStart, inst)
		}
	}

	b.mu.Unlock()

	// Stop removed instance pollers (outside lock)
	for _, inst := range toStop {
		log.Printf("[reload] removing instance %q (channel %s)", inst.Config.Name, inst.Config.ChannelID)
		close(inst.cancel)
	}

	// Start new instance pollers
	for _, inst := range toStart {
		log.Printf("[reload] adding instance %q (channel %s)", inst.Config.Name, inst.Config.ChannelID)
		go inst.Poller.Run(inst.cancel)
	}

	log.Printf("[reload] instances updated: %d active", len(b.instances))
}

func (b *Bot) registerHandlers() {
	// Text messages
	b.session.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		if m.Author.ID == s.State.User.ID {
			return
		}

		inst := b.getInstance(m.ChannelID)
		if inst == nil {
			return
		}

		log.Printf("[msg] channel=%s instance=%s author=%s content=%q", m.ChannelID, inst.Config.Name, m.Author.Username, m.Content)
		content := strings.TrimSpace(m.Content)

		switch content {
		case "/claude-now":
			s.MessageReactionAdd(m.ChannelID, m.ID, "\u2705")
			inst.Cmd.HandleNow(s)
		case "/claude-screenshot":
			s.MessageReactionAdd(m.ChannelID, m.ID, "\u2705")
			inst.Cmd.HandleScreenshot(s)
		case "/claude-restart":
			s.MessageReactionAdd(m.ChannelID, m.ID, "\u2705")
			inst.Cmd.HandleRestart(s)
		case "/claude-usage":
			s.MessageReactionAdd(m.ChannelID, m.ID, "\u2705")
			inst.Cmd.HandleUsage(s)
		case "/claude-export":
			s.MessageReactionAdd(m.ChannelID, m.ID, "\u2705")
			inst.Cmd.HandleExport(s)
		case "/claude-model":
			s.MessageReactionAdd(m.ChannelID, m.ID, "\u2705")
			inst.Cmd.HandleModel(s)
		case "/claude-login":
			s.MessageReactionAdd(m.ChannelID, m.ID, "\u2705")
			inst.Cmd.HandleLogin(s)
		case "/claude-logout":
			s.MessageReactionAdd(m.ChannelID, m.ID, "\u2705")
			inst.Cmd.HandleLogout(s)
		case "/claude-compact":
			s.MessageReactionAdd(m.ChannelID, m.ID, "\u2705")
			inst.Cmd.HandleCompact(s)
		case "/claude-clear":
			s.MessageReactionAdd(m.ChannelID, m.ID, "\u2705")
			inst.Cmd.HandleClear(s)
		case "/claude-skills":
			s.MessageReactionAdd(m.ChannelID, m.ID, "\u2705")
			inst.Cmd.HandleSkills(s)
		}
	})

	// Emoji reactions for choice selection
	b.session.AddHandler(func(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
		if r.UserID == s.State.User.ID {
			return
		}

		inst := b.getInstance(r.ChannelID)
		if inst == nil {
			return
		}

		if !inst.State.IsActiveMessage(r.MessageID) {
			return
		}

		if r.Emoji.Name == escEmoji {
			log.Printf("[choice] instance=%s user selected Esc (cancel)", inst.Config.Name)
			inst.State.ClearChoice()
			inst.Config.Tmux.SendKeys("Escape")
			s.ChannelMessageSend(inst.Config.ChannelID, "Sent Esc (cancelled)")
			go inst.Cmd.SendDelayedNow(s)
			return
		}

		if r.Emoji.Name == enterEmoji && inst.State.IsConfirm() {
			log.Printf("[choice] instance=%s user selected Enter (confirm)", inst.Config.Name)
			inst.State.ClearChoice()
			inst.Config.Tmux.SendKeys("Enter")
			s.ChannelMessageSend(inst.Config.ChannelID, "Sent Enter (confirmed)")
			go inst.Cmd.SendDelayedNow(s)
			return
		}

		selectedNum := -1
		for i, emoji := range numberEmojis {
			if r.Emoji.Name == emoji {
				selectedNum = i + 1
				break
			}
		}

		inst.State.mu.Lock()
		numChoices := inst.State.numChoices
		inst.State.mu.Unlock()

		if selectedNum < 1 || selectedNum > numChoices {
			return
		}

		log.Printf("[choice] instance=%s user selected option %d", inst.Config.Name, selectedNum)
		inst.State.ClearChoice()

		for i := 1; i < selectedNum; i++ {
			inst.Config.Tmux.SendKeys("Down")
			time.Sleep(100 * time.Millisecond)
		}
		inst.Config.Tmux.SendKeys("Enter")

		log.Printf("[choice] instance=%s sent keys for option %d (Down x%d + Enter)", inst.Config.Name, selectedNum, selectedNum-1)
		s.ChannelMessageSend(inst.Config.ChannelID, fmt.Sprintf("Selected option %d", selectedNum))
		go inst.Cmd.SendDelayedNow(s)
		go inst.Poller.CheckForTextInput()
	})

	// Slash command interactions
	b.session.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.Type == discordgo.InteractionApplicationCommandAutocomplete {
			// Autocomplete doesn't need instance routing — just respond with choices
			if i.ApplicationCommandData().Name == "claude-sendkey" {
				// Use any instance's handler (autocomplete is static)
				for _, inst := range b.instances {
					inst.Cmd.HandleSendkeyAutocomplete(s, i.Interaction)
					return
				}
			}
			return
		}

		if i.Type != discordgo.InteractionApplicationCommand {
			return
		}

		inst := b.getInstance(i.ChannelID)
		if inst == nil {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "This channel is not linked to a Claude instance.",
				},
			})
			return
		}

		log.Printf("[slash] instance=%s command=%s", inst.Config.Name, i.ApplicationCommandData().Name)

		switch i.ApplicationCommandData().Name {
		case "claude-now":
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			})
			inst.Cmd.HandleNowSlash(s, i.Interaction)

		case "claude-screenshot":
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			})
			go inst.Cmd.HandleScreenshotSlash(s, i.Interaction)

		case "claude-restart":
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			})
			go inst.Cmd.HandleRestartSlash(s, i.Interaction)

		case "claude-usage":
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			})
			go inst.Cmd.HandleUsageSlash(s, i.Interaction)

		case "claude-export":
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			})
			go inst.Cmd.HandleExportSlash(s, i.Interaction)

		case "claude-model":
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			})
			go inst.Cmd.HandleModelSlash(s, i.Interaction)

		case "claude-login":
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			})
			go inst.Cmd.HandleLoginSlash(s, i.Interaction)

		case "claude-logout":
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			})
			go inst.Cmd.HandleLogoutSlash(s, i.Interaction)

		case "claude-compact":
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			})
			go inst.Cmd.HandleCompactSlash(s, i.Interaction)

		case "claude-clear":
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			})
			go inst.Cmd.HandleClearSlash(s, i.Interaction)

		case "claude-skills":
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			})
			go inst.Cmd.HandleSkillsSlash(s, i.Interaction)

		case "claude-sendkey":
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			})
			go inst.Cmd.HandleSendkeySlash(s, i.Interaction)
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
