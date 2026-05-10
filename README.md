# agent-landlock

Run AI coding agents under a Linux Landlock write boundary.

The wrapper keeps the agent as your Unix user with normal file ownership, then
asks the kernel Landlock LSM to make the host filesystem read-only except for
the workspace and any paths you explicitly grant. No paired user, no sudoers
drop-in, no recursive ACL pass, no bwrap mount namespace, no cleanup.

## Requirements

- Linux kernel with Landlock ABI v3 or newer (≥ 6.2). `agent-landlock` fails
  closed if Landlock is unavailable or if the kernel exposes an older ABI;
  v3 is required for write-truncation control.
- Go 1.24+ to build.

## Install

```sh
go build -o agent-landlock ./cmd/agent-landlock
```

## Quick start

```sh
agent-landlock doctor                  # verify Landlock is available
agent-landlock doctor --heal           # initialize state and repair ~/.claude/CLAUDE.md

agent-landlock claude                  # start Claude interactively
agent-landlock claude -p "summarize this repo"
agent-landlock codex exec "summarize this repo"
agent-landlock run -- pytest -x        # arbitrary command under Landlock

agent-landlock -d ~/src/project run -- npm test
agent-landlock -g ~/.cache/my-tool run -- ./build.sh
```

Running `agent-landlock` with no command prints help. If an interactive agent
launch appears to hang, the agent itself is waiting for input or stuck during
its own startup — try `claude -p` or `codex exec` to isolate that from the
wrapper.

## How it works

The boundary is **write-oriented, not read-oriented**. Landlock is
process-local — there is no host state to clean up.

**Reads** — the process sees what your user can normally see across `/`.

**Writes** are allowed in:

- The project directory selected by `--dir` or `$PWD`.
- Any path passed with `--grant` (one-shot) or recorded by `agent-landlock grant`
  (persistent, stored as records under `~/.local/state/agent-landlock/grants.json` —
  does not mutate filesystem ACLs).
- System-managed roots: `/tmp`, `/dev`, `/proc`, `/sys`, `/run`, `/etc`, `/usr`,
  `/var`, `/media`, `/mnt`. These remain writable only to the extent normal
  Unix permissions, ACLs, mode bits, and device permissions already allow —
  Landlock never makes them more permissive.
- Known-agent state directories: `~/.claude`, `~/.codex`, `~/.gemini`. Claude
  additionally gets `~/.local/state/claude` for its runtime locks.

**Blocked** in any ungranted user or workspace path: file creation, write-open,
truncation, unlink, rename, and other directory mutations.

### Caveat

Landlock does not currently restrict every metadata operation. Kernel
documentation calls out `stat`, `chmod`, `chown`, `utime`, some `ioctl`, and
`fcntl` as outside its filesystem access controls. `agent-landlock` is designed
to stop ungranted filesystem writes — not to hide secrets or provide a
container boundary.

## Agent wrappers

Known-agent subcommands force no-prompt mode unless `--no-yolo` is passed:

| Agent     | Forced flags / env                                                       |
|-----------|--------------------------------------------------------------------------|
| `claude`  | `--dangerously-skip-permissions`                                         |
| `codex`   | `--dangerously-bypass-approvals-and-sandbox`                             |
| `gemini`  | `--approval-mode yolo --skip-trust`, plus `GEMINI_SANDBOX=false`         |

For `claude` specifically the wrapper also sets `CLAUDE_CONFIG_DIR=~/.claude`
(unless already set) and, if `~/.claude.json` exists but
`~/.claude/.claude.json` does not, copies the top-level config into the
writable Claude state directory before Landlock is applied. This avoids
granting write access to all of `$HOME` just so Claude can update its config.

`agent-landlock doctor --heal` repairs a managed runtime-instructions block
inside `~/.claude/CLAUDE.md` so global Claude sessions know to keep fallback
writes inside the current workspace when Landlock denies an outside path. User
content outside that marked block is preserved.

## Persistent grants

```sh
agent-landlock grant ~/.npm            # add
agent-landlock grants                  # list
agent-landlock revoke ~/.npm           # remove
```

## Config

Search order:

1. built-in defaults
2. `/etc/agent-landlock/config`
3. `~/.config/agent-landlock/config`
4. environment variables prefixed `AGENT_LANDLOCK_`

Supported keys:

```sh
SAFETY_DENY_PATHS="/ /root"
EXTRA_ENV='RUSTC_WRAPPER=sccache'
```

Environment equivalents: `AGENT_LANDLOCK_SAFETY_DENY_PATHS`,
`AGENT_LANDLOCK_EXTRA_ENV`.

## Tests

```sh
GOCACHE=/tmp/agent-landlock-gocache go test ./...
```

Landlock e2e suite (requires kernel ABI v3+):

```sh
test/e2e/run.sh
test/e2e/docker-run.sh test
```

`test/e2e/run.sh` builds with a local temporary binary and sets
`GOCACHE=/tmp/agent-landlock-gocache` automatically. It covers project writes,
runtime grants, persistent grants, grant revocation, known-agent state
directories, forced YOLO argv/env, config-file defaults, parallel runs,
status/doctor output, safety-denied paths, and timed grant cleanup. The Docker
runner uses an unconfined seccomp profile because some default container
profiles block Landlock syscalls.

Real-agent tests can use host credentials and make live model requests. They
run as part of `test/e2e/run.sh` by default and fail if selected tools are not
installed, authenticated, reachable, responsive within the per-agent timeout,
or able to follow the boundary-write prompt. They may spend API quota:

```sh
test/e2e/run.sh --skip-real-agents
test/e2e/run.sh --real-agent-tools claude,codex --real-agent-timeout 120
```

The real-agent test preserves the host `HOME`/auth state, creates a temporary
project and outside directory, and asks each selected agent to perform one
allowed write plus one forbidden outside write. Use
`--keep-real-agent-fixture` to keep the temp files after a run.
