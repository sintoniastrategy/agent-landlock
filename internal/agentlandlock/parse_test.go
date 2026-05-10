package agentlandlock

import "testing"

func TestParseDefaultAgent(t *testing.T) {
	inv, err := ParseArgs(nil, Config{DefaultAgent: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	if inv.Command != "agent" || inv.Agent != "codex" {
		t.Fatalf("unexpected invocation: %#v", inv)
	}
}

func TestParseAgentPassthroughStopsAtUnknownFlag(t *testing.T) {
	inv, err := ParseArgs([]string{"claude", "--model", "opus", "--dry-run"}, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if inv.Common.DryRun {
		t.Fatal("--dry-run after an agent arg should be passthrough")
	}
	want := []string{"--model", "opus", "--dry-run"}
	if !sameStrings(inv.Args, want) {
		t.Fatalf("args = %#v, want %#v", inv.Args, want)
	}
}

func TestParseAgentWrapperFlagBeforeAgentArgs(t *testing.T) {
	inv, err := ParseArgs([]string{"claude", "--dry-run", "--", "--continue"}, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if !inv.Common.DryRun {
		t.Fatal("expected dry-run wrapper flag")
	}
	want := []string{"--continue"}
	if !sameStrings(inv.Args, want) {
		t.Fatalf("args = %#v, want %#v", inv.Args, want)
	}
}

func TestParseRunWithRuntimeGrant(t *testing.T) {
	inv, err := ParseArgs([]string{"-g", "/cache", "run", "--", "bash", "-lc", "true"}, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if inv.Command != "run" {
		t.Fatalf("command = %s", inv.Command)
	}
	if !sameStrings(inv.Common.RuntimeGrant, []string{"/cache"}) {
		t.Fatalf("runtime grants = %#v", inv.Common.RuntimeGrant)
	}
	if !sameStrings(inv.Args, []string{"bash", "-lc", "true"}) {
		t.Fatalf("args = %#v", inv.Args)
	}
}

func TestParseDoctorHeal(t *testing.T) {
	inv, err := ParseArgs([]string{"doctor", "--heal", "/work"}, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if !inv.Heal || inv.Path != "/work" {
		t.Fatalf("unexpected invocation: %#v", inv)
	}
}

func TestParseGrantPathThenTimeout(t *testing.T) {
	inv, err := ParseArgs([]string{"grant", "/cache", "--timeout=30s"}, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if inv.Command != "grant" || inv.Path != "/cache" || inv.Common.Timeout != "30s" {
		t.Fatalf("unexpected invocation: %#v", inv)
	}
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
