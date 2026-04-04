package tmux

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const defaultTimeout = 10 * time.Second

// Config holds tmux connection parameters.
type Config struct {
	Path    string // path to tmux binary
	Session string // target session name
	Timeout time.Duration
}

// DefaultConfig returns the default tmux configuration.
// It searches PATH for tmux rather than hardcoding a specific location.
func DefaultConfig() Config {
	path := FindTmux()
	return Config{
		Path:    path,
		Session: "claude-channel",
		Timeout: defaultTimeout,
	}
}

// FindTmux locates the tmux binary using PATH lookup.
func FindTmux() string {
	if p, err := exec.LookPath("tmux"); err == nil {
		return p
	}
	// Fallback to common locations
	for _, p := range []string{
		"/opt/homebrew/bin/tmux",
		"/usr/local/bin/tmux",
		"/usr/bin/tmux",
	} {
		if _, err := exec.LookPath(p); err == nil {
			return p
		}
	}
	return "tmux"
}

func (c Config) timeout() time.Duration {
	if c.Timeout > 0 {
		return c.Timeout
	}
	return defaultTimeout
}

// SendKeys sends a key sequence to the tmux session.
func (c Config) SendKeys(key string) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout())
	defer cancel()
	cmd := exec.CommandContext(ctx, c.Path, "send-keys", "-t", c.Session, key)
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("tmux send-keys timed out after %v", c.timeout())
		}
		return err
	}
	return nil
}

// SendText sends literal text to the tmux session.
func (c Config) SendText(text string) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout())
	defer cancel()
	cmd := exec.CommandContext(ctx, c.Path, "send-keys", "-t", c.Session, "-l", text)
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("tmux send-text timed out after %v", c.timeout())
		}
		return err
	}
	return nil
}

// CapturePane captures the current tmux pane content.
func (c Config) CapturePane() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout())
	defer cancel()
	cmd := exec.CommandContext(ctx, c.Path, "capture-pane", "-t", c.Session, "-p")
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("tmux capture-pane timed out after %v", c.timeout())
		}
		return "", err
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// KillSession kills the tmux session.
func (c Config) KillSession() error {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout())
	defer cancel()
	cmd := exec.CommandContext(ctx, c.Path, "kill-session", "-t", c.Session)
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("tmux kill-session timed out after %v", c.timeout())
		}
		return err
	}
	return nil
}
