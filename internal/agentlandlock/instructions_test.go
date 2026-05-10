package agentlandlock

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHealGlobalClaudeInstructionsPreservesUserMemory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(claudeDir, "CLAUDE.md")
	if err := os.WriteFile(path, []byte("# User memory\n\nKeep this line.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	gotPath, status, err := healGlobalClaudeInstructions(false)
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != path || status != "healed" {
		t.Fatalf("path/status = %q/%q, want %q/healed", gotPath, status, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	for _, want := range []string{
		"# User memory",
		"Keep this line.",
		runtimeInstructionsBegin,
		"## agent-landlock Runtime",
		"permission error outside the current working directory",
		runtimeInstructionsEnd,
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("CLAUDE.md missing %q:\n%s", want, content)
		}
	}
}

func TestHealGlobalClaudeInstructionsReplacesStaleManagedBlock(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(claudeDir, "CLAUDE.md")
	stale := "# User memory\n\n" + runtimeInstructionsBegin + "\nstale\n" + runtimeInstructionsEnd + "\n\nAfter block.\n"
	if err := os.WriteFile(path, []byte(stale), 0o600); err != nil {
		t.Fatal(err)
	}

	_, status, err := healGlobalClaudeInstructions(false)
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
	if strings.Contains(content, "\nstale\n") {
		t.Fatalf("stale managed block was not replaced:\n%s", content)
	}
	if strings.Count(content, runtimeInstructionsBegin) != 1 {
		t.Fatalf("managed block count = %d, want 1:\n%s", strings.Count(content, runtimeInstructionsBegin), content)
	}
	if !strings.Contains(content, "# User memory") || !strings.Contains(content, "After block.") {
		t.Fatalf("user content was not preserved:\n%s", content)
	}
}

func TestHealGlobalClaudeInstructionsIsIdempotent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if _, status, err := healGlobalClaudeInstructions(false); err != nil {
		t.Fatal(err)
	} else if status != "healed" {
		t.Fatalf("first status = %q, want healed", status)
	}
	path, status, err := healGlobalClaudeInstructions(false)
	if err != nil {
		t.Fatal(err)
	}
	if status != "ok" {
		t.Fatalf("second status = %q, want ok", status)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(data), runtimeInstructionsBegin) != 1 {
		t.Fatalf("managed block duplicated:\n%s", data)
	}
}

func TestHealGlobalClaudeInstructionsDryRunDoesNotCreateFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path, status, err := healGlobalClaudeInstructions(true)
	if err != nil {
		t.Fatal(err)
	}
	if status != "would heal" {
		t.Fatalf("status = %q, want would heal", status)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("dry-run created CLAUDE.md or returned unexpected error: %v", err)
	}
}
