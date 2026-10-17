package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeEnv(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadEnvFile(t *testing.T) {
	path := writeEnv(t, "# comment\n\nCFG_A=one\nCFG_B = \"two words\"\n")
	t.Cleanup(func() { os.Unsetenv("CFG_A"); os.Unsetenv("CFG_B") })

	if err := LoadEnvFile(path); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("CFG_A"); got != "one" {
		t.Errorf("CFG_A = %q, want %q", got, "one")
	}
	if got := os.Getenv("CFG_B"); got != "two words" {
		t.Errorf("CFG_B = %q, want %q", got, "two words")
	}
}

func TestLoadEnvFileKeepsExistingEnvironment(t *testing.T) {
	t.Setenv("CFG_C", "from-env")
	path := writeEnv(t, "CFG_C=from-file\n")

	if err := LoadEnvFile(path); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("CFG_C"); got != "from-env" {
		t.Errorf("CFG_C = %q, want the environment value", got)
	}
}

func TestLoadEnvFileMissingFile(t *testing.T) {
	if err := LoadEnvFile(filepath.Join(t.TempDir(), "nope")); err != nil {
		t.Fatalf("missing file: got %v, want nil", err)
	}
}

func TestLoadEnvFileInvalidLine(t *testing.T) {
	if err := LoadEnvFile(writeEnv(t, "not a pair\n")); err == nil {
		t.Fatal("want an error for a line without =")
	}
}

func TestZooAddr(t *testing.T) {
	t.Setenv("ZOO_HTTP_PORT", "")
	if got, want := ZooAddr(), "127.0.0.1:8080"; got != want {
		t.Errorf("default = %q, want %q", got, want)
	}

	t.Setenv("ZOO_HTTP_PORT", "9090")
	if got, want := ZooAddr(), "127.0.0.1:9090"; got != want {
		t.Errorf("custom = %q, want %q", got, want)
	}
}
