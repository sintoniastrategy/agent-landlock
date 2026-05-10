# agent-lsm

Run AI coding agents under a Linux Landlock write boundary.

`agent-lsm` runs the child process as the current Unix user, keeps normal file
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
- Persistent grants are records under `~/.local/state/agent-lsm/grants.json`.
  They do not mutate filesystem ACLs.

Landlock is process-local. There is no paired Unix user, sudoers drop-in,
recursive `setfacl`, bwrap mount namespace, or cleanup pass.

`agent-lsm` fails closed if Landlock is unavailable or if the kernel exposes an
ABI older than v3. ABI v3 is required so truncation is restricted.

## Usage

```sh
go build -o agent-lsm ./cmd/agent-lsm

agent-lsm                         # default agent, claude
agent-lsm codex --model gpt-5.2
agent-lsm run -- pytest -x
agent-lsm -d ~/src/project run -- npm test
agent-lsm -g ~/.cache/my-tool run -- ./build.sh

agent-lsm grant ~/.npm            # persistent writable path
agent-lsm grants
agent-lsm revoke ~/.npm
agent-lsm doctor
```

Known agent invocations force no-prompt mode unless `--no-yolo` is passed:

- Claude: `--dangerously-skip-permissions`
- Codex: `--dangerously-bypass-approvals-and-sandbox`
- Gemini: `--approval-mode yolo --skip-trust`, plus `GEMINI_SANDBOX=false`

## Config

Config search order:

1. built-in defaults
2. `/etc/agent-lsm/config`
3. `~/.config/agent-lsm/config`
4. environment variables with the `AGENT_LSM_` prefix

Supported keys:

```sh
DEFAULT_AGENT=claude
SAFETY_DENY_PATHS="/ /etc /var /usr /opt /boot /dev /proc /sys /root"
EXTRA_ENV='RUSTC_WRAPPER=sccache'
```

Environment equivalents include `AGENT_LSM_DEFAULT_AGENT`,
`AGENT_LSM_SAFETY_DENY_PATHS`, and `AGENT_LSM_EXTRA_ENV`.

## Tests

The Go tests are separate from the old Python ACL tests:

```sh
GOCACHE=/tmp/agent-lsm-gocache go test ./...
```

The Landlock e2e suite is separate too:

```sh
e2e-agent-lsm/run.sh
```

It checks project writes, runtime grants, persistent grants, and known-agent
state directories. It requires a Linux kernel with Landlock ABI v3 or newer.

## Landlock Notes

`agent-lsm` grants read access to `/` and write access only to selected writable
roots. This preserves normal tool visibility while blocking file creation,
write-open, truncation, unlink, rename, and other directory mutations outside
those roots.

Landlock does not currently restrict every metadata operation. Kernel
documentation calls out operations such as `stat`, `chmod`, `chown`, `utime`,
some `ioctl`, and `fcntl` as outside current filesystem access controls. This
tool is designed to stop ungranted filesystem writes, not to hide secrets or
provide a container boundary.
