package agentlandlock

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

type App struct {
	Config Config
	Stdout io.Writer
	Stderr io.Writer
}

func Main(argv []string) (int, error) {
	cfg := LoadConfig()
	inv, err := ParseArgs(argv)
	if err != nil {
		return exitCode(err), err
	}
	app := App{Config: cfg, Stdout: os.Stdout, Stderr: os.Stderr}
	return app.Run(inv)
}

func exitCode(err error) int {
	var ee *ExitError
	if errors.As(err, &ee) {
		return ee.Code
	}
	return ExitGeneric
}

func (a App) Run(inv Invocation) (int, error) {
	switch inv.Command {
	case "agent":
		return a.runAgent(inv)
	case "run":
		return a.runCommand(inv)
	case "shell":
		return a.runShell(inv)
	case "grant":
		return a.grant(inv)
	case "revoke":
		return a.revoke(inv)
	case "grants":
		return a.grants(inv)
	case "status":
		return a.status(inv)
	case "doctor":
		return a.doctor(inv)
	case "bootstrap":
		return a.bootstrap(inv)
	case "help":
		fmt.Fprint(a.Stdout, helpText())
		return ExitOK, nil
	default:
		return ExitUsage, exitError(ExitUsage, "unknown command: "+inv.Command)
	}
}

func (a App) runAgent(inv Invocation) (int, error) {
	if !KnownAgents[inv.Agent] {
		return ExitUsage, exitError(ExitUsage, "unknown agent: "+inv.Agent)
	}
	cmd := append([]string{inv.Agent}, inv.Args...)
	return a.execute(inv, cmd, inv.Agent)
}

func (a App) runCommand(inv Invocation) (int, error) {
	return a.execute(inv, inv.Args, inferAgent(inv.Args))
}

func (a App) runShell(inv Invocation) (int, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	return a.execute(inv, []string{shell, "-l"}, "")
}

func (a App) execute(inv Invocation, cmdArgs []string, agent string) (int, error) {
	if len(cmdArgs) == 0 {
		return ExitUsage, exitError(ExitUsage, "missing command")
	}
	workdirRaw := inv.Common.Dir
	if workdirRaw == "" {
		var err error
		workdirRaw, err = os.Getwd()
		if err != nil {
			return ExitGeneric, err
		}
	}
	workdir, err := resolveExistingDir(workdirRaw)
	if err != nil {
		return ExitUsage, err
	}
	if err := safetyCheckWritable(workdir, a.Config, inv.Common.Force); err != nil {
		return exitCode(err), err
	}
	env, err := runtimeEnv(a.Config)
	if err != nil {
		return ExitUsage, err
	}
	if err := prepareAgentEnv(agent, env, inv.Common.NoAgentState, inv.Common.DryRun); err != nil {
		return exitCode(err), err
	}
	cmdArgs, err = forceAgentYolo(cmdArgs, agent, env, inv.Common.NoYolo)
	if err != nil {
		return exitCode(err), err
	}
	writable, err := a.writableRoots(inv, workdir, agent)
	if err != nil {
		return exitCode(err), err
	}
	policy := SandboxPolicy{
		ReadOnlyRoot:   true,
		Writable:       writable,
		SystemWritable: systemWritableRoots(defaultSystemWritablePaths),
	}
	if inv.Common.DryRun {
		a.printDryRun(workdir, cmdArgs, policy, env)
		return ExitOK, nil
	}
	if err := applySandbox(policy); err != nil {
		return exitCode(err), err
	}
	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	cmd.Dir = workdir
	cmd.Env = envList(env)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err == nil {
		return ExitOK, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			if status.Signaled() {
				return 128 + int(status.Signal()), nil
			}
		}
		code := exitErr.ExitCode()
		if code >= 0 {
			return code, nil
		}
	}
	return ExitGeneric, err
}

func (a App) writableRoots(inv Invocation, workdir string, agent string) ([]string, error) {
	writable := []string{workdir}
	for _, raw := range inv.Common.RuntimeGrant {
		path, err := resolveExistingDir(raw)
		if err != nil {
			return nil, err
		}
		if err := safetyCheckWritable(path, a.Config, inv.Common.Force); err != nil {
			return nil, err
		}
		writable = append(writable, path)
	}
	grants, _, err := activeGrants(time.Now().UTC())
	if err != nil {
		return nil, err
	}
	for _, grant := range grants {
		path, err := resolveExistingDir(grant.Path)
		if err != nil {
			continue
		}
		if err := safetyCheckWritable(path, a.Config, inv.Common.Force); err != nil {
			continue
		}
		writable = append(writable, path)
	}
	stateDirs, err := ensureAgentStateDirs(agent, inv.Common.NoAgentState || agent == "", inv.Common.DryRun)
	if err != nil {
		return nil, err
	}
	writable = append(writable, stateDirs...)
	return uniquePruned(writable), nil
}

func (a App) printDryRun(workdir string, cmdArgs []string, policy SandboxPolicy, env map[string]string) {
	fmt.Fprintf(a.Stdout, "DRY-RUN: chdir %s\n", shellQuote(workdir))
	fmt.Fprintf(a.Stdout, "DRY-RUN: landlock read-only /\n")
	fmt.Fprintf(a.Stdout, "DRY-RUN: landlock writable roots:\n")
	for _, path := range policy.Writable {
		fmt.Fprintf(a.Stdout, "  %s\n", path)
	}
	fmt.Fprintf(a.Stdout, "DRY-RUN: landlock system writable roots:\n")
	for _, path := range policy.SystemWritable {
		fmt.Fprintf(a.Stdout, "  %s\n", path)
	}
	if value := env["CLAUDE_CONFIG_DIR"]; value != "" {
		fmt.Fprintf(a.Stdout, "DRY-RUN: env CLAUDE_CONFIG_DIR=%s\n", shellQuote(value))
	}
	fmt.Fprintf(a.Stdout, "DRY-RUN: exec")
	for _, arg := range cmdArgs {
		fmt.Fprintf(a.Stdout, " %s", shellQuote(arg))
	}
	fmt.Fprintln(a.Stdout)
}

func (a App) grant(inv Invocation) (int, error) {
	targetRaw := inv.Path
	if targetRaw == "" {
		targetRaw = "."
	}
	target, err := resolveExistingDir(targetRaw)
	if err != nil {
		return ExitUsage, err
	}
	if err := safetyCheckWritable(target, a.Config, inv.Common.Force); err != nil {
		return exitCode(err), err
	}
	expiresAt := ""
	if inv.Common.Timeout != "" {
		d, err := parseDuration(inv.Common.Timeout)
		if err != nil {
			return ExitUsage, err
		}
		expiresAt = time.Now().UTC().Add(d).Format(time.RFC3339)
	}
	if inv.Common.DryRun {
		fmt.Fprintf(a.Stdout, "DRY-RUN: grant %s\n", target)
		return ExitOK, nil
	}
	if err := upsertGrant(target, expiresAt); err != nil {
		return ExitGeneric, err
	}
	fmt.Fprintf(a.Stdout, "granted writable path: %s\n", target)
	if expiresAt != "" {
		fmt.Fprintf(a.Stdout, "expires at: %s\n", expiresAt)
	}
	return ExitOK, nil
}

func (a App) revoke(inv Invocation) (int, error) {
	targetRaw := inv.Path
	if targetRaw == "" {
		targetRaw = "."
	}
	target, err := resolvePathForStatus(targetRaw)
	if err != nil {
		return ExitUsage, err
	}
	if inv.Common.DryRun {
		fmt.Fprintf(a.Stdout, "DRY-RUN: revoke %s\n", target)
		return ExitOK, nil
	}
	removed, err := removeGrant(target)
	if err != nil {
		return ExitGeneric, err
	}
	if removed {
		fmt.Fprintf(a.Stdout, "revoked writable path: %s\n", target)
	} else {
		fmt.Fprintf(a.Stdout, "no grant found for: %s\n", target)
	}
	return ExitOK, nil
}

func (a App) grants(inv Invocation) (int, error) {
	if inv.Cleanup {
		removed, err := cleanupExpiredGrants()
		if err != nil {
			return ExitGeneric, err
		}
		fmt.Fprintf(a.Stdout, "cleanup: removed %d expired grant(s)\n", removed)
	}
	grants, expired, err := activeGrants(time.Now().UTC())
	if err != nil {
		return ExitGeneric, err
	}
	if len(grants) == 0 && len(expired) == 0 {
		fmt.Fprintln(a.Stdout, "no persistent grants")
		return ExitOK, nil
	}
	fmt.Fprintln(a.Stdout, "PERSISTENT GRANTS")
	for _, grant := range grants {
		expiry := "-"
		if grant.ExpiresAt != "" {
			expiry = grant.ExpiresAt
		}
		fmt.Fprintf(a.Stdout, "%s  expires=%s  created=%s\n", grant.Path, expiry, grant.CreatedAt)
	}
	if len(expired) > 0 {
		fmt.Fprintln(a.Stdout, "EXPIRED GRANTS")
		for _, grant := range expired {
			fmt.Fprintf(a.Stdout, "%s  expired=%s\n", grant.Path, grant.ExpiresAt)
		}
	}
	return ExitOK, nil
}

func (a App) status(inv Invocation) (int, error) {
	targetRaw := inv.Path
	if targetRaw == "" {
		targetRaw = inv.Common.Dir
	}
	target, err := resolvePathForStatus(targetRaw)
	if err != nil {
		return ExitUsage, err
	}
	state, _ := stateDir()
	abi, abiErr := LandlockABIVersion()
	fmt.Fprintf(a.Stdout, "tool            : %s\n", ToolName)
	fmt.Fprintf(a.Stdout, "target          : %s\n", target)
	fmt.Fprintf(a.Stdout, "state dir       : %s\n", state)
	if abiErr != nil {
		fmt.Fprintf(a.Stdout, "landlock        : unavailable (%v)\n", abiErr)
	} else {
		fmt.Fprintf(a.Stdout, "landlock ABI    : v%d\n", abi)
	}
	grants, expired, err := activeGrants(time.Now().UTC())
	if err == nil {
		fmt.Fprintf(a.Stdout, "active grants   : %d\n", len(grants))
		fmt.Fprintf(a.Stdout, "expired grants  : %d\n", len(expired))
	}
	return ExitOK, nil
}

func (a App) doctor(inv Invocation) (int, error) {
	targetRaw := inv.Path
	if targetRaw == "" {
		targetRaw = inv.Common.Dir
	}
	target, err := resolvePathForStatus(targetRaw)
	if err != nil {
		return ExitUsage, err
	}
	state, stateErr := stateDir()
	abi, abiErr := LandlockABIVersion()
	fmt.Fprintf(a.Stdout, "doctor mode     : ")
	if inv.Heal {
		fmt.Fprintln(a.Stdout, "heal")
	} else {
		fmt.Fprintln(a.Stdout, "check")
	}
	fmt.Fprintf(a.Stdout, "target          : %s\n", target)
	if stateErr == nil {
		fmt.Fprintf(a.Stdout, "state dir       : %s\n", state)
	}
	if abiErr != nil {
		fmt.Fprintf(a.Stdout, "landlock        : unavailable (%v)\n", abiErr)
		return ExitLandlockUnavailable, nil
	}
	fmt.Fprintf(a.Stdout, "landlock ABI    : v%d\n", abi)
	if abi < 3 {
		fmt.Fprintln(a.Stdout, "result          : unsupported; ABI v3+ required")
		return ExitLandlockUnavailable, nil
	}
	if inv.Heal {
		if inv.Common.DryRun {
			if state != "" {
				fmt.Fprintf(a.Stdout, "DRY-RUN: mkdir -p %s\n", state)
			}
		} else {
			if _, err := ensureStateDir(); err != nil {
				return ExitGeneric, err
			}
		}
		for _, ag := range supportedAgents {
			path, status, err := healAgentInstructions(ag, inv.Common.DryRun)
			if err != nil {
				return exitCode(err), err
			}
			fmt.Fprintf(a.Stdout, "global %-9s: %s (%s)\n", ag.file, status, path)
		}
	} else {
		for _, ag := range supportedAgents {
			path, status, err := checkAgentInstructions(ag)
			if err != nil {
				continue
			}
			if status == "ok" {
				fmt.Fprintf(a.Stdout, "global %-9s: ok (%s)\n", ag.file, path)
			} else {
				fmt.Fprintf(a.Stdout, "global %-9s: %s; run doctor --heal (%s)\n", ag.file, status, path)
			}
		}
	}
	fmt.Fprintln(a.Stdout, "result          : ok")
	return ExitOK, nil
}

func (a App) bootstrap(inv Invocation) (int, error) {
	if inv.Undo {
		fmt.Fprintln(a.Stdout, "bootstrap is not required for agent-landlock; no paired user or sudoers dropin exists")
		return ExitOK, nil
	}
	if inv.Common.DryRun {
		dir, _ := stateDir()
		fmt.Fprintf(a.Stdout, "DRY-RUN: mkdir -p %s\n", dir)
		return ExitOK, nil
	}
	dir, err := ensureStateDir()
	if err != nil {
		return ExitGeneric, err
	}
	fmt.Fprintf(a.Stdout, "bootstrap not required; initialized state dir: %s\n", dir)
	return ExitOK, nil
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	if strings.IndexFunc(s, func(r rune) bool {
		return !(r == '/' || r == '.' || r == '-' || r == '_' ||
			(r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') ||
			(r >= 'a' && r <= 'z'))
	}) == -1 {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func helpText() string {
	cmds := []string{"run", "shell", "claude", "codex", "gemini", "grant", "revoke", "grants", "status", "doctor"}
	sort.Strings(cmds)
	return fmt.Sprintf(`agent-landlock: run AI agents under a Linux Landlock write boundary

usage:
  agent-landlock [FLAGS] claude|codex|gemini [-- AGENT_ARGS...]
  agent-landlock [FLAGS] run -- CMD ARGS...
  agent-landlock grant PATH [--timeout=30m]
  agent-landlock revoke PATH
  agent-landlock grants [--cleanup]
  agent-landlock status [DIR]
  agent-landlock doctor [--heal] [DIR]

common flags:
  -d, --dir DIR          writable project directory (default: current directory)
  -g, --grant DIR        extra writable directory for this run; repeatable
      --dry-run          print policy and command without running
      --force            bypass safety-denied writable path checks
      --no-agent-state   do not auto-grant known agent state directory
      --no-yolo          do not inject known agent no-prompt flags

commands: %s
state:    ~/.local/state/agent-landlock/
config:   /etc/agent-landlock/config, ~/.config/agent-landlock/config

note: running agent-landlock with no command prints this help. Start an agent
      explicitly, for example: agent-landlock claude
`, strings.Join(cmds, ", "))
}

func executableName(path string) string {
	return filepath.Base(path)
}
