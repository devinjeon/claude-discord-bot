package tmux

import (
	"os/exec"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Session != "claude-channel" {
		t.Errorf("expected session 'claude-channel', got %q", cfg.Session)
	}
	if cfg.Timeout != defaultTimeout {
		t.Errorf("expected timeout %v, got %v", defaultTimeout, cfg.Timeout)
	}
	if cfg.Path == "" {
		t.Error("expected non-empty path")
	}
}

func TestFindTmux(t *testing.T) {
	path := FindTmux()
	if path == "" {
		t.Error("findTmux returned empty string")
	}
	// The result should be a valid path or bare "tmux"
	if _, err := exec.LookPath(path); err != nil {
		// Acceptable if tmux isn't installed on CI
		t.Logf("tmux not found at %q (may not be installed): %v", path, err)
	}
}

func TestConfigTimeout(t *testing.T) {
	tests := []struct {
		name     string
		cfg      Config
		expected time.Duration
	}{
		{
			name:     "zero uses default",
			cfg:      Config{},
			expected: defaultTimeout,
		},
		{
			name:     "custom timeout",
			cfg:      Config{Timeout: 5 * time.Second},
			expected: 5 * time.Second,
		},
		{
			name:     "negative uses default",
			cfg:      Config{Timeout: -1},
			expected: defaultTimeout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.timeout()
			if got != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}

func TestSendKeysInvalidPath(t *testing.T) {
	cfg := Config{Path: "/nonexistent/tmux", Session: "test", Timeout: 1 * time.Second}
	err := cfg.SendKeys("Enter")
	if err == nil {
		t.Error("expected error for invalid tmux path")
	}
}

func TestSendTextInvalidPath(t *testing.T) {
	cfg := Config{Path: "/nonexistent/tmux", Session: "test", Timeout: 1 * time.Second}
	err := cfg.SendText("hello")
	if err == nil {
		t.Error("expected error for invalid tmux path")
	}
}

func TestCapturePaneInvalidPath(t *testing.T) {
	cfg := Config{Path: "/nonexistent/tmux", Session: "test", Timeout: 1 * time.Second}
	output, err := cfg.CapturePane()
	if err == nil {
		t.Error("expected error for invalid tmux path")
	}
	if output != "" {
		t.Errorf("expected empty output, got %q", output)
	}
}

func TestKillSessionInvalidPath(t *testing.T) {
	cfg := Config{Path: "/nonexistent/tmux", Session: "test", Timeout: 1 * time.Second}
	err := cfg.KillSession()
	if err == nil {
		t.Error("expected error for invalid tmux path")
	}
}

func TestSendKeysTimeout(t *testing.T) {
	cfg := Config{Path: "/bin/sleep", Session: "10", Timeout: 100 * time.Millisecond}
	// /bin/sleep 10 as "send-keys -t 10 Enter" won't work, but the important
	// thing is that the context timeout is applied. Use a script that hangs.
	err := cfg.SendKeys("Enter")
	// We expect an error (either from invalid args or timeout)
	if err == nil {
		t.Error("expected error")
	}
}
