#!/usr/bin/env bash
set -Eeuo pipefail

if [[ -z "${AGENT_LANDLOCK_SOURCE:-}" ]]; then
  printf '[agent-landlock-e2e][fail] AGENT_LANDLOCK_SOURCE is not set\n' >&2
  exit 2
fi
if [[ ! -x "$AGENT_LANDLOCK_SOURCE" ]]; then
  printf '[agent-landlock-e2e][fail] AGENT_LANDLOCK_SOURCE is not executable: %s\n' "$AGENT_LANDLOCK_SOURCE" >&2
  exit 2
fi

export AGENT_LANDLOCK_BIN="$AGENT_LANDLOCK_SOURCE"

if [[ $# -gt 0 ]]; then
  tests=()
  for requested in "$@"; do
    if [[ -f "$requested" ]]; then
      tests+=("$requested")
    elif [[ -f "/test/e2e/tests/$requested" ]]; then
      tests+=("/test/e2e/tests/$requested")
    elif [[ -f "/test/e2e/tests/${requested}.sh" ]]; then
      tests+=("/test/e2e/tests/${requested}.sh")
    else
      printf '[agent-landlock-e2e][fail] test not found: %s\n' "$requested" >&2
      exit 2
    fi
  done
else
  mapfile -t tests < <(find /test/e2e/tests -maxdepth 1 -type f -name '[0-9][0-9]-*.sh' | sort)
fi

for test_file in "${tests[@]}"; do
  printf '[agent-landlock-e2e] RUN %s\n' "$(basename "$test_file")" >&2
  bash "$test_file"
  printf '[agent-landlock-e2e] OK  %s\n' "$(basename "$test_file")" >&2
done

printf '[agent-landlock-e2e] all tests passed\n' >&2
