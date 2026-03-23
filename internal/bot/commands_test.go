package bot

import (
	"strings"
	"testing"
)

func TestStrPtr(t *testing.T) {
	s := "hello"
	p := strPtr(s)
	if p == nil {
		t.Fatal("expected non-nil pointer")
	}
	if *p != s {
		t.Errorf("expected %q, got %q", s, *p)
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{
			name:   "short string",
			input:  "hello",
			maxLen: 10,
			want:   "hello",
		},
		{
			name:   "exact length",
			input:  "hello",
			maxLen: 5,
			want:   "hello",
		},
		{
			name:   "needs truncation",
			input:  "hello world, this is a long string",
			maxLen: 15,
			want:   "... long string",
		},
		{
			name:   "empty string",
			input:  "",
			maxLen: 10,
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestFormatCodeBlock(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{
			name:   "simple text",
			input:  "hello",
			maxLen: 100,
			want:   "```\nhello\n```",
		},
		{
			name:   "escapes triple backticks",
			input:  "some ```code``` here",
			maxLen: 100,
			want:   "```\nsome ` ` `code` ` ` here\n```",
		},
		{
			name:   "empty",
			input:  "",
			maxLen: 100,
			want:   "```\n\n```",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatCodeBlock(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("formatCodeBlock(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestSendkeyChoices(t *testing.T) {
	// Verify all expected keys are present
	expectedKeys := []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "Escape", "Enter", "Tab", "Up", "Down"}
	if len(sendkeyChoices) != len(expectedKeys) {
		t.Errorf("expected %d sendkey choices, got %d", len(expectedKeys), len(sendkeyChoices))
	}

	choiceMap := make(map[string]bool)
	for _, c := range sendkeyChoices {
		choiceMap[c.Value.(string)] = true
	}

	for _, key := range expectedKeys {
		if !choiceMap[key] {
			t.Errorf("missing sendkey choice: %s", key)
		}
	}
}

func TestSendkeyChoicesFiltering(t *testing.T) {
	tests := []struct {
		query    string
		minCount int
		mustHave []string
	}{
		{"", 14, nil},                         // empty query returns all
		{"esc", 1, []string{"Escape"}},        // partial match
		{"enter", 1, []string{"Enter"}},       // partial match
		{"tab", 1, []string{"Tab"}},           // partial match
		{"1", 1, []string{"1"}},               // number match
		{"xyz", 0, nil},                       // no match
		{"up", 1, []string{"Up"}},             // case insensitive
		{"DOWN", 1, []string{"Down"}},         // case insensitive
	}

	for _, tt := range tests {
		t.Run("query="+tt.query, func(t *testing.T) {
			var filtered []*sendkeyChoice
			for _, c := range sendkeyChoices {
				if tt.query == "" || strings.Contains(strings.ToLower(c.Name), strings.ToLower(tt.query)) {
					filtered = append(filtered, &sendkeyChoice{Name: c.Name, Value: c.Value})
				}
			}

			if len(filtered) < tt.minCount {
				t.Errorf("expected at least %d results for query %q, got %d", tt.minCount, tt.query, len(filtered))
			}

			for _, must := range tt.mustHave {
				found := false
				for _, c := range filtered {
					if c.Name == must {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected %q in results for query %q", must, tt.query)
				}
			}
		})
	}
}

type sendkeyChoice struct {
	Name  string
	Value interface{}
}

func TestExtractUsageBlock(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   string
	}{
		{
			name:   "no usage block",
			input:  "just some random output\nwith multiple lines",
			want:   "just some random output\nwith multiple lines",
		},
		{
			name: "with usage block",
			input: "header stuff\nCurrent session usage:\n  API calls: 42\n  Tokens: 1000\nEsc to cancel\ntrailing",
			want:  "Current session usage:\n  API calls: 42\n  Tokens: 1000\nEsc to cancel",
		},
		{
			name: "current session only",
			input: "before\nCurrent session stats\nsome data\nmore data",
			want:  "Current session stats\nsome data\nmore data",
		},
		{
			name:   "empty input",
			input:  "",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractUsageBlock(tt.input)
			if got != tt.want {
				t.Errorf("extractUsageBlock() = %q, want %q", got, tt.want)
			}
		})
	}
}
