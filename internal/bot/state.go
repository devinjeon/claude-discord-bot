package bot

import (
	"log"
	"sync"
	"time"
)

// InteractionState tracks the current pending choice/text input state.
type InteractionState struct {
	mu              sync.Mutex
	active    bool
	isConfirm bool
	messageID       string
	numChoices      int
	lastDetectedSig string
	cooldownUntil   time.Time
}

// ClearChoice resets the active choice state and applies a cooldown.
func (s *InteractionState) ClearChoice() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active = false
	s.messageID = ""
	s.cooldownUntil = time.Now().Add(5 * time.Second)
}

// ClearActive clears the active state without cooldown (e.g., when prompt disappears).
func (s *InteractionState) ClearActive() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active = false
	s.messageID = ""
	s.lastDetectedSig = ""
}

// ClearIfActive clears the active state and signature only if currently active.
// This avoids the TOCTOU race of checking active outside the lock.
func (s *InteractionState) ClearIfActive() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active {
		s.active = false
		s.messageID = ""
		log.Println("[poll] choice prompt disappeared, clearing state")
	}
	s.lastDetectedSig = ""
}

// IsActiveMessage checks if the given message is the current active choice message.
func (s *InteractionState) IsActiveMessage(messageID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active && s.messageID == messageID
}

// IsConfirm returns true if the current active prompt is a confirm prompt.
func (s *InteractionState) IsConfirm() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active && s.isConfirm
}

// InCooldown returns true if we're within a cooldown period.
func (s *InteractionState) InCooldown() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return time.Now().Before(s.cooldownUntil)
}
