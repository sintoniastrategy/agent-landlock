#!/usr/bin/env bash
set -Eeuo pipefail

source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
trap cleanup_fixture EXIT

new_fixture
require_landlock

"$AGENT_LANDLOCK_BIN" grant "$GRANT_DIR" --timeout=1s >/tmp/agent-landlock-timeout-grant.out
assert_contains "$(cat /tmp/agent-landlock-timeout-grant.out)" "expires at"

before=$("$AGENT_LANDLOCK_BIN" grants)
assert_contains "$before" "$GRANT_DIR"

sleep 2
cleanup=$("$AGENT_LANDLOCK_BIN" grants --cleanup)
assert_contains "$cleanup" "cleanup: removed 1 expired grant(s)"

after=$("$AGENT_LANDLOCK_BIN" grants)
assert_contains "$after" "no persistent grants"
