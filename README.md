# agent-landlock

Run AI coding agents under a Linux Landlock write boundary.

`agent-landlock` runs the child process as the current Unix user, keeps normal file
ownership, and uses the kernel Landlock LSM to make the host filesystem
read-only except for explicitly writable directories.

## Model

The boundary is write-oriented, not read-oriented.

- The process can read the same files the current user can read.
- The process can write the current project directory selected by `--dir` or
  `$PWD`.
- The process can write additional directories passed with `--grant`.
- Known agents get their own state directory writable by default:
  `~/.claude`, `~/.codex`, or `~/.gemini`.
- Persistent grants are records under `~/.local/state/agent-landlock/grants.json`.
  They do not mutate filesystem ACLs.

Landlock is process-local. There is no paired Unix user, sudoers drop-in,
recursive `setfacl`, bwrap mount namespace, or cleanup pass.

`agent-landlock` fails closed if Landlock is unavailable or if the kernel exposes an
ABI older than v3. ABI v3 is required so truncation is restricted.

## Usage

```sh
go build -o agent-landlock ./cmd/agent-landlock

agent-landlock                         # default agent, claude
agent-landlock codex --model gpt-5.2
agent-landlock run -- pytest -x
agent-landlock -d ~/src/project run -- npm test
agent-landlock -g ~/.cache/my-tool run -- ./build.sh

agent-landlock grant ~/.npm            # persistent writable path
agent-landlock grants
agent-landlock revoke ~/.npm
agent-landlock doctor
```

Known agent invocations force no-prompt mode unless `--no-yolo` is passed:

- Claude: `--dangerously-skip-permissions`
- Codex: `--dangerously-bypass-approvals-and-sandbox`
- Gemini: `--approval-mode yolo --skip-trust`, plus `GEMINI_SANDBOX=false`

## Config

Config search order:

1. built-in defaults
2. `/etc/agent-landlock/config`
3. `~/.config/agent-landlock/config`
4. environment variables with the `AGENT_LANDLOCK_` prefix

Supported keys:

```sh
DEFAULT_AGENT=claude
SAFETY_DENY_PATHS="/ /etc /var /usr /opt /boot /dev /proc /sys /root"
EXTRA_ENV='RUSTC_WRAPPER=sccache'
```

Environment equivalents include `AGENT_LANDLOCK_DEFAULT_AGENT`,
`AGENT_LANDLOCK_SAFETY_DENY_PATHS`, and `AGENT_LANDLOCK_EXTRA_ENV`.

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

It checks project writes, runtime grants, persistent grants, grant revocation,
known-agent state directories, forced YOLO argv/env, config-file defaults,
parallel runs, status/doctor output, safety-denied paths, and timed grant
cleanup. It requires a Linux kernel with Landlock ABI v3 or newer. The Docker
runner uses an unconfined seccomp profile because some default container
profiles block Landlock syscalls.

## Landlock Notes

`agent-landlock` grants read access to `/` and write access only to selected writable
roots. This preserves normal tool visibility while blocking file creation,
write-open, truncation, unlink, rename, and other directory mutations outside
those roots.

Landlock does not currently restrict every metadata operation. Kernel
documentation calls out operations such as `stat`, `chmod`, `chown`, `utime`,
some `ioctl`, and `fcntl` as outside current filesystem access controls. This
tool is designed to stop ungranted filesystem writes, not to hide secrets or
provide a container boundary.
