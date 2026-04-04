package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/devinjeon/claude-discord-bot/internal/bot"
	"github.com/devinjeon/claude-discord-bot/internal/tmux"
)

const defaultPollInterval = 15 * time.Second

// instanceEntry represents a single instance in instances.json.
type instanceEntry struct {
	Name        string `json:"name"`
	ChannelID   string `json:"channel_id"`
	TmuxSession string `json:"tmux_session"`
}

// instancesFile represents the top-level instances.json structure.
type instancesFile struct {
	Instances []instanceEntry `json:"instances"`
}

func main() {
	configDir := discordConfigDir()
	envVars := loadEnvFile(configDir)

	token := envOrFile("DISCORD_BOT_TOKEN", envVars)
	if token == "" {
		log.Fatal("DISCORD_BOT_TOKEN is required (set env var or add to ~/.claude/channels/discord/.env)")
	}

	pollInterval := parsePollInterval(envOrFile("POLL_INTERVAL", envVars))

	// Try multi-instance mode first (instances.json)
	instances := loadInstances(configDir)

	if len(instances) > 0 {
		// Multi-instance mode
		guildID := envOrFile("DISCORD_GUILD_ID", envVars)
		if guildID == "" {
			// Resolve from first instance's channel
			var err error
			guildID, err = fetchGuildID(token, instances[0].ChannelID)
			if err != nil {
				log.Fatalf("Failed to fetch guild ID from Discord API: %v", err)
			}
			log.Printf("Resolved guild ID from Discord API: %s", guildID)
		}

		var botInstances []bot.InstanceConfig
		for _, inst := range instances {
			botInstances = append(botInstances, bot.InstanceConfig{
				Name:      inst.Name,
				ChannelID: inst.ChannelID,
				Tmux: tmux.Config{
					Path:    tmux.FindTmux(),
					Session: inst.TmuxSession,
					Timeout: 10 * time.Second,
				},
			})
		}

		cfg := bot.Config{
			Token:        token,
			GuildID:      guildID,
			Instances:    botInstances,
			PollInterval: pollInterval,
		}

		b, err := bot.New(cfg)
		if err != nil {
			log.Fatalf("Failed to create bot: %v", err)
		}

		done := make(chan struct{})
		go func() {
			sig := make(chan os.Signal, 1)
			signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
			<-sig
			close(done)
		}()

		if err := b.Run(done); err != nil {
			log.Fatalf("Bot error: %v", err)
		}
		return
	}

	// Legacy single-instance mode
	channelID := envOrFile("DISCORD_CHANNEL_ID", envVars)
	if channelID == "" {
		channelID = loadChannelIDFromAccess(configDir)
	}
	if channelID == "" {
		log.Fatal("No instances.json found and DISCORD_CHANNEL_ID could not be determined")
	}

	guildID := envOrFile("DISCORD_GUILD_ID", envVars)
	if guildID == "" {
		var err error
		guildID, err = fetchGuildID(token, channelID)
		if err != nil {
			log.Fatalf("Failed to fetch guild ID from Discord API: %v", err)
		}
		log.Printf("Resolved guild ID from Discord API: %s", guildID)
	}

	cfg := bot.Config{
		Token:   token,
		GuildID: guildID,
		Instances: []bot.InstanceConfig{
			{
				Name:      "default",
				ChannelID: channelID,
				Tmux:      tmux.DefaultConfig(),
			},
		},
		PollInterval: pollInterval,
	}

	b, err := bot.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create bot: %v", err)
	}

	done := make(chan struct{})

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		close(done)
	}()

	if err := b.Run(done); err != nil {
		log.Fatalf("Bot error: %v", err)
	}
}

func discordConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude", "channels", "discord")
}

// loadInstances loads the instances.json file from the config directory.
func loadInstances(configDir string) []instanceEntry {
	if configDir == "" {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(configDir, "instances.json"))
	if err != nil {
		return nil
	}
	var f instancesFile
	if err := json.Unmarshal(data, &f); err != nil {
		log.Printf("Failed to parse instances.json: %v", err)
		return nil
	}
	log.Printf("Loaded %d instances from instances.json", len(f.Instances))
	return f.Instances
}

// envOrFile returns the environment variable value, falling back to the .env file value.
func envOrFile(key string, fileVars map[string]string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fileVars[key]
}

// loadEnvFile parses the .env file in the given directory and returns key-value pairs.
func loadEnvFile(dir string) map[string]string {
	vars := make(map[string]string)
	if dir == "" {
		return vars
	}

	f, err := os.Open(filepath.Join(dir, ".env"))
	if err != nil {
		return vars
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		vars[key] = value
	}
	return vars
}

// loadChannelIDFromAccess extracts the first channel ID from access.json groups.
func loadChannelIDFromAccess(dir string) string {
	if dir == "" {
		return ""
	}

	data, err := os.ReadFile(filepath.Join(dir, "access.json"))
	if err != nil {
		return ""
	}

	var access struct {
		Groups map[string]json.RawMessage `json:"groups"`
	}
	if err := json.Unmarshal(data, &access); err != nil {
		return ""
	}

	for id := range access.Groups {
		log.Printf("Resolved channel ID from access.json: %s", id)
		return id
	}
	return ""
}

// parsePollInterval parses the poll interval from a string (seconds).
// Returns defaultPollInterval if empty or invalid.
func parsePollInterval(s string) time.Duration {
	if s == "" {
		return defaultPollInterval
	}
	sec, err := strconv.Atoi(s)
	if err != nil || sec < 1 {
		log.Printf("Invalid POLL_INTERVAL %q, using default %v", s, defaultPollInterval)
		return defaultPollInterval
	}
	return time.Duration(sec) * time.Second
}

// fetchGuildIDFunc is the function used to fetch guild ID. Replaceable for testing.
var fetchGuildIDFunc = fetchGuildID

// fetchGuildID calls the Discord API to get the guild ID for a channel.
func fetchGuildID(token, channelID string) (string, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("https://discord.com/api/v10/channels/%s", channelID), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bot "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Discord API returned %d: %s", resp.StatusCode, string(body))
	}

	var channel struct {
		GuildID string `json:"guild_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&channel); err != nil {
		return "", err
	}
	if channel.GuildID == "" {
		return "", fmt.Errorf("channel %s has no guild_id (DM channel?)", channelID)
	}
	return channel.GuildID, nil
}
