package agentlandlock

import (
	"slices"
	"testing"
)

func TestForceCodexYoloNormalizesFlags(t *testing.T) {
	cases := [][]string{
		{"codex", "--yolo", "--dangerously-bypass-approvals-and-sandbox", "--no-daemon", "--no-daemon", "--dangerously-bypass-hook-trust", "exec", "--yolo", "prompt"},
		{"codex", "--sandbox=danger-full-access", "-anever", "exec", "--ask-for-approval", "never", "-s", "danger-full-access", "prompt"},
		{"codex", "-csandbox_mode='danger-full-access'", "--config", `"approval_policy" = "never"`, "exec", "--config=approvals_reviewer=user", "prompt"},
	}
	want, err := forceCodexYolo([]string{"codex", "exec", "prompt"})
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range cases {
		t.Run(input[1], func(t *testing.T) {
			original := slices.Clone(input)
			got, err := forceCodexYolo(input)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got, want) {
				t.Fatalf("cmd = %#v, want %#v", got, want)
			}
			if !slices.Equal(input, original) {
				t.Fatal("modified caller arguments")
			}
			again, err := forceCodexYolo(got)
			if err != nil || !slices.Equal(again, got) {
				t.Fatalf("normalizing twice changed arguments: %#v, %v", again, err)
			}
		})
	}
}

func TestForceCodexYoloRejectsConflicts(t *testing.T) {
	cases := [][]string{
		{"--sandbox", "workspace-write"},
		{"--sandbox=read-only"},
		{"-sread-only"},
		{"-s=read-only"},
		{"--ask-for-approval", "on-request"},
		{"-aon-request"},
		{"--approve-for-me"},
		{"--full-auto"},
		{"--remote", "unix:///tmp/server.sock"},
		{"--remote=ws://localhost:4500"},
		{"-c", `sandbox_mode="workspace-write"`},
		{"--config=approval_policy='on-request'"},
		{"-capprovals_reviewer=auto_review"},
		{"-c", `"approval_policy" = "on-request"`},
		{"-c", "approval_policy.granular.rules=true"},
		{"--sandbox"},
		{"-a"},
		{"--config"},
	}
	for _, args := range cases {
		t.Run(args[0], func(t *testing.T) {
			input := append([]string{"codex", "exec"}, args...)
			_, err := forceAgentYolo(input, "codex", map[string]string{}, false)
			if err == nil || exitCode(err) != ExitUsage {
				t.Fatalf("expected usage error for %#v, got %v", args, err)
			}
		})
	}
}

func TestForceCodexYoloPreservesArguments(t *testing.T) {
	cases := [][]string{
		{"exec", "--", "--yolo", "--sandbox", "read-only"},
		{"--model", "--yolo", "exec", "prompt"},
		{"app-server", "--stdio", "--config", "model='test'"},
		{"exec", "--config=developer_instructions='--no-daemon'", "prompt"},
		{"resume", "--last", "continue"},
	}
	for _, args := range cases {
		t.Run(args[0], func(t *testing.T) {
			input := append([]string{"/opt/codex/bin/codex"}, args...)
			got, err := forceAgentYolo(input, inferAgent(input), map[string]string{}, false)
			if err != nil {
				t.Fatal(err)
			}
			if got[0] != input[0] || !slices.Equal(got[len(got)-len(args):], args) {
				t.Fatalf("original arguments not preserved: %#v", got)
			}
		})
	}
}

func TestForceCodexYoloOptOut(t *testing.T) {
	input := []string{"codex", "--sandbox", "read-only", "--ask-for-approval", "on-request"}
	got, err := forceAgentYolo(input, "codex", map[string]string{}, true)
	if err != nil || !slices.Equal(got, input) {
		t.Fatalf("--no-yolo changed arguments: %#v, %v", got, err)
	}
}
