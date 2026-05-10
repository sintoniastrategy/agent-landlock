#!/usr/bin/env bash

fail() {
  printf '[agent-lsm-e2e][fail] %s\n' "$*" >&2
  exit 1
}

assert_file() {
  [[ -f "$1" ]] || fail "missing file: $1"
}

assert_not_exists() {
  [[ ! -e "$1" ]] || fail "unexpected path exists: $1"
}

assert_contains() {
  local haystack=$1
  local needle=$2
  [[ "$haystack" == *"$needle"* ]] || fail "expected output to contain: $needle"
}

new_fixture() {
  FIXTURE=$(mktemp -d "${TMPDIR:-/tmp}/agent-lsm-fixture.XXXXXX")
  export HOME="$FIXTURE/home"
  export XDG_STATE_HOME="$FIXTURE/state"
  export XDG_CONFIG_HOME="$FIXTURE/config"
  PROJECT="$HOME/project"
  GRANT_DIR="$HOME/cache"
  OUTSIDE="$HOME/outside"
  mkdir -p "$PROJECT" "$GRANT_DIR" "$OUTSIDE" "$XDG_STATE_HOME" "$XDG_CONFIG_HOME"
}

cleanup_fixture() {
  rm -rf "${FIXTURE:-}"
}

require_landlock() {
  if ! "$AGENT_LSM_BIN" doctor "$PROJECT" >/tmp/agent-lsm-doctor.out 2>&1; then
    cat /tmp/agent-lsm-doctor.out >&2 || true
    fail "Landlock ABI v3+ is required for agent-lsm e2e tests"
  fi
}
