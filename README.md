# agent-landlock

**Run Claude Code, Codex CLI, and Gemini CLI in YOLO mode — but only inside your project directory.**

A tiny Go wrapper that uses the Linux **Landlock** LSM to make the host
filesystem read-only for the agent process, except for the workspace and the
paths you explicitly grant. No containers. No namespaces. No paired user. No
sudoers drop-in. No cleanup.

```sh
agent-landlock claude                  # Claude Code, --dangerously-skip-permissions, scoped to $PWD
agent-landlock codex exec "fix tests"  # Codex CLI, --dangerously-bypass-approvals-and-sandbox
agent-landlock gemini                  # Gemini CLI, --approval-mode yolo --skip-trust
agent-landlock run -- pytest -x        # any command, write-confined to $PWD
```

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

---

## Why

You want to run the agent in YOLO mode (`--dangerously-skip-permissions`,
`--dangerously-bypass-approvals-and-sandbox`, `--approval-mode yolo`) so it
stops asking you to confirm every file write — but you don't want it touching
anything outside the project you opened.

The existing options all overshoot:

| Approach | Problem |
|---|---|
| **Docker / containers** | Heavy. Breaks USB devices, GPUs, IDE integrations, host networking, file watchers. Persistent volume juggling. You wanted a write boundary, you got a VM. |
| **bwrap / unshare / user namespaces** | Mount-namespace fragility, UID-mapping headaches, breaks tools that introspect `/proc`, breaks editors and language servers. Every distro disagrees on what's allowed. |
| **Built-in agent sandboxes** | Constantly prompt *"this won't work in the sandbox — allow unsandboxed?"*. You end up clicking allow on everything, which defeats the entire point of YOLO. |
| **Just trust the agent** | One bad rename in `$HOME` and your dotfiles are gone. |

The insight: **you don't need a full sandbox.** You don't need namespaces, you
don't need containers, you don't need a separate UID. You almost always just
need **write control** — stop the agent from clobbering files outside the
project. Reads and network are rarely the actual risk, and Linux can scope
those too if you need it (planned for v2).

`agent-landlock` does exactly that one thing, with the kernel's built-in
Landlock LSM, in-process, with zero host state.

## How it works

- The wrapper runs as **your normal Unix user** with normal file ownership.
- Before `exec`-ing the agent, it asks the kernel (Landlock ABI v3+) to make
  the host filesystem **read-only for this process tree, except for** the
  workspace and any paths you grant.
- Landlock is **process-local**: nothing to clean up, nothing to unmount, no
  background daemon.
- Reads still work everywhere your user can normally read — so language
  servers, `git`, `node_modules` resolution, USB devices, GPU, IDE plugins,
  and host networking all keep working.

**Writes are allowed in:**

- The project directory selected by `--dir` or `$PWD`.
- Any path passed with `--grant` (one-shot) or recorded by `agent-landlock grant`
  (persistent — stored as records under
  `~/.local/state/agent-landlock/grants.json`, does **not** mutate filesystem
  ACLs).
- System-managed roots: `/tmp`, `/dev`, `/proc`, `/sys`, `/run`, `/etc`,
  `/usr`, `/var`, `/media`, `/mnt`. These remain writable only to the extent
  normal Unix permissions, ACLs, mode bits, and device permissions already
  allow — Landlock never makes them more permissive.
- Known-agent state directories: `~/.claude`, `~/.codex`, `~/.gemini`. Claude
  additionally gets `~/.local/state/claude` for its runtime locks.

**Blocked** in any ungranted user or workspace path: file creation,
write-open, truncation, unlink, rename, and other directory mutations.

### Caveat

Landlock does not currently restrict every metadata operation. The kernel
documentation calls out `stat`, `chmod`, `chown`, `utime`, some `ioctl`, and
`fcntl` as outside its filesystem access controls. `agent-landlock` is
designed to stop ungranted filesystem **writes** — not to hide secrets, gate
network access, or replace a container boundary.

## Requirements

- Linux kernel with **Landlock ABI v3+** (≥ 6.2). `agent-landlock` fails
  closed if Landlock is unavailable or the kernel exposes an older ABI;
  v3 is required for write-truncation control.
- Go 1.24+ to build from source.

## Install

```sh
go install github.com/sintoniastrategy/agent-landlock/cmd/agent-landlock@latest
```

Or from source:

```sh
git clone https://github.com/sintoniastrategy/agent-landlock
cd agent-landlock
go build -o agent-landlock ./cmd/agent-landlock
```

Prebuilt binaries: see [Releases](https://github.com/sintoniastrategy/agent-landlock/releases).

## Quick start

```sh
agent-landlock doctor                  # verify Landlock is available
agent-landlock doctor --heal           # initialize state and repair ~/.claude/CLAUDE.md

agent-landlock claude                  # start Claude Code in YOLO, scoped
agent-landlock claude -p "summarize this repo"
agent-landlock codex exec "summarize this repo"
agent-landlock gemini                  # Gemini CLI in YOLO, scoped
agent-landlock run -- pytest -x        # any command under Landlock

agent-landlock -d ~/src/project run -- npm test
agent-landlock -g ~/.cache/my-tool run -- ./build.sh
```

Running `agent-landlock` with no command prints help. If an interactive agent
launch appears to hang, the agent itself is waiting for input or stuck during
its own startup — try `claude -p` or `codex exec` to isolate that from the
wrapper.

## Agent wrappers

Known-agent subcommands force no-prompt (YOLO) mode unless `--no-yolo` is passed:

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

## Roadmap

- **v2**: optional read scoping and network egress control. Same philosophy —
  the smallest kernel-level boundary that solves the actual problem, no
  container theater.

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

## License

[MIT](LICENSE) © Sintonia Strategy
