#!/usr/bin/env bash
set -Eeuo pipefail

source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
trap cleanup_fixture EXIT

new_fixture
require_landlock

status=$("$AGENT_LANDLOCK_BIN" status "$PROJECT")
assert_contains "$status" "tool            : agent-landlock"
assert_contains "$status" "state dir       : $XDG_STATE_HOME/agent-landlock"
assert_contains "$status" "landlock ABI"

mkdir -p "$HOME/.claude"
printf '# User Claude memory\n\nKeep this line.\n' >"$HOME/.claude/CLAUDE.md"
doctor=$("$AGENT_LANDLOCK_BIN" doctor --heal "$PROJECT")
assert_contains "$doctor" "doctor mode     : heal"
assert_contains "$doctor" "global CLAUDE.md: healed"
assert_contains "$doctor" "result          : ok"
[[ -d "$XDG_STATE_HOME/agent-landlock" ]] || fail "doctor --heal did not create state dir"
assert_file "$HOME/.claude/CLAUDE.md"
claude_md=$(cat "$HOME/.claude/CLAUDE.md")
assert_contains "$claude_md" "# User Claude memory"
assert_contains "$claude_md" "Keep this line."
assert_contains "$claude_md" "<!-- agent-landlock:runtime-instructions begin -->"
assert_contains "$claude_md" "## agent-landlock Runtime"

doctor_again=$("$AGENT_LANDLOCK_BIN" doctor --heal "$PROJECT")
assert_contains "$doctor_again" "global CLAUDE.md: ok"

set +e
safety=$("$AGENT_LANDLOCK_BIN" grant / 2>&1)
rc=$?
set -e
[[ "$rc" -ne 0 ]] || fail "grant / unexpectedly succeeded"
assert_contains "$safety" "refusing writable access to safety path"

set +e
home_safety=$("$AGENT_LANDLOCK_BIN" grant "$HOME" 2>&1)
rc=$?
set -e
[[ "$rc" -ne 0 ]] || fail "grant HOME unexpectedly succeeded"
assert_contains "$home_safety" "refusing to make all of \$HOME writable"
