package agentlandlock

import "testing"

func TestParseConfigText(t *testing.T) {
	kv := ParseConfigText(`
# comment
SAFETY_DENY_PATHS="/ /etc /root"
EXTRA_ENV="FOO=hello BAR=baz"
BAD line
`)
	if kv["SAFETY_DENY_PATHS"] != "/ /etc /root" {
		t.Fatalf("SAFETY_DENY_PATHS = %q", kv["SAFETY_DENY_PATHS"])
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
		"SAFETY_DENY_PATHS":        "/ /etc /root",
		"AGENT_LANDLOCK_EXTRA_ENV": "FOO=bar",
	})
	if !sameStrings(cfg.SafetyDenyPaths, []string{"/", "/etc", "/root"}) {
		t.Fatalf("SafetyDenyPaths = %#v", cfg.SafetyDenyPaths)
	}
	if cfg.ExtraEnv != "FOO=bar" {
		t.Fatalf("ExtraEnv = %q", cfg.ExtraEnv)
	}
}
