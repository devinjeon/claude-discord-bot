package bot

import (
	"sync"
	"testing"
	"time"
)

func TestClearChoice(t *testing.T) {
	s := &InteractionState{}
	s.mu.Lock()
	s.active = true
	s.messageID = "msg123"
	s.mu.Unlock()

	s.ClearChoice()

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.active {
		t.Error("expected active to be false")
	}
	if s.messageID != "" {
		t.Error("expected messageID to be empty")
	}
	if s.cooldownUntil.IsZero() {
		t.Error("expected cooldownUntil to be set")
	}
}

func TestClearActive(t *testing.T) {
	s := &InteractionState{}
	s.mu.Lock()
	s.active = true
	s.messageID = "msg123"
	s.lastDetectedSig = "sig"
	s.mu.Unlock()

	s.ClearActive()

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.active {
		t.Error("expected active to be false")
	}
	if s.messageID != "" {
		t.Error("expected messageID to be empty")
	}
	if s.lastDetectedSig != "" {
		t.Error("expected lastDetectedSig to be empty")
	}
}

func TestClearIfActive(t *testing.T) {
	// When active
	s := &InteractionState{}
	s.mu.Lock()
	s.active = true
	s.messageID = "msg123"
	s.lastDetectedSig = "sig"
	s.mu.Unlock()

	s.ClearIfActive()

	s.mu.Lock()
	if s.active {
		t.Error("expected active to be false")
	}
	if s.messageID != "" {
		t.Error("expected messageID to be empty")
	}
	if s.lastDetectedSig != "" {
		t.Error("expected lastDetectedSig to be cleared")
	}
	s.mu.Unlock()

	// When not active — should still clear sig
	s2 := &InteractionState{}
	s2.mu.Lock()
	s2.lastDetectedSig = "old-sig"
	s2.mu.Unlock()

	s2.ClearIfActive()

	s2.mu.Lock()
	if s2.lastDetectedSig != "" {
		t.Error("expected lastDetectedSig to be cleared even when not active")
	}
	s2.mu.Unlock()
}

func TestIsActiveMessage(t *testing.T) {
	s := &InteractionState{}

	if s.IsActiveMessage("msg1") {
		t.Error("expected false when not active")
	}

	s.mu.Lock()
	s.active = true
	s.messageID = "msg1"
	s.mu.Unlock()

	if !s.IsActiveMessage("msg1") {
		t.Error("expected true for matching message")
	}

	if s.IsActiveMessage("msg2") {
		t.Error("expected false for non-matching message")
	}
}

func TestInCooldown(t *testing.T) {
	s := &InteractionState{}

	// No cooldown set
	if s.InCooldown() {
		t.Error("expected false when no cooldown")
	}

	// Set cooldown
	s.mu.Lock()
	s.cooldownUntil = time.Now().Add(1 * time.Second)
	s.mu.Unlock()

	if !s.InCooldown() {
		t.Error("expected true during cooldown")
	}

	// Expired cooldown
	s.mu.Lock()
	s.cooldownUntil = time.Now().Add(-1 * time.Second)
	s.mu.Unlock()

	if s.InCooldown() {
		t.Error("expected false after cooldown expired")
	}
}

func TestIsConfirm(t *testing.T) {
	s := &InteractionState{}

	// Not active, not confirm
	if s.IsConfirm() {
		t.Error("expected false when not active")
	}

	// Active but not confirm
	s.mu.Lock()
	s.active = true
	s.isConfirm = false
	s.mu.Unlock()

	if s.IsConfirm() {
		t.Error("expected false when active but not confirm")
	}

	// Active and confirm
	s.mu.Lock()
	s.isConfirm = true
	s.mu.Unlock()

	if !s.IsConfirm() {
		t.Error("expected true when active and confirm")
	}

	// Not active but confirm flag set
	s.mu.Lock()
	s.active = false
	s.isConfirm = true
	s.mu.Unlock()

	if s.IsConfirm() {
		t.Error("expected false when not active even if isConfirm is true")
	}
}

func TestConcurrentStateAccess(t *testing.T) {
	s := &InteractionState{}
	var wg sync.WaitGroup

	// Run multiple goroutines accessing state concurrently
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			s.ClearChoice()
		}()
		go func() {
			defer wg.Done()
			s.InCooldown()
		}()
	}
	wg.Wait()
}
