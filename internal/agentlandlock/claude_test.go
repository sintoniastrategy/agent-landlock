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

func TestPrepareAgentEnvToleratesHealedSymlink(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	configDir := filepath.Join(home, ".claude")
	dst := filepath.Join(configDir, ".claude.json")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("bridged"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Mirror a healed setup: legacy path is a symlink to the bridged config.
	if err := os.Symlink(dst, filepath.Join(home, ".claude.json")); err != nil {
		t.Fatal(err)
	}

	env := map[string]string{}
	if err := prepareAgentEnv("claude", env, false, false); err != nil {
		t.Fatalf("prepareAgentEnv returned error for healed symlink: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil || string(got) != "bridged" {
		t.Fatalf("bridged config = %q err = %v", got, err)
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
