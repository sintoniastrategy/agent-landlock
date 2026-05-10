#!/usr/bin/env bash
set -Eeuo pipefail

source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
trap cleanup_fixture EXIT

new_fixture
require_landlock

pids=()
for n in 1 2 3; do
  "$AGENT_LSM_BIN" --no-agent-state -d "$PROJECT" run -- bash -lc '
    set -Eeuo pipefail
    n=$1
    outside=$2
    printf "%s" "$n" > "parallel-${n}.txt"
    if { printf outside > "${outside}/parallel-${n}.txt"; } 2>"parallel-${n}.err"; then
      exit 42
    fi
    while [[ ! -e "stop-${n}" ]]; do
      sleep 0.1
    done
  ' _ "$n" "$OUTSIDE" &
  pids+=("$!")
done

for n in 1 2 3; do
  wait_until 20 test -f "$PROJECT/parallel-${n}.txt" \
    || fail "parallel run ${n} did not write project file"
  assert_not_exists "$OUTSIDE/parallel-${n}.txt"
done

for n in 1 2 3; do
  touch "$PROJECT/stop-${n}"
done
for pid in "${pids[@]}"; do
  wait "$pid"
done
