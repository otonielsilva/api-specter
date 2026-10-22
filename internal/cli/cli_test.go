package cli

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantErr    error
		wantStdout bool // usage goes to stdout for help, to stderr for errors
	}{
		{"no args", []string{}, ErrUsage, false},
		{"unknown command", []string{"bogus"}, ErrUsage, false},
		{"unknown command with flag", []string{"bogus", "--bogus"}, ErrUsage, false},
		{"flag as command", []string{"--run"}, ErrUsage, false},
		{"help", []string{"help"}, nil, true},
		{"-h", []string{"-h"}, nil, true},
		{"--help", []string{"--help"}, nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := Run(t.Context(), tt.args, &stdout, &stderr)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Run(%v) = %v, want %v", tt.args, err, tt.wantErr)
			}
			if got := stdout.Len() > 0; got != tt.wantStdout {
				t.Errorf("Run(%v) wrote to stdout = %v, want %v", tt.args, got, tt.wantStdout)
			}
		})
	}
}

func TestPlanCmd(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantErr    error
		wantStderr string // substring expected on stderr; "" skips the check
	}{
		{"help", []string{"-h"}, nil, "Usage of plan:"},
		{"missing --spec", []string{}, ErrUsage, "plan: --spec is required"},
		{"unknown flag", []string{"--unknow"}, ErrUsage, "flag provided but not defined: -unknow"},
		{"--spec without value", []string{"--spec"}, ErrUsage, "flag needs an argument: -spec"},
		{"missing spec file", []string{"--spec", filepath.Join(t.TempDir(), "nope.yaml")}, fs.ErrNotExist, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := planCmd(t.Context(), tt.args, &stdout, &stderr)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("planCmd(%v) = %v, wanted %v", tt.args, err, tt.wantErr)
			}
			if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("planCmd(%v) stderr = %q, want it to contain %q", tt.args, stderr.String(), tt.wantStderr)
			}
		})
	}
}

func TestRunCmd(t *testing.T) {
	badPlan := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(badPlan, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	zooPlan := filepath.Join("..", "plan", "testdata", "zoo.plan.json")
	missing := filepath.Join(t.TempDir(), "nope")

	tests := []struct {
		name       string
		args       []string
		wantErr    error
		wantAnyErr bool   // an error that is not ErrUsage; wantErr is ignored
		wantStdout string // substring expected on stdout; "" skips the check
		wantStderr string // substring expected on stderr; "" skips the check
	}{
		{name: "help", args: []string{"-h"}, wantStderr: "Usage of run:"},
		{name: "missing --plan", args: []string{}, wantErr: ErrUsage, wantStderr: "run: --plan is required"},
		{name: "unknown flag", args: []string{"--unknow"}, wantErr: ErrUsage, wantStderr: "flag provided but not defined: -unknow"},
		{name: "--plan without value", args: []string{"--plan"}, wantErr: ErrUsage, wantStderr: "flag needs an argument: -plan"},
		{name: "--spec without value", args: []string{"--plan", zooPlan, "--spec"}, wantErr: ErrUsage, wantStderr: "flag needs an argument: -spec"},
		{name: "missing plan file", args: []string{"--plan", missing + ".json"}, wantErr: fs.ErrNotExist},
		{name: "plan is not json", args: []string{"--plan", badPlan}, wantAnyErr: true},
		{name: "missing spec file", args: []string{"--plan", zooPlan, "--spec", missing + ".yaml"}, wantErr: fs.ErrNotExist},
		{name: "zoo plan", args: []string{"--plan", zooPlan}, wantStdout: "operations"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := runCmd(t.Context(), tt.args, &stdout, &stderr)

			if tt.wantAnyErr {
				if err == nil || errors.Is(err, ErrUsage) {
					t.Fatalf("runCmd(%v) = %v, want a non-usage error", tt.args, err)
				}
			} else if !errors.Is(err, tt.wantErr) {
				t.Fatalf("runCmd(%v) = %v, want %v", tt.args, err, tt.wantErr)
			}
			if !strings.Contains(stdout.String(), tt.wantStdout) {
				t.Errorf("runCmd(%v) stdout = %q, want it to contain %q", tt.args, stdout.String(), tt.wantStdout)
			}
			if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("runCmd(%v) stderr = %q, want it to contain %q", tt.args, stderr.String(), tt.wantStderr)
			}
		})
	}
}

func TestSpecLocation(t *testing.T) {
	tests := []struct {
		name     string
		planFile string
		inPlan   string
		override string
		want     string
	}{
		{"--spec wins", "plans/a.json", "spec.yaml", "other.yaml", "other.yaml"},
		{"--spec wins over absolute", "plans/a.json", "/abs/spec.yaml", "other.yaml", "other.yaml"},
		{"absolute path is kept", "plans/a.json", "/abs/spec.yaml", "", "/abs/spec.yaml"},
		{"relative to plan directory", "plans/a.json", "../spec.yaml", "", "spec.yaml"},
		{"plan in current directory", "a.json", "spec.yaml", "", "spec.yaml"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := specLocation(tt.planFile, tt.inPlan, tt.override)
			if got != tt.want {
				t.Errorf("specLocation(%q, %q, %q) = %q, want %q", tt.planFile, tt.inPlan, tt.override, got, tt.want)
			}
		})
	}
}
