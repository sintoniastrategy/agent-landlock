#!/usr/bin/env bash
set -Eeuo pipefail

if [[ -z "${AGENT_LSM_SOURCE:-}" ]]; then
  printf '[agent-lsm-e2e][fail] AGENT_LSM_SOURCE is not set\n' >&2
  exit 2
fi
if [[ ! -x "$AGENT_LSM_SOURCE" ]]; then
  printf '[agent-lsm-e2e][fail] AGENT_LSM_SOURCE is not executable: %s\n' "$AGENT_LSM_SOURCE" >&2
  exit 2
fi

export AGENT_LSM_BIN="$AGENT_LSM_SOURCE"

if [[ $# -gt 0 ]]; then
  tests=()
  for requested in "$@"; do
    if [[ -f "$requested" ]]; then
      tests+=("$requested")
    elif [[ -f "/e2e-agent-lsm/tests/$requested" ]]; then
      tests+=("/e2e-agent-lsm/tests/$requested")
    elif [[ -f "/e2e-agent-lsm/tests/${requested}.sh" ]]; then
      tests+=("/e2e-agent-lsm/tests/${requested}.sh")
    else
      printf '[agent-lsm-e2e][fail] test not found: %s\n' "$requested" >&2
      exit 2
    fi
  done
else
  mapfile -t tests < <(find /e2e-agent-lsm/tests -maxdepth 1 -type f -name '[0-9][0-9]-*.sh' | sort)
fi

for test_file in "${tests[@]}"; do
  printf '[agent-lsm-e2e] RUN %s\n' "$(basename "$test_file")" >&2
  bash "$test_file"
  printf '[agent-lsm-e2e] OK  %s\n' "$(basename "$test_file")" >&2
done

printf '[agent-lsm-e2e] all tests passed\n' >&2
