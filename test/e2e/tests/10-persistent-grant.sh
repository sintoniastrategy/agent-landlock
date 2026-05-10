#!/usr/bin/env bash
set -Eeuo pipefail

source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
trap cleanup_fixture EXIT

new_fixture
require_landlock

"$AGENT_LANDLOCK_BIN" grant "$GRANT_DIR" >/tmp/agent-landlock-grant.out
assert_contains "$(cat /tmp/agent-landlock-grant.out)" "granted writable path"

grants=$("$AGENT_LANDLOCK_BIN" grants)
assert_contains "$grants" "PERSISTENT GRANTS"
assert_contains "$grants" "$GRANT_DIR"

"$AGENT_LANDLOCK_BIN" --no-agent-state -d "$PROJECT" run -- bash -lc '
  set -Eeuo pipefail
  printf persistent > "$1/persistent-write.txt"
' _ "$GRANT_DIR"

assert_file "$GRANT_DIR/persistent-write.txt"

"$AGENT_LANDLOCK_BIN" revoke "$GRANT_DIR" >/tmp/agent-landlock-revoke.out
assert_contains "$(cat /tmp/agent-landlock-revoke.out)" "revoked writable path"

if "$AGENT_LANDLOCK_BIN" --no-agent-state -d "$PROJECT" run -- bash -lc 'printf nope > "$1/after-revoke.txt"' _ "$GRANT_DIR" 2>/tmp/agent-landlock-revoke-deny.err; then
  fail "revoked grant remained writable"
fi
assert_not_exists "$GRANT_DIR/after-revoke.txt"
