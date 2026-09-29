package agentlandlock

import "testing"

func TestForceCodexYolo(t *testing.T) {
	env := map[string]string{}
	got, err := forceAgentYolo([]string{"codex", "exec"}, "codex", env, CommonOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"codex", "--no-daemon", "--dangerously-bypass-approvals-and-sandbox",
		"-c", `sandbox_mode="danger-full-access"`, "-c", `approval_policy="never"`, "-c", `approvals_reviewer="user"`, "exec",
	}
	if !sameStrings(got, want) {
		t.Fatalf("cmd = %#v, want %#v", got, want)
	}
}

func TestForceGeminiYoloEnv(t *testing.T) {
	env := map[string]string{}
	got, err := forceAgentYolo([]string{"gemini"}, "gemini", env, CommonOptions{})
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
		CommonOptions{},
	)
	if err == nil {
		t.Fatal("expected error")
	}
	if exitCode(err) != ExitUsage {
		t.Fatalf("exit code = %d", exitCode(err))
	}
}

func TestYoloMaxPreservesOtherAgents(t *testing.T) {
	for _, agent := range []string{"claude", "gemini", "bash"} {
		t.Run(agent, func(t *testing.T) {
			env := map[string]string{}
			want, err := forceAgentYolo([]string{agent}, agent, env, CommonOptions{})
			if err != nil {
				t.Fatal(err)
			}
			maxEnv := map[string]string{}
			got, err := forceAgentYolo([]string{agent}, agent, maxEnv, CommonOptions{YoloMax: true})
			if err != nil || !sameStrings(got, want) {
				t.Fatalf("--yolo-max changed arguments: %#v, %v; want %#v", got, err, want)
			}
			if len(maxEnv) != len(env) {
				t.Fatalf("env = %#v, want %#v", maxEnv, env)
			}
			for key, value := range env {
				if maxEnv[key] != value {
					t.Fatalf("env[%q] = %q, want %q", key, maxEnv[key], value)
				}
			}
		})
	}
}
