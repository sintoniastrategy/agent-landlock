# doctor: Claude config-split detection and CLAUDE_CONFIG_DIR heal

Date: 2026-06-04
Status: approved

## Problem

`agent-landlock claude` injects `CLAUDE_CONFIG_DIR=$HOME/.claude`, so landlocked
sessions read/write `~/.claude/.claude.json` (the "bridged" config). A plain
`claude` run has no such env and keeps writing the legacy `~/.claude.json`.
After the one-time first-run migration (copy legacy → bridged), the two files
diverge silently: different MCP servers, project trust, counters. Users expect
landlocked and plain sessions to behave identically.

## Decision summary (user-approved)

- `doctor --heal` writes the export to **`~/.profile` only** (managed marker
  block). No shell-aware detection; zsh users are out of scope.
- An existing diverged legacy file is **detect + warn only**. Heal never
  copies, merges, or archives `~/.claude.json`.
- Any user-managed `CLAUDE_CONFIG_DIR` export found in common rc files counts
  as `ok (user-managed)`; heal then does nothing.

## doctor (check mode) — two new Claude-only rows

1. `claude env`
   - Scan, in order: `~/.profile`, `~/.bash_profile`, `~/.bashrc`,
     `~/.zshenv`, `~/.zprofile`, `~/.zshrc`.
   - Status:
     - `ok` — managed block present in `~/.profile` and current.
     - `ok (user-managed)` — `CLAUDE_CONFIG_DIR` appears outside a managed
       block in any scanned file (commented-out lines do not count).
     - `stale` / `partial` — managed block present but outdated or broken
       (same semantics as the instructions-block heal); suggests `doctor --heal`.
     - `missing` — not found anywhere; suggests `doctor --heal`.
2. `claude config`
   - If both `~/.claude.json` and `~/.claude/.claude.json` exist **and**
     legacy mtime > bridged mtime: warn
     `split: ~/.claude.json written after bridged copy; merge mcpServers/projects manually`.
   - Otherwise `ok`. Detect-only; never healed. The warning self-clears once
     the env fix lands (bridged keeps updating, legacy freezes).

## doctor --heal

- If `claude env` status is `ok` or `ok (user-managed)`: report, do nothing.
- Otherwise upsert the managed block into `~/.profile`:

  ```sh
  # agent-landlock:claude-env begin
  export CLAUDE_CONFIG_DIR="$HOME/.claude"
  # agent-landlock:claude-env end
  ```

- Safety: `refuseSymlink` on `~/.profile`; atomic write via existing
  `writeFileAtomic`; preserve existing file mode, `0o644` when creating.
- `--dry-run` reports `would heal`.
- After healing, print a note that the export takes effect on the next login
  shell.

## Code structure

- Generalize `managedInstructionStatus` / `upsertManagedInstructionBlock` to
  accept begin/end markers as parameters (instructions.go callers pass the
  existing HTML-comment markers; behavior unchanged).
- New `internal/agentlandlock/profileenv.go`:
  - `checkClaudeProfileEnv() (path, status string, err error)`
  - `healClaudeProfileEnv(dryRun bool) (path, status string, err error)`
  - `checkClaudeConfigSplit() (status string, err error)`
- Wire both rows into `App.doctor` check and heal paths (claude-only; codex
  and gemini have no equivalent env mechanism in this tool).

## Testing

`internal/agentlandlock/profileenv_test.go`, temp `HOME` via `t.Setenv`:

- missing everywhere → `missing`; heal creates `~/.profile` with block.
- user-managed export in `~/.bashrc` → `ok (user-managed)`; heal no-ops.
- commented-out export only → still `missing`.
- stale managed block → heal replaces it; user content preserved.
- heal idempotent (second run → `ok`).
- dry-run → `would heal`, file untouched.
- symlinked `~/.profile` → refused with safety error.
- config split: legacy newer → warn; bridged newer or file missing → `ok`.

## Out of scope

- zsh/fish profile targets.
- Merging or archiving the legacy `~/.claude.json`.
- Equivalent mechanisms for codex/gemini.
