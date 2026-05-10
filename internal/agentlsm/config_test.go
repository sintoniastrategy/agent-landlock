package agentlsm

import "testing"

func TestParseConfigText(t *testing.T) {
	kv := ParseConfigText(`
# comment
DEFAULT_AGENT=gemini
EXTRA_ENV="FOO=hello BAR=baz"
BAD line
`)
	if kv["DEFAULT_AGENT"] != "gemini" {
		t.Fatalf("DEFAULT_AGENT = %q", kv["DEFAULT_AGENT"])
	}
	if kv["EXTRA_ENV"] != "FOO=hello BAR=baz" {
		t.Fatalf("EXTRA_ENV = %q", kv["EXTRA_ENV"])
	}
}

func TestShellFieldsQuotes(t *testing.T) {
	got, err := shellFields(`FOO="hello world" BAR=baz EMPTY=''`)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"FOO=hello world", "BAR=baz", "EMPTY="}
	if !sameStrings(got, want) {
		t.Fatalf("fields = %#v, want %#v", got, want)
	}
}

func TestApplyConfigEnvNames(t *testing.T) {
	cfg := DefaultConfig()
	applyConfig(&cfg, map[string]string{
		"DEFAULT_AGENT":       "codex",
		"SAFETY_DENY_PATHS":   "/ /etc /root",
		"AGENT_LSM_EXTRA_ENV": "FOO=bar",
	})
	if cfg.DefaultAgent != "codex" {
		t.Fatalf("DefaultAgent = %q", cfg.DefaultAgent)
	}
	if !sameStrings(cfg.SafetyDenyPaths, []string{"/", "/etc", "/root"}) {
		t.Fatalf("SafetyDenyPaths = %#v", cfg.SafetyDenyPaths)
	}
	if cfg.ExtraEnv != "FOO=bar" {
		t.Fatalf("ExtraEnv = %q", cfg.ExtraEnv)
	}
}
