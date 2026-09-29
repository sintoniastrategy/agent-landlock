package agentlandlock

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestForceClaudeYoloNormalizesFlags(t *testing.T) {
	cases := [][]string{
		{"claude", "--dangerously-skip-permissions", "--dangerously-skip-permissions", "-p", "prompt"},
		{"claude", "--permission-mode=bypassPermissions", "--allow-dangerously-skip-permissions", "-p", "prompt"},
		{"claude", "--permission-mode", "bypassPermissions", "--settings", "{}", "-p", "prompt"},
	}
	want := []string{"claude", "--dangerously-skip-permissions", "--permission-mode", "bypassPermissions", "--settings", "{}", "-p", "prompt"}
	for _, input := range cases {
		original := slices.Clone(input)
		got, err := forceAgentYolo(input, "claude", map[string]string{}, CommonOptions{})
		if err != nil || !slices.Equal(got, want) {
			t.Fatalf("cmd = %#v, err = %v, want %#v", got, err, want)
		}
		if !slices.Equal(input, original) {
			t.Fatal("modified caller arguments")
		}
		again, err := forceClaudeYolo(got)
		if err != nil || !slices.Equal(again, got) {
			t.Fatalf("normalizing twice changed arguments: %#v, %v", again, err)
		}
	}
}

func TestForceClaudeYoloRejectsConflicts(t *testing.T) {
	for _, args := range [][]string{
		{"--permission-mode", "plan"}, {"--permission-mode=auto"},
		{"--permission-mode"}, {"--permission-mode="},
		{"--settings"}, {"--settings="}, {"--settings", "--restricted"},
		{"--restricted"}, {"--cloud=task"}, {"--teleport"},
		{"--environment", "remote"}, {"--bg"}, {"--background"},
		{"--dangerously-skip-permissions=false"},
		{"attach", "session"}, {"agents"}, {"respawn"}, {"ultrareview"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			input := append([]string{"claude"}, args...)
			_, err := forceAgentYolo(input, "claude", map[string]string{}, CommonOptions{})
			if err == nil || exitCode(err) != ExitUsage {
				t.Fatalf("expected usage error, got %v", err)
			}
			got, err := forceAgentYolo(input, "claude", map[string]string{}, CommonOptions{NoYolo: true})
			if err != nil || !slices.Equal(input, got) {
				t.Fatalf("--no-yolo changed arguments: %#v, %v", got, err)
			}
		})
	}
}

func TestForceClaudeYoloPreservesArguments(t *testing.T) {
	for _, args := range [][]string{
		{"-p", "--", "--restricted", "--settings", "not-a-file"},
		{"--system-prompt", "--restricted", "-p", "prompt"},
		{"--model", "agents", "-p", "prompt"},
		{"--agents", `{"worker":{"prompt":"--cloud"}}`, "-p", "prompt"},
		{"--resume=previous", "continue"},
	} {
		input := append([]string{"/opt/claude/bin/claude"}, args...)
		got, err := forceAgentYolo(input, inferAgent(input), map[string]string{}, CommonOptions{})
		if err != nil || got[0] != input[0] || !slices.Equal(got[6:], args) {
			t.Fatalf("original arguments not preserved: %#v, %v", got, err)
		}
	}
}

func TestClaudeYoloSettingsPreserveCustomSettings(t *testing.T) {
	workdir := t.TempDir()
	input := `{"sandbox":{"enabled":true,"allowUnsandboxedCommands":false,"network":{"allowedDomains":["example.com"]}},"permissions":{"defaultMode":"plan","additionalDirectories":["/tmp/cache"]},"skipDangerousModePermissionPrompt":false,"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"true"}]}]},"env":{"PRIVATE_VALUE":"keep-private"},"customNumber":9007199254740993}`
	path := filepath.Join(workdir, "settings.json")
	if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{input, "settings.json", path} {
		cmd, err := forceClaudeYolo([]string{"claude", "--settings", "ignored-missing-file", "--settings=" + value, "-p", "prompt"})
		if err != nil {
			t.Fatal(err)
		}
		got, cleanup, err := prepareClaudeYoloSettings(cmd, workdir, false)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(cleanup)
		if strings.Contains(strings.Join(got, " "), "keep-private") {
			t.Fatal("settings contents exposed in argv")
		}
		info, err := os.Stat(got[5])
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("temporary settings permissions: %v, %v", info, err)
		}
		data, err := os.ReadFile(got[5])
		if err != nil {
			t.Fatal(err)
		}
		var settings struct {
			Sandbox struct {
				Enabled                  bool
				AllowUnsandboxedCommands bool
				Network                  json.RawMessage
			}
			Permissions struct {
				DefaultMode           string
				AdditionalDirectories []string
			}
			SkipDangerousModePermissionPrompt bool
			Hooks                             json.RawMessage
			Env                               map[string]string
			CustomNumber                      json.Number
		}
		if err := json.Unmarshal(data, &settings); err != nil {
			t.Fatal(err)
		}
		if settings.Sandbox.Enabled || !settings.Sandbox.AllowUnsandboxedCommands || settings.Permissions.DefaultMode != "bypassPermissions" || !settings.SkipDangerousModePermissionPrompt {
			t.Fatalf("YOLO overrides missing: %s", data)
		}
		if len(settings.Hooks) == 0 || len(settings.Sandbox.Network) == 0 || !slices.Equal(settings.Permissions.AdditionalDirectories, []string{"/tmp/cache"}) || settings.Env["PRIVATE_VALUE"] != "keep-private" || settings.CustomNumber.String() != "9007199254740993" {
			t.Fatalf("custom settings lost: %s", data)
		}
		cleanup()
		if _, err := os.Stat(got[5]); !os.IsNotExist(err) {
			t.Fatalf("temporary settings not removed: %v", err)
		}
		original, err := os.ReadFile(path)
		if err != nil || string(original) != input {
			t.Fatalf("source settings changed: %v", err)
		}
	}
}

func TestClaudeYoloSettingsRejectInvalidInput(t *testing.T) {
	for _, input := range []string{"missing.json", "{bad", `{"sandbox":true}`, `{"permissions":null}`} {
		cmd, err := forceClaudeYolo([]string{"claude", "--settings", input})
		if err != nil {
			t.Fatal(err)
		}
		_, _, err = prepareClaudeYoloSettings(cmd, t.TempDir(), false)
		if err == nil || exitCode(err) != ExitUsage {
			t.Fatalf("expected usage error for %q, got %v", input, err)
		}
	}
}

func TestClaudeYoloSettingsDryRunRedactsSettings(t *testing.T) {
	cmd, err := forceClaudeYolo([]string{"claude", "--settings", `{"env":{"PRIVATE_VALUE":"keep-private"}}`})
	if err != nil {
		t.Fatal(err)
	}
	got, cleanup, err := prepareClaudeYoloSettings(cmd, t.TempDir(), true)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if strings.Contains(strings.Join(got, " "), "keep-private") || !strings.Contains(got[5], "sandbox=off") {
		t.Fatalf("unexpected dry-run settings: %#v", got)
	}
}
