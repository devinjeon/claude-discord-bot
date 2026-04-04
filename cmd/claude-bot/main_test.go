package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadEnvFile(t *testing.T) {
	tmpDir := t.TempDir()
	content := "# comment\nDISCORD_BOT_TOKEN=test-token-123\nDISCORD_CHANNEL_ID=12345\n"
	if err := os.WriteFile(filepath.Join(tmpDir, ".env"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	vars := loadEnvFile(tmpDir)

	if vars["DISCORD_BOT_TOKEN"] != "test-token-123" {
		t.Errorf("expected 'test-token-123', got %q", vars["DISCORD_BOT_TOKEN"])
	}
	if vars["DISCORD_CHANNEL_ID"] != "12345" {
		t.Errorf("expected '12345', got %q", vars["DISCORD_CHANNEL_ID"])
	}
}

func TestLoadEnvFileEmpty(t *testing.T) {
	vars := loadEnvFile("")
	if len(vars) != 0 {
		t.Errorf("expected empty map, got %v", vars)
	}
}

func TestLoadEnvFileNoFile(t *testing.T) {
	vars := loadEnvFile(t.TempDir())
	if len(vars) != 0 {
		t.Errorf("expected empty map, got %v", vars)
	}
}

func TestLoadEnvFileSkipsComments(t *testing.T) {
	tmpDir := t.TempDir()
	content := "# comment\n\nKEY=value\n# another comment\n"
	os.WriteFile(filepath.Join(tmpDir, ".env"), []byte(content), 0644)

	vars := loadEnvFile(tmpDir)
	if len(vars) != 1 || vars["KEY"] != "value" {
		t.Errorf("expected {KEY: value}, got %v", vars)
	}
}

func TestLoadEnvFileEqualsInValue(t *testing.T) {
	tmpDir := t.TempDir()
	content := "TOKEN=abc123==\n"
	os.WriteFile(filepath.Join(tmpDir, ".env"), []byte(content), 0644)

	vars := loadEnvFile(tmpDir)
	if vars["TOKEN"] != "abc123==" {
		t.Errorf("expected 'abc123==', got %q", vars["TOKEN"])
	}
}

func TestEnvOrFile(t *testing.T) {
	fileVars := map[string]string{"KEY": "from-file"}

	// File fallback
	got := envOrFile("KEY", fileVars)
	if got != "from-file" {
		t.Errorf("expected 'from-file', got %q", got)
	}

	// Env takes precedence
	t.Setenv("KEY", "from-env")
	got = envOrFile("KEY", fileVars)
	if got != "from-env" {
		t.Errorf("expected 'from-env', got %q", got)
	}

	// Missing key
	got = envOrFile("MISSING", fileVars)
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestLoadChannelIDFromAccess(t *testing.T) {
	tmpDir := t.TempDir()
	access := map[string]any{
		"groups": map[string]any{
			"999888777": map[string]any{"requireMention": false},
		},
	}
	data, _ := json.Marshal(access)
	os.WriteFile(filepath.Join(tmpDir, "access.json"), data, 0644)

	got := loadChannelIDFromAccess(tmpDir)
	if got != "999888777" {
		t.Errorf("expected '999888777', got %q", got)
	}
}

func TestLoadChannelIDFromAccessNoFile(t *testing.T) {
	got := loadChannelIDFromAccess(t.TempDir())
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestLoadChannelIDFromAccessEmpty(t *testing.T) {
	got := loadChannelIDFromAccess("")
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestLoadChannelIDFromAccessNoGroups(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "access.json"), []byte(`{"groups":{}}`), 0644)

	got := loadChannelIDFromAccess(tmpDir)
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestParsePollInterval(t *testing.T) {
	tests := []struct {
		name string
		input string
		want  time.Duration
	}{
		{"empty", "", defaultPollInterval},
		{"valid 3", "3", 3 * time.Second},
		{"valid 10", "10", 10 * time.Second},
		{"zero", "0", defaultPollInterval},
		{"negative", "-1", defaultPollInterval},
		{"invalid", "abc", defaultPollInterval},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parsePollInterval(tt.input)
			if got != tt.want {
				t.Errorf("parsePollInterval(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestLoadInstances(t *testing.T) {
	tmpDir := t.TempDir()
	content := `{"instances":[{"name":"test","channel_id":"123","tmux_session":"claude-channel-test"}]}`
	os.WriteFile(filepath.Join(tmpDir, "instances.json"), []byte(content), 0644)

	instances := loadInstances(tmpDir)
	if len(instances) != 1 {
		t.Fatalf("expected 1 instance, got %d", len(instances))
	}
	if instances[0].Name != "test" {
		t.Errorf("expected name 'test', got %q", instances[0].Name)
	}
	if instances[0].ChannelID != "123" {
		t.Errorf("expected channel_id '123', got %q", instances[0].ChannelID)
	}
	if instances[0].TmuxSession != "claude-channel-test" {
		t.Errorf("expected tmux_session 'claude-channel-test', got %q", instances[0].TmuxSession)
	}
}

func TestLoadInstancesNoFile(t *testing.T) {
	instances := loadInstances(t.TempDir())
	if len(instances) != 0 {
		t.Errorf("expected empty, got %v", instances)
	}
}

func TestLoadInstancesEmpty(t *testing.T) {
	instances := loadInstances("")
	if instances != nil {
		t.Errorf("expected nil, got %v", instances)
	}
}

func TestFetchGuildID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bot test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"guild_id": "111222333"})
	}))
	defer server.Close()

	// Override the API URL by using a custom transport
	origFetch := fetchGuildIDFunc
	fetchGuildIDFunc = func(token, channelID string) (string, error) {
		req, _ := http.NewRequest("GET", server.URL+"/channels/"+channelID, nil)
		req.Header.Set("Authorization", "Bot "+token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		var ch struct {
			GuildID string `json:"guild_id"`
		}
		json.NewDecoder(resp.Body).Decode(&ch)
		return ch.GuildID, nil
	}
	defer func() { fetchGuildIDFunc = origFetch }()

	guildID, err := fetchGuildIDFunc("test-token", "12345")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if guildID != "111222333" {
		t.Errorf("expected '111222333', got %q", guildID)
	}
}
