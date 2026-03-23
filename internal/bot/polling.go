package bot

import (
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/devinjeon/claude-discord-bot/internal/tmux"
)

var numberEmojis = []string{"1️⃣", "2️⃣", "3️⃣", "4️⃣", "5️⃣", "6️⃣", "7️⃣", "8️⃣", "9️⃣"}

const escEmoji = "❌"
const enterEmoji = "✅"

var (
	choicePattern    = regexp.MustCompile(`^\s*(?:[^\d\s]\s+)?(\d+)\.\s+(.+)`)
	footerPattern    = regexp.MustCompile(`Enter to select|to navigate|Esc to cancel|Enter to save|Enter to confirm`)
	confirmPattern   = regexp.MustCompile(`Enter to (save|confirm).*Esc to (go back|exit)`)
	textInputPattern = regexp.MustCompile(`Tab to amend|Esc to cancel|Enter to submit|Type your|Type a response|submit your`)
)

// Poller watches the tmux session for choice prompts and notifies Discord.
type Poller struct {
	Session   *discordgo.Session
	Tmux      tmux.Config
	State     *InteractionState
	ChannelID string
	Interval  time.Duration

	textCheckMu sync.Mutex
	textChecking bool
}

// Run starts the polling loop. It blocks until done is closed.
func (p *Poller) Run(done <-chan struct{}) {
	ticker := time.NewTicker(p.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			p.checkForChoicePrompt()
		}
	}
}

// CheckForTextInput checks if a text input prompt appeared after a choice selection.
// It guards against concurrent invocations.
func (p *Poller) CheckForTextInput() {
	p.textCheckMu.Lock()
	if p.textChecking {
		p.textCheckMu.Unlock()
		return
	}
	p.textChecking = true
	p.textCheckMu.Unlock()
	defer func() {
		p.textCheckMu.Lock()
		p.textChecking = false
		p.textCheckMu.Unlock()
	}()

	time.Sleep(1500 * time.Millisecond)

	output, err := p.Tmux.CapturePane()
	if err != nil {
		log.Printf("[text] capture failed: %v", err)
		return
	}

	lines := strings.Split(output, "\n")
	start := len(lines) - 5
	if start < 0 {
		start = 0
	}

	hasTextInput := false
	for i := start; i < len(lines); i++ {
		if textInputPattern.MatchString(lines[i]) {
			hasTextInput = true
			break
		}
	}

	if !hasTextInput {
		log.Println("[text] no text input detected after selection")
		return
	}

	log.Println("[text] text input detected, notifying user")

	sanitized := strings.ReplaceAll(output, "```", "` ` `")
	msg := fmt.Sprintf("```\n%s\n```\nText input required. Use `/claude-sendkey` to send text.", truncate(sanitized, 1850))
	p.Session.ChannelMessageSend(p.ChannelID, msg)
}

func (p *Poller) checkForChoicePrompt() {
	output, err := p.Tmux.CapturePane()
	if err != nil {
		return
	}

	lines := strings.Split(output, "\n")

	hasFooter := false
	footerIdx := -1
	startSearch := len(lines) - 5
	if startSearch < 0 {
		startSearch = 0
	}
	for i := startSearch; i < len(lines); i++ {
		if footerPattern.MatchString(lines[i]) {
			hasFooter = true
			footerIdx = i
			break
		}
	}

	if !hasFooter {
		p.State.ClearIfActive()
		return
	}

	if p.State.InCooldown() {
		return
	}

	var choices []string
	searchStart := footerIdx - 20
	if searchStart < 0 {
		searchStart = 0
	}
	for i := searchStart; i < footerIdx; i++ {
		if choicePattern.MatchString(lines[i]) {
			matches := choicePattern.FindStringSubmatch(lines[i])
			if len(matches) >= 3 {
				choices = append(choices, fmt.Sprintf("%s. %s", matches[1], matches[2]))
			}
		}
	}

	var sig string
	if len(choices) >= 2 {
		sig = strings.Join(choices, "|")
	} else {
		sig = "esc-only:" + lines[footerIdx]
	}

	p.State.mu.Lock()
	defer p.State.mu.Unlock()

	if p.State.lastDetectedSig == sig {
		return
	}

	sanitized := strings.ReplaceAll(output, "```", "` ` `")

	// Check if this is a confirm prompt (Enter to save · Esc to go back)
	isConfirm := confirmPattern.MatchString(lines[footerIdx])

	if len(choices) >= 2 {
		log.Printf("[poll] choice prompt detected: %d options", len(choices))
		msg := fmt.Sprintf("```\n%s\n```\nSelect an option by reacting with an emoji.", truncate(sanitized, 1850))

		sentMsg, err := p.Session.ChannelMessageSend(p.ChannelID, msg)
		if err != nil {
			log.Printf("[poll] failed to send choice message: %v", err)
			return
		}

		numChoices := len(choices)
		if numChoices > len(numberEmojis) {
			numChoices = len(numberEmojis)
		}
		for i := 0; i < numChoices; i++ {
			p.Session.MessageReactionAdd(p.ChannelID, sentMsg.ID, numberEmojis[i])
		}
		p.Session.MessageReactionAdd(p.ChannelID, sentMsg.ID, escEmoji)

		p.State.active = true
		p.State.messageID = sentMsg.ID
		p.State.numChoices = numChoices
		p.State.isConfirm = false
	} else if isConfirm {
		log.Println("[poll] confirm prompt detected (Enter to save / Esc to go back)")
		msg := fmt.Sprintf("```\n%s\n```\n✅ Enter (save) · ❌ Esc (go back)", truncate(sanitized, 1850))

		sentMsg, err := p.Session.ChannelMessageSend(p.ChannelID, msg)
		if err != nil {
			log.Printf("[poll] failed to send confirm message: %v", err)
			return
		}
		p.Session.MessageReactionAdd(p.ChannelID, sentMsg.ID, enterEmoji)
		p.Session.MessageReactionAdd(p.ChannelID, sentMsg.ID, escEmoji)

		p.State.active = true
		p.State.messageID = sentMsg.ID
		p.State.numChoices = 0
		p.State.isConfirm = true
	} else {
		log.Println("[poll] esc-only prompt detected")
		msg := fmt.Sprintf("```\n%s\n```\nWaiting for input. Press ❌ to cancel.", truncate(sanitized, 1850))

		sentMsg, err := p.Session.ChannelMessageSend(p.ChannelID, msg)
		if err != nil {
			log.Printf("[poll] failed to send esc message: %v", err)
			return
		}
		p.Session.MessageReactionAdd(p.ChannelID, sentMsg.ID, escEmoji)

		p.State.active = true
		p.State.messageID = sentMsg.ID
		p.State.numChoices = 0
		p.State.isConfirm = false
	}

	p.State.lastDetectedSig = sig
}
