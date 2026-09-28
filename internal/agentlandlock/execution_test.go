package agentlandlock

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"syscall"
	"testing"
)

func TestAgentExecutionDenied(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	work := filepath.Join(root, "work")
	for _, path := range []string{home, work} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("missing", filepath.Join(work, "alias")); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		codex  string
		claude string
		want   []string
	}{
		{"defaults", "", "", []string{filepath.Join(home, ".claude"), filepath.Join(home, ".codex")}},
		{"nested", filepath.Join(home, ".codex", "nested"), filepath.Join(home, ".claude", "nested"), []string{filepath.Join(home, ".claude"), filepath.Join(home, ".codex")}},
		{"relative codex", "alias/state", "", []string{filepath.Join(home, ".claude"), filepath.Join(home, ".codex"), filepath.Join(work, "missing", "state")}},
		{"relative claude", "", "alias/state", []string{filepath.Join(home, ".claude"), filepath.Join(home, ".codex"), filepath.Join(work, "missing", "state")}},
		{"shared custom", work, work, []string{filepath.Join(home, ".claude"), filepath.Join(home, ".codex"), work}},
		{"custom ancestor", "", home, []string{home}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := agentExecutionDenied(map[string]string{"HOME": home, "CODEX_HOME": tc.codex, "CLAUDE_CONFIG_DIR": tc.claude}, work)
			slices.Sort(got)
			if err != nil || !slices.Equal(got, tc.want) {
				t.Fatalf("denied = %v, %v; want %v", got, err, tc.want)
			}
		})
	}
}

func TestExecutableRoots(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	denied := []string{filepath.Join(home, ".codex"), filepath.Join(root, "missing", "state")}
	for _, path := range []string{denied[0], filepath.Join(home, "project"), filepath.Join(root, "system")} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(denied[0], filepath.Join(root, "alias")); err != nil {
		t.Fatal(err)
	}
	got, err := executableRoots(root, denied)
	want := []string{filepath.Join(home, "project"), filepath.Join(root, "system")}
	if err != nil || !slices.Equal(got, want) {
		t.Fatalf("executable roots = %v, %v; want %v", got, err, want)
	}
}

func TestAgentExecutionSandbox(t *testing.T) {
	if root := os.Getenv("AGENT_LANDLOCK_EXECUTION_TEST"); root != "" {
		testAgentExecutionSandbox(t, root)
		return
	}
	root := t.TempDir()
	for _, path := range []string{"home/.codex/packages", "home/.claude/local", "project", "custom"} {
		if err := os.MkdirAll(filepath.Join(root, path), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("custom/missing", filepath.Join(root, "custom-link")); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(executable, "-test.run=^TestAgentExecutionSandbox$", "-test.v")
	cmd.Env = append(os.Environ(), "AGENT_LANDLOCK_EXECUTION_TEST="+root)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("sandbox child: %v\n%s", err, output)
	}
}

func testAgentExecutionSandbox(t *testing.T, root string) {
	truePath, err := exec.LookPath("true")
	if err != nil {
		t.Fatal(err)
	}
	binary, err := os.ReadFile(truePath)
	if err != nil {
		t.Fatal(err)
	}
	env := map[string]string{
		"HOME":              filepath.Join(root, "home"),
		"CODEX_HOME":        filepath.Join(root, "custom-link"),
		"CLAUDE_CONFIG_DIR": filepath.Join(root, "custom-claude", "missing"),
	}
	denied, err := agentExecutionDenied(env, root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(filepath.Join(root, "home", ".claude")); err != nil {
		t.Fatal(err)
	}
	if err := applySandbox(SandboxPolicy{
		ReadOnlyRoot:   true,
		Writable:       []string{root},
		SystemWritable: systemWritableRoots(defaultSystemWritablePaths),
		NoExecute:      denied,
	}); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command(truePath).Run(); err != nil {
		t.Fatalf("system executable: %v", err)
	}
	for _, path := range append(slices.Clone(denied), filepath.Join(root, "home", ".codex", "packages", "new-release"), filepath.Join(root, "home", ".claude", "local", "new-release")) {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
		server := filepath.Join(path, "server")
		if err := os.WriteFile(server, binary, 0o700); err != nil {
			t.Fatalf("state write: %v", err)
		}
		if err := exec.Command(server).Run(); !errors.Is(err, syscall.EACCES) {
			t.Fatalf("execution of %s: got %v, want EACCES", server, err)
		}
	}
	project := filepath.Join(root, "project")
	for _, path := range denied {
		alias := filepath.Join(project, "alias")
		if err := os.Symlink(filepath.Join(path, "server"), alias); err != nil {
			t.Fatal(err)
		}
		if err := exec.Command(alias).Run(); !errors.Is(err, syscall.EACCES) {
			t.Fatalf("execution through symlink to %s: got %v, want EACCES", path, err)
		}
		if err := os.Remove(alias); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range append([]string{project}, denied...) {
		from := filepath.Join(path, "from")
		to := filepath.Join(path, "to")
		for _, dir := range []string{from, to} {
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(from, "state"), binary, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(filepath.Join(from, "state"), filepath.Join(to, "state")); err != nil {
			t.Fatalf("cross-directory rename: %v", err)
		}
	}
	if err := exec.Command(filepath.Join(project, "to", "state")).Run(); err != nil {
		t.Fatalf("new project executable: %v", err)
	}
}
