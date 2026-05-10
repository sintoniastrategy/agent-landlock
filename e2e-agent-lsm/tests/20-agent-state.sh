#!/usr/bin/env bash
set -Eeuo pipefail

source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
trap cleanup_fixture EXIT

new_fixture
require_landlock

bin_dir="$FIXTURE/bin"
mkdir -p "$bin_dir"
cat >"$bin_dir/claude" <<'SH'
#!/usr/bin/env bash
set -Eeuo pipefail
mkdir -p "$HOME/.claude"
printf state > "$HOME/.claude/state-write.txt"
SH
chmod +x "$bin_dir/claude"

PATH="$bin_dir:$PATH" "$AGENT_LSM_BIN" -d "$PROJECT" claude -- --version
assert_file "$HOME/.claude/state-write.txt"

if PATH="$bin_dir:$PATH" "$AGENT_LSM_BIN" --no-agent-state -d "$PROJECT" claude -- --version 2>/tmp/agent-lsm-state-deny.err; then
  fail "agent state write succeeded with --no-agent-state"
fi
