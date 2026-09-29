package agentlandlock

import (
	"strings"
	"testing"
)

func TestParseNoArgsShowsHelp(t *testing.T) {
	inv, err := ParseArgs(nil)
	if err != nil {
		t.Fatal(err)
	}
	if inv.Command != "help" {
		t.Fatalf("unexpected invocation: %#v", inv)
	}
}

func TestParseTopLevelHelpFlag(t *testing.T) {
	inv, err := ParseArgs([]string{"--help"})
	if err != nil {
		t.Fatal(err)
	}
	if inv.Command != "help" {
		t.Fatalf("command = %s, want help", inv.Command)
	}
}

func TestParseUnknownCommandDoesNotDefaultToAgent(t *testing.T) {
	_, err := ParseArgs([]string{"--print"})
	if err == nil {
		t.Fatal("expected unknown command error")
	}
}

func TestParseAgentPassthroughStopsAtUnknownFlag(t *testing.T) {
	inv, err := ParseArgs([]string{"claude", "--model", "opus", "--dry-run"})
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
	inv, err := ParseArgs([]string{"claude", "--dry-run", "--", "--continue"})
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
	inv, err := ParseArgs([]string{"-g", "/cache", "run", "--", "bash", "-lc", "true"})
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

func TestParseYoloMax(t *testing.T) {
	for _, args := range [][]string{
		{"--yolo-max", "codex", "--", "exec", "prompt"},
		{"codex", "--yolo-max", "--", "exec", "prompt"},
		{"--yolo-max", "codex", "--yolo-max", "exec", "prompt"},
		{"--yolo-max", "run", "--", "codex", "exec", "prompt"},
		{"run", "--yolo-max", "--", "codex", "exec", "prompt"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			inv, err := ParseArgs(args)
			if err != nil {
				t.Fatal(err)
			}
			if !inv.Common.YoloMax || inv.Common.NoYolo {
				t.Fatalf("unexpected options: %#v", inv.Common)
			}
			want := []string{"exec", "prompt"}
			if inv.Command == "run" {
				want = append([]string{"codex"}, want...)
			}
			if !sameStrings(inv.Args, want) {
				t.Fatalf("args = %#v, want %#v", inv.Args, want)
			}
		})
	}
}

func TestParseYoloMaxRejectsNoYolo(t *testing.T) {
	for _, args := range [][]string{
		{"--yolo-max", "--no-yolo", "codex"},
		{"--no-yolo", "--yolo-max", "codex"},
		{"--yolo-max", "codex", "--no-yolo"},
		{"--no-yolo", "codex", "--yolo-max"},
		{"codex", "--yolo-max", "--no-yolo"},
		{"codex", "--no-yolo", "--yolo-max"},
		{"--yolo-max", "run", "--no-yolo", "--", "codex"},
		{"--no-yolo", "run", "--yolo-max", "--", "codex"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			_, err := ParseArgs(args)
			if err == nil || exitCode(err) != ExitUsage {
				t.Fatalf("expected usage error, got %v", err)
			}
		})
	}
}

func TestParseYoloMaxPassthrough(t *testing.T) {
	for _, args := range [][]string{
		{"--", "--yolo-max", "--no-yolo"},
		{"--model", "test", "--yolo-max", "--no-yolo"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			inv, err := ParseArgs(append([]string{"codex"}, args...))
			if err != nil {
				t.Fatal(err)
			}
			if inv.Common.YoloMax || inv.Common.NoYolo {
				t.Fatalf("passthrough changed wrapper options: %#v", inv.Common)
			}
			want := args
			if args[0] == "--" {
				want = args[1:]
			}
			if !sameStrings(inv.Args, want) {
				t.Fatalf("args = %#v, want %#v", inv.Args, want)
			}
		})
	}
}

func TestParseDoctorHeal(t *testing.T) {
	inv, err := ParseArgs([]string{"doctor", "--heal", "/work"})
	if err != nil {
		t.Fatal(err)
	}
	if !inv.Heal || inv.Path != "/work" {
		t.Fatalf("unexpected invocation: %#v", inv)
	}
}

func TestParseGrantPathThenTimeout(t *testing.T) {
	inv, err := ParseArgs([]string{"grant", "/cache", "--timeout=30s"})
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
