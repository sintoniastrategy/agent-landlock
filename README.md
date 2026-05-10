# agent-landlock

Run AI coding agents under a Linux Landlock write boundary.

`agent-landlock` runs the child process as the current Unix user, keeps normal file
ownership, and uses the kernel Landlock LSM to make the host filesystem
read-only except for explicitly writable directories and system-managed paths.

## Model

The boundary is write-oriented, not read-oriented.

- The process can read the same files the current user can read.
- The process can write the current project directory selected by `--dir` or
  `$PWD`.
- The process can write additional directories passed with `--grant`.
- System-managed paths such as `/tmp`, `/dev`, `/proc`, `/sys`, `/run`, `/etc`,
  `/usr`, `/var`, `/media`, and `/mnt` remain writable according to normal Unix
  permissions.
- Known agents get their own state directory writable by default:
  `~/.claude`, `~/.codex`, or `~/.gemini`. Claude also gets
  `~/.local/state/claude` for its runtime locks.
- Persistent grants are records under `~/.local/state/agent-landlock/grants.json`.
  They do not mutate filesystem ACLs.

Landlock is process-local. There is no paired Unix user, sudoers drop-in,
recursive `setfacl`, bwrap mount namespace, or cleanup pass.

`agent-landlock` fails closed if Landlock is unavailable or if the kernel exposes an
ABI older than v3. ABI v3 is required so truncation is restricted.

## Usage

```sh
go build -o agent-landlock ./cmd/agent-landlock

agent-landlock doctor                  # verify Landlock is available
agent-landlock                         # show help

agent-landlock claude                  # start Claude interactively
agent-landlock claude -p "summarize this repo"
agent-landlock codex exec "summarize this repo"
agent-landlock run -- pytest -x        # run an arbitrary command under Landlock
agent-landlock -d ~/src/project run -- npm test
agent-landlock -g ~/.cache/my-tool run -- ./build.sh

agent-landlock grant ~/.npm            # persistent writable path
agent-landlock grants
agent-landlock revoke ~/.npm
```

Running `agent-landlock` with no command prints help. Start agents explicitly,
for example `agent-landlock claude` or `agent-landlock codex`. If an explicit
interactive agent launch appears to hang, the underlying agent is waiting for
input or blocked during its own startup. Use a non-interactive agent subcommand
such as `claude -p` or `codex exec` to isolate that from the wrapper.

Known agent invocations force no-prompt mode unless `--no-yolo` is passed:

- Claude: `--dangerously-skip-permissions`
- Codex: `--dangerously-bypass-approvals-and-sandbox`
- Gemini: `--approval-mode yolo --skip-trust`, plus `GEMINI_SANDBOX=false`

For Claude, `agent-landlock` also sets `CLAUDE_CONFIG_DIR=~/.claude` unless
that variable is already set. If `~/.claude.json` exists and
`~/.claude/.claude.json` does not, the top-level file is copied into the
writable Claude state directory before Landlock is applied. This avoids granting
write access to all of `$HOME` just so Claude can update its config.

## Config

Config search order:

1. built-in defaults
2. `/etc/agent-landlock/config`
3. `~/.config/agent-landlock/config`
4. environment variables with the `AGENT_LANDLOCK_` prefix

Supported keys:

```sh
SAFETY_DENY_PATHS="/ /root"
EXTRA_ENV='RUSTC_WRAPPER=sccache'
```

Environment equivalents include `AGENT_LANDLOCK_SAFETY_DENY_PATHS` and
`AGENT_LANDLOCK_EXTRA_ENV`.

## Tests

The Go tests are separate from the old Python ACL tests:

```sh
GOCACHE=/tmp/agent-landlock-gocache go test ./...
```

The Landlock e2e suite is separate too:

```sh
test/e2e/run.sh
test/e2e/docker-run.sh test
```

`test/e2e/run.sh` builds with a local temporary binary and sets
`GOCACHE=/tmp/agent-landlock-gocache` automatically if `GOCACHE` is not already
set. It checks project writes, runtime grants, persistent grants, grant revocation,
known-agent state directories, forced YOLO argv/env, config-file defaults,
parallel runs, status/doctor output, safety-denied paths, and timed grant
cleanup. It requires a Linux kernel with Landlock ABI v3 or newer. The Docker
runner uses an unconfined seccomp profile because some default container
profiles block Landlock syscalls.

Real-agent e2e tests can use host credentials and make live model requests.
They run as part of `test/e2e/run.sh` by default and fail if selected tools are
not installed, authenticated, reachable, responsive before the per-agent timeout,
or able to follow the boundary-write prompt. They may spend API quota when host
auth is available:

```sh
test/e2e/run.sh
test/e2e/run.sh --skip-real-agents
test/e2e/run.sh --real-agent-tools claude,codex --real-agent-timeout 120
```

The real-agent test preserves the host `HOME`/auth state, creates a temporary
project and outside directory, and asks each selected agent to perform one
allowed write plus one forbidden outside write. Use `--skip-real-agents` to
disable this live test on machines without usable real-agent auth. Use
`--keep-real-agent-fixture` to keep the temp files after a run.

## Landlock Notes

`agent-landlock` grants read access to `/`, write access to selected writable
roots, and write access to system-managed roots such as `/tmp`, `/dev`, `/proc`,
`/sys`, `/run`, `/etc`, `/usr`, `/var`, `/media`, and `/mnt`. The system roots
are still controlled by Unix ownership, mode bits, ACLs, groups, and device
permissions; Landlock does not make them more writable than they already are.
This preserves normal tool visibility and runtime behavior while blocking file
creation, write-open, truncation, unlink, rename, and other directory mutations
in ungranted user/workspace paths.

Landlock does not currently restrict every metadata operation. Kernel
documentation calls out operations such as `stat`, `chmod`, `chown`, `utime`,
some `ioctl`, and `fcntl` as outside current filesystem access controls. This
tool is designed to stop ungranted filesystem writes, not to hide secrets or
provide a container boundary.
