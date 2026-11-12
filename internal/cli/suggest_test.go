package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestLevenshtein(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"plan", "plan", 0},
		{"", "run", 3},
		{"pln", "plan", 1},
		{"palns", "plan", 3},
		{"kitten", "sitting", 3},
		{"çao", "cao", 1}, // runes, not bytes
	}
	for _, tt := range tests {
		if got := levenshtein(tt.a, tt.b); got != tt.want {
			t.Errorf("levenshtein(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
		if got := levenshtein(tt.b, tt.a); got != tt.want {
			t.Errorf("levenshtein(%q, %q) = %d, want %d (symmetry)", tt.b, tt.a, got, tt.want)
		}
	}
}

func TestSuggest(t *testing.T) {
	tests := []struct {
		name, want string
	}{
		{"pln", "plan"},
		{"plann", "plan"},
		{"rum", "run"},
		{"vresion", "version"},
		{"hlep", "help"},
		{"bogus", ""},
		{"", ""},
	}
	for _, tt := range tests {
		if got := suggest(tt.name); got != tt.want {
			t.Errorf("suggest(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestRunSuggestsCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := Run(t.Context(), []string{"pln"}, &stdout, &stderr)
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("Run(pln) = %v, want %v", err, ErrUsage)
	}
	if !strings.Contains(stderr.String(), `did you mean "plan"?`) {
		t.Errorf("stderr = %q, want a suggestion for plan", stderr.String())
	}

	stderr.Reset()
	_ = Run(t.Context(), []string{"bogus"}, &stdout, &stderr)
	if strings.Contains(stderr.String(), "did you mean") {
		t.Errorf("stderr = %q, want no suggestion for bogus", stderr.String())
	}
}
