#!/usr/bin/env bash
set -Eeuo pipefail

source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
trap cleanup_fixture EXIT

new_fixture
require_landlock

bin_dir="$FIXTURE/bin"
mkdir -p "$bin_dir"
printf home-config > "$HOME/.claude.json"
cat >"$bin_dir/claude" <<'SH'
#!/usr/bin/env bash
set -Eeuo pipefail
mkdir -p "$HOME/.claude"
mkdir -p "$HOME/.local/state/claude/locks"
printf '%s\n' "${CLAUDE_CONFIG_DIR:-}" > "$PROJECT/claude-config-dir.txt"
cat "${CLAUDE_CONFIG_DIR:?}/.claude.json" > "$PROJECT/claude-migrated-config.txt"
printf bridged-config > "$CLAUDE_CONFIG_DIR/.claude.json"
printf state > "$HOME/.claude/state-write.txt"
printf lock > "$HOME/.local/state/claude/locks/state-lock.txt"
SH
chmod +x "$bin_dir/claude"

PROJECT="$PROJECT" PATH="$bin_dir:$PATH" "$AGENT_LANDLOCK_BIN" -d "$PROJECT" claude -- --version
assert_file "$HOME/.claude/state-write.txt"
assert_file "$HOME/.local/state/claude/locks/state-lock.txt"
assert_contains "$(cat "$PROJECT/claude-config-dir.txt")" "$HOME/.claude"
assert_contains "$(cat "$PROJECT/claude-migrated-config.txt")" "home-config"
assert_contains "$(cat "$HOME/.claude/.claude.json")" "bridged-config"
assert_contains "$(cat "$HOME/.claude.json")" "home-config"

if PROJECT="$PROJECT" PATH="$bin_dir:$PATH" "$AGENT_LANDLOCK_BIN" --no-agent-state -d "$PROJECT" claude -- --version 2>/tmp/agent-landlock-state-deny.err; then
  fail "agent state write succeeded with --no-agent-state"
fi
