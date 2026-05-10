package agentlandlock

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareAgentEnvSetsClaudeConfigDirAndMigratesHomeConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.WriteFile(filepath.Join(home, ".claude.json"), []byte("home-config"), 0o600); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{}
	if err := prepareAgentEnv("claude", env, false, false); err != nil {
		t.Fatal(err)
	}
	wantDir := filepath.Join(home, ".claude")
	if env["CLAUDE_CONFIG_DIR"] != wantDir {
		t.Fatalf("CLAUDE_CONFIG_DIR = %q, want %q", env["CLAUDE_CONFIG_DIR"], wantDir)
	}
	got, err := os.ReadFile(filepath.Join(wantDir, ".claude.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "home-config" {
		t.Fatalf("bridged config = %q", got)
	}
}

func TestPrepareAgentEnvDoesNotOverrideExistingClaudeConfigDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	env := map[string]string{"CLAUDE_CONFIG_DIR": "/custom/claude"}
	if err := prepareAgentEnv("claude", env, false, false); err != nil {
		t.Fatal(err)
	}
	if env["CLAUDE_CONFIG_DIR"] != "/custom/claude" {
		t.Fatalf("CLAUDE_CONFIG_DIR = %q", env["CLAUDE_CONFIG_DIR"])
	}
}

func TestPrepareAgentEnvSkippedWithNoAgentState(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	env := map[string]string{}
	if err := prepareAgentEnv("claude", env, true, false); err != nil {
		t.Fatal(err)
	}
	if _, ok := env["CLAUDE_CONFIG_DIR"]; ok {
		t.Fatal("CLAUDE_CONFIG_DIR should not be set with --no-agent-state")
	}
}

func TestPrepareAgentEnvDryRunDoesNotCreateClaudeConfigDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	env := map[string]string{}
	if err := prepareAgentEnv("claude", env, false, true); err != nil {
		t.Fatal(err)
	}
	if env["CLAUDE_CONFIG_DIR"] != filepath.Join(home, ".claude") {
		t.Fatalf("CLAUDE_CONFIG_DIR = %q", env["CLAUDE_CONFIG_DIR"])
	}
	if _, err := os.Stat(filepath.Join(home, ".claude")); !os.IsNotExist(err) {
		t.Fatalf("dry-run created Claude config dir or returned unexpected error: %v", err)
	}
}
