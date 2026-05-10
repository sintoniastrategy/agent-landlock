#!/usr/bin/env bash
set -Eeuo pipefail

source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
trap cleanup_fixture EXIT

new_fixture
require_landlock

status=$("$AGENT_LSM_BIN" status "$PROJECT")
assert_contains "$status" "tool            : agent-lsm"
assert_contains "$status" "state dir       : $XDG_STATE_HOME/agent-lsm"
assert_contains "$status" "landlock ABI"

doctor=$("$AGENT_LSM_BIN" doctor --heal "$PROJECT")
assert_contains "$doctor" "doctor mode     : heal"
assert_contains "$doctor" "result          : ok"
[[ -d "$XDG_STATE_HOME/agent-lsm" ]] || fail "doctor --heal did not create state dir"

set +e
safety=$("$AGENT_LSM_BIN" grant / 2>&1)
rc=$?
set -e
[[ "$rc" -ne 0 ]] || fail "grant / unexpectedly succeeded"
assert_contains "$safety" "refusing writable access to safety path"

set +e
home_safety=$("$AGENT_LSM_BIN" grant "$HOME" 2>&1)
rc=$?
set -e
[[ "$rc" -ne 0 ]] || fail "grant HOME unexpectedly succeeded"
assert_contains "$home_safety" "refusing to make all of \$HOME writable"
