#!/usr/bin/env bash
set -Eeuo pipefail

source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
trap cleanup_fixture EXIT

new_fixture
require_landlock

out=$("$AGENT_LSM_BIN" --no-agent-state -d "$PROJECT" -g "$GRANT_DIR" run -- bash -lc '
  set -Eeuo pipefail
  printf project > "$1/project-write.txt"
  printf grant > "$2/grant-write.txt"
  if { printf outside > "$3/outside-write.txt"; } 2>"$1/outside.err"; then
    printf outside-write-unexpected
    exit 42
  fi
  cat "$1/outside.err"
' _ "$PROJECT" "$GRANT_DIR" "$OUTSIDE")

assert_file "$PROJECT/project-write.txt"
assert_file "$GRANT_DIR/grant-write.txt"
assert_not_exists "$OUTSIDE/outside-write.txt"
assert_contains "$out" "Permission denied"

owner=$(stat -c '%U:%G' "$PROJECT/project-write.txt")
expected="$(id -un):$(id -gn)"
[[ "$owner" == "$expected" ]] || fail "unexpected owner: $owner, want $expected"
