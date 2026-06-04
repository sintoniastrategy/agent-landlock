package agentlandlock

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCheckClaudeProfileEnvMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path, status, err := checkClaudeProfileEnv()
	if err != nil {
		t.Fatal(err)
	}
	if status != "missing" {
		t.Fatalf("status = %q, want missing", status)
	}
	if path != filepath.Join(home, ".profile") {
		t.Fatalf("path = %q", path)
	}
}

func TestHealClaudeProfileEnvCreatesProfile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path, status, err := healClaudeProfileEnv(false)
	if err != nil {
		t.Fatal(err)
	}
	if status != "healed" {
		t.Fatalf("status = %q, want healed", status)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	for _, want := range []string{claudeEnvBegin, `export CLAUDE_CONFIG_DIR="$HOME/.claude"`, claudeEnvEnd} {
		if !strings.Contains(content, want) {
			t.Fatalf("profile missing %q:\n%s", want, content)
		}
	}

	// Second run is idempotent.
	_, status, err = healClaudeProfileEnv(false)
	if err != nil {
		t.Fatal(err)
	}
	if status != "ok" {
		t.Fatalf("second heal status = %q, want ok", status)
	}
}

func TestHealClaudeProfileEnvPreservesUserContent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, ".profile")
	if err := os.WriteFile(path, []byte("# my profile\numask 022\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, status, err := healClaudeProfileEnv(false)
	if err != nil {
		t.Fatal(err)
	}
	if status != "healed" {
		t.Fatalf("status = %q, want healed", status)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	for _, want := range []string{"# my profile", "umask 022", claudeEnvBegin} {
		if !strings.Contains(content, want) {
			t.Fatalf("profile missing %q:\n%s", want, content)
		}
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want 0600 preserved", st.Mode().Perm())
	}
}

func TestHealClaudeProfileEnvReplacesStaleBlock(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, ".profile")
	stale := claudeEnvBegin + "\nexport CLAUDE_CONFIG_DIR=/old/path\n" + claudeEnvEnd + "\n"
	if err := os.WriteFile(path, []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, status, err := checkClaudeProfileEnv(); err != nil || status != "stale" {
		t.Fatalf("check status = %q err = %v, want stale", status, err)
	}
	_, status, err := healClaudeProfileEnv(false)
	if err != nil {
		t.Fatal(err)
	}
	if status != "healed" {
		t.Fatalf("status = %q, want healed", status)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "/old/path") {
		t.Fatalf("stale export survived:\n%s", data)
	}
}

func TestCheckClaudeProfileEnvUserManaged(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	bashrc := filepath.Join(home, ".bashrc")
	if err := os.WriteFile(bashrc, []byte("export CLAUDE_CONFIG_DIR=$HOME/.claude\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	path, status, err := checkClaudeProfileEnv()
	if err != nil {
		t.Fatal(err)
	}
	if status != "ok (user-managed)" {
		t.Fatalf("status = %q, want ok (user-managed)", status)
	}
	if path != bashrc {
		t.Fatalf("path = %q, want %q", path, bashrc)
	}

	// Heal must not touch anything.
	_, status, err = healClaudeProfileEnv(false)
	if err != nil {
		t.Fatal(err)
	}
	if status != "ok (user-managed)" {
		t.Fatalf("heal status = %q, want ok (user-managed)", status)
	}
	if _, err := os.Stat(filepath.Join(home, ".profile")); !os.IsNotExist(err) {
		t.Fatalf(".profile was created, err = %v", err)
	}
}

func TestCheckClaudeProfileEnvIgnoresComments(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, ".profile")
	if err := os.WriteFile(path, []byte("# export CLAUDE_CONFIG_DIR=$HOME/.claude\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, status, err := checkClaudeProfileEnv()
	if err != nil {
		t.Fatal(err)
	}
	if status != "missing" {
		t.Fatalf("status = %q, want missing", status)
	}
}

func TestHealClaudeProfileEnvDryRun(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path, status, err := healClaudeProfileEnv(true)
	if err != nil {
		t.Fatal(err)
	}
	if status != "would heal" {
		t.Fatalf("status = %q, want would heal", status)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf(".profile was created in dry-run, err = %v", err)
	}
}

func TestHealClaudeProfileEnvRefusesSymlink(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	target := filepath.Join(home, "real-profile")
	if err := os.WriteFile(target, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(home, ".profile")); err != nil {
		t.Fatal(err)
	}

	_, _, err := healClaudeProfileEnv(false)
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("err = %v, want symlink refusal", err)
	}
}

func TestCheckClaudeConfigSplit(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	legacy := filepath.Join(home, ".claude.json")
	bridged := filepath.Join(home, ".claude", ".claude.json")

	// Neither file exists.
	if status, err := checkClaudeConfigSplit(); err != nil || status != "ok" {
		t.Fatalf("status = %q err = %v, want ok", status, err)
	}

	if err := os.MkdirAll(filepath.Dir(bridged), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Legacy only.
	if status, err := checkClaudeConfigSplit(); err != nil || status != "ok" {
		t.Fatalf("status = %q err = %v, want ok", status, err)
	}

	if err := os.WriteFile(bridged, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Bridged newer than legacy.
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(legacy, old, old); err != nil {
		t.Fatal(err)
	}
	if status, err := checkClaudeConfigSplit(); err != nil || status != "ok" {
		t.Fatalf("status = %q err = %v, want ok", status, err)
	}

	// Legacy newer than bridged.
	newer := time.Now().Add(time.Hour)
	if err := os.Chtimes(legacy, newer, newer); err != nil {
		t.Fatal(err)
	}
	if status, err := checkClaudeConfigSplit(); err != nil || status != "split" {
		t.Fatalf("status = %q err = %v, want split", status, err)
	}
}
