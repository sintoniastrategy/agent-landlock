package agentlandlock

import (
	"slices"
	"testing"
)

func TestForceCodexYoloNormalizesFlags(t *testing.T) {
	cases := [][]string{
		{"codex", "--yolo", "--dangerously-bypass-approvals-and-sandbox", "--no-daemon", "--no-daemon", "exec", "--yolo", "prompt"},
		{"codex", "--sandbox=danger-full-access", "-anever", "exec", "--ask-for-approval", "never", "-s", "danger-full-access", "prompt"},
		{"codex", "-csandbox_mode='danger-full-access'", "--config", `"approval_policy" = "never"`, "exec", "--config=approvals_reviewer=user", "prompt"},
	}
	want, err := forceCodexYolo([]string{"codex", "exec", "prompt"}, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range cases {
		t.Run(input[1], func(t *testing.T) {
			original := slices.Clone(input)
			got, err := forceCodexYolo(input, false)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got, want) {
				t.Fatalf("cmd = %#v, want %#v", got, want)
			}
			if !slices.Equal(input, original) {
				t.Fatal("modified caller arguments")
			}
			again, err := forceCodexYolo(got, false)
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
			_, err := forceAgentYolo(input, "codex", map[string]string{}, CommonOptions{})
			if err == nil || exitCode(err) != ExitUsage {
				t.Fatalf("expected usage error for %#v, got %v", args, err)
			}
		})
	}
}

func TestForceCodexYoloPreservesArguments(t *testing.T) {
	cases := [][]string{
		{"exec", "--", "--yolo", "--sandbox", "read-only"},
		{"exec", "--", "--dangerously-bypass-hook-trust"},
		{"--model", "--yolo", "exec", "prompt"},
		{"--model", "--dangerously-bypass-hook-trust", "exec", "prompt"},
		{"app-server", "--stdio", "--config", "model='test'"},
		{"exec", "--config=developer_instructions='--no-daemon'", "prompt"},
		{"resume", "--last", "continue"},
	}
	for _, args := range cases {
		t.Run(args[0], func(t *testing.T) {
			input := append([]string{"/opt/codex/bin/codex"}, args...)
			got, err := forceAgentYolo(input, inferAgent(input), map[string]string{}, CommonOptions{})
			if err != nil {
				t.Fatal(err)
			}
			want := append([]string{input[0]}, agentYoloArgs["codex"]...)
			want = append(want, args...)
			if !slices.Equal(got, want) {
				t.Fatalf("original arguments not preserved: %#v", got)
			}
		})
	}
}

func TestForceCodexYoloOptOut(t *testing.T) {
	input := []string{"codex", "--sandbox", "read-only", "--ask-for-approval", "on-request"}
	got, err := forceAgentYolo(input, "codex", map[string]string{}, CommonOptions{NoYolo: true})
	if err != nil || !slices.Equal(got, input) {
		t.Fatalf("--no-yolo changed arguments: %#v, %v", got, err)
	}
}

func TestForceCodexYoloHookTrust(t *testing.T) {
	const flag = "--dangerously-bypass-hook-trust"
	cases := []struct {
		name    string
		yoloMax bool
		args    []string
		count   int
	}{
		{name: "default", args: []string{"exec", "prompt"}},
		{name: "max", yoloMax: true, args: []string{"exec", "prompt"}, count: 1},
		{name: "max_app_server", yoloMax: true, args: []string{"app-server", "--stdio"}, count: 1},
		{name: "explicit", args: []string{flag, "exec", flag, "prompt"}, count: 1},
		{name: "max_explicit", yoloMax: true, args: []string{flag, "exec", flag, "prompt"}, count: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := append([]string{"codex"}, tc.args...)
			original := slices.Clone(input)
			options := CommonOptions{YoloMax: tc.yoloMax}
			got, err := forceAgentYolo(input, "codex", map[string]string{}, options)
			if err != nil {
				t.Fatal(err)
			}
			count := 0
			for _, arg := range got {
				if arg == flag {
					count++
				}
			}
			if count != tc.count {
				t.Fatalf("hook trust bypass count = %d, want %d: %#v", count, tc.count, got)
			}
			if !slices.Equal(input, original) {
				t.Fatal("modified caller arguments")
			}
			again, err := forceAgentYolo(got, "codex", map[string]string{}, options)
			if err != nil || !slices.Equal(again, got) {
				t.Fatalf("normalizing twice changed arguments: %#v, %v", again, err)
			}
		})
	}
}
