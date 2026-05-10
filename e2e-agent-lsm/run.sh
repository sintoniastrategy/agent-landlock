#!/usr/bin/env bash
set -Eeuo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."

tmp=$(mktemp -d "${TMPDIR:-/tmp}/agent-lsm-e2e.XXXXXX")
cleanup() {
  rm -rf "$tmp"
}
trap cleanup EXIT

bin="${AGENT_LSM_BIN:-}"
if [[ -z "$bin" ]]; then
  bin="$tmp/agent-lsm"
  GOCACHE="${GOCACHE:-/tmp/agent-lsm-gocache}" go build -buildvcs=false -o "$bin" ./cmd/agent-lsm
fi

if [[ $# -gt 0 ]]; then
  tests=("$@")
else
  mapfile -t tests < <(find e2e-agent-lsm/tests -maxdepth 1 -type f -name '[0-9][0-9]-*.sh' | sort)
fi

for test_file in "${tests[@]}"; do
  if [[ ! -f "$test_file" && -f "e2e-agent-lsm/tests/$test_file" ]]; then
    test_file="e2e-agent-lsm/tests/$test_file"
  fi
  printf '[agent-lsm-e2e] RUN %s\n' "$(basename "$test_file")" >&2
  AGENT_LSM_BIN="$bin" bash "$test_file"
  printf '[agent-lsm-e2e] OK  %s\n' "$(basename "$test_file")" >&2
done

printf '[agent-lsm-e2e] all tests passed\n' >&2
