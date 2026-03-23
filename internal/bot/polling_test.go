package bot

import (
	"testing"
)

func TestChoicePattern(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		match   bool
		num     string
		text    string
	}{
		{
			name:  "standard choice",
			input: "  1. Accept all changes",
			match: true,
			num:   "1",
			text:  "Accept all changes",
		},
		{
			name:  "choice with icon",
			input: "  ❯ 2. Reject and revert",
			match: true,
			num:   "2",
			text:  "Reject and revert",
		},
		{
			name:  "no leading space",
			input: "3. Option three",
			match: true,
			num:   "3",
			text:  "Option three",
		},
		{
			name:  "no match — text only",
			input: "just some regular text",
			match: false,
		},
		{
			name:  "no match — empty",
			input: "",
			match: false,
		},
		{
			name:  "double digit",
			input: "  10. Tenth option",
			match: true,
			num:   "10",
			text:  "Tenth option",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := choicePattern.FindStringSubmatch(tt.input)
			if tt.match {
				if matches == nil {
					t.Fatalf("expected match for %q", tt.input)
				}
				if len(matches) < 3 {
					t.Fatalf("expected at least 3 groups, got %d", len(matches))
				}
				if matches[1] != tt.num {
					t.Errorf("expected num %q, got %q", tt.num, matches[1])
				}
				if matches[2] != tt.text {
					t.Errorf("expected text %q, got %q", tt.text, matches[2])
				}
			} else {
				if matches != nil {
					t.Errorf("expected no match for %q, got %v", tt.input, matches)
				}
			}
		})
	}
}

func TestFooterPattern(t *testing.T) {
	tests := []struct {
		input string
		match bool
	}{
		{"  Enter to select  |  ↑↓ to navigate  |  Esc to cancel", true},
		{"Enter to select", true},
		{"Use arrows to navigate", true},
		{"Esc to cancel", true},
		{"just some text", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := footerPattern.MatchString(tt.input)
			if got != tt.match {
				t.Errorf("footerPattern.MatchString(%q) = %v, want %v", tt.input, got, tt.match)
			}
		})
	}
}

func TestTextInputPattern(t *testing.T) {
	tests := []struct {
		input string
		match bool
	}{
		{"Tab to amend, Esc to cancel, Enter to submit", true},
		{"Tab to amend", true},
		{"Enter to submit", true},
		{"Type your response:", true},
		{"Type a response below:", true},
		{"submit your changes", true},
		{"just some text", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := textInputPattern.MatchString(tt.input)
			if got != tt.match {
				t.Errorf("textInputPattern.MatchString(%q) = %v, want %v", tt.input, got, tt.match)
			}
		})
	}
}

func TestNumberEmojis(t *testing.T) {
	if len(numberEmojis) != 9 {
		t.Errorf("expected 9 number emojis, got %d", len(numberEmojis))
	}
}

func TestEscEmoji(t *testing.T) {
	if escEmoji != "❌" {
		t.Errorf("expected ❌, got %s", escEmoji)
	}
}
