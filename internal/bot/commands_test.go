package bot

import (
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
