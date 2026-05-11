package agentlandlock

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHealAgentInstructionsPreservesUserMemory(t *testing.T) {
	for _, ag := range supportedAgents {
		t.Run(ag.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			dir := filepath.Join(home, ag.dir)
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, ag.file)
			if err := os.WriteFile(path, []byte("# User memory\n\nKeep this line.\n"), 0o644); err != nil {
				t.Fatal(err)
			}

			gotPath, status, err := healAgentInstructions(ag, false)
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
				"This CLI can run " + ag.name + " through",
				"permission error outside the current working directory",
				runtimeInstructionsEnd,
			} {
				if !strings.Contains(content, want) {
					t.Fatalf("%s missing %q:\n%s", ag.file, want, content)
				}
			}
		})
	}
}

func TestHealAgentInstructionsReplacesStaleManagedBlock(t *testing.T) {
	for _, ag := range supportedAgents {
		t.Run(ag.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			dir := filepath.Join(home, ag.dir)
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, ag.file)
			stale := "# User memory\n\n" + runtimeInstructionsBegin + "\nstale\n" + runtimeInstructionsEnd + "\n\nAfter block.\n"
			if err := os.WriteFile(path, []byte(stale), 0o600); err != nil {
				t.Fatal(err)
			}

			_, status, err := healAgentInstructions(ag, false)
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
		})
	}
}

func TestHealAgentInstructionsIsIdempotent(t *testing.T) {
	for _, ag := range supportedAgents {
		t.Run(ag.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)

			if _, status, err := healAgentInstructions(ag, false); err != nil {
				t.Fatal(err)
			} else if status != "healed" {
				t.Fatalf("first status = %q, want healed", status)
			}
			path, status, err := healAgentInstructions(ag, false)
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
		})
	}
}

func TestHealAgentInstructionsDryRunDoesNotCreateFile(t *testing.T) {
	for _, ag := range supportedAgents {
		t.Run(ag.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)

			path, status, err := healAgentInstructions(ag, true)
			if err != nil {
				t.Fatal(err)
			}
			if status != "would heal" {
				t.Fatalf("status = %q, want would heal", status)
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatalf("dry-run created %s or returned unexpected error: %v", ag.file, err)
			}
		})
	}
}
