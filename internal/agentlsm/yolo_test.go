package agentlsm

import "testing"

func TestForceCodexYolo(t *testing.T) {
	env := map[string]string{}
	got, err := forceAgentYolo([]string{"codex", "exec"}, "codex", env, false)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"codex", "--dangerously-bypass-approvals-and-sandbox", "exec"}
	if !sameStrings(got, want) {
		t.Fatalf("cmd = %#v, want %#v", got, want)
	}
}

func TestForceGeminiYoloEnv(t *testing.T) {
	env := map[string]string{}
	got, err := forceAgentYolo([]string{"gemini"}, "gemini", env, false)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"gemini", "--approval-mode", "yolo", "--skip-trust"}
	if !sameStrings(got, want) {
		t.Fatalf("cmd = %#v, want %#v", got, want)
	}
	if env["GEMINI_SANDBOX"] != "false" {
		t.Fatalf("GEMINI_SANDBOX = %q", env["GEMINI_SANDBOX"])
	}
}

func TestRefuseNonYoloCodexSandbox(t *testing.T) {
	_, err := forceAgentYolo(
		[]string{"codex", "--sandbox", "read-only"},
		"codex",
		map[string]string{},
		false,
	)
	if err == nil {
		t.Fatal("expected error")
	}
	if exitCode(err) != ExitUsage {
		t.Fatalf("exit code = %d", exitCode(err))
	}
}
