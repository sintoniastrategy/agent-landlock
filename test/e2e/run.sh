#!/usr/bin/env bash
set -Eeuo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/../.."

usage() {
  cat <<'EOF'
usage:
  test/e2e/run.sh [OPTIONS] [TEST ...]

By default, runs every numbered test in test/e2e/tests, including optional
real-agent checks. The script sets a local GOCACHE automatically when building.

options:
  --skip TEST                  skip a test by basename or path; repeatable
  --skip-real-agents           skip live Claude/Codex/Gemini prompt tests
  --no-real-agents             alias for --skip-real-agents
  --real-agent-tools LIST      comma-separated real agents; default: claude,codex,gemini
  --real-agent-timeout SECS    per-agent live prompt timeout; default: 60
  --keep-real-agent-fixture    keep temp files from real-agent tests
  -h, --help                   show this help
EOF
}

tests=()
skip_tests=()
while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;
    --skip)
      [[ $# -ge 2 ]] || { printf 'missing value for --skip\n' >&2; exit 2; }
      skip_tests+=("$2")
      shift 2
      ;;
    --skip=*)
      skip_tests+=("${1#--skip=}")
      shift
      ;;
    --skip-real-agents|--no-real-agents)
      export E2E_SKIP_REAL_AGENTS=1
      shift
      ;;
    --real-agent-tools)
      [[ $# -ge 2 ]] || { printf 'missing value for --real-agent-tools\n' >&2; exit 2; }
      export E2E_REAL_AGENT_TOOLS="$2"
      shift 2
      ;;
    --real-agent-tools=*)
      export E2E_REAL_AGENT_TOOLS="${1#--real-agent-tools=}"
      shift
      ;;
    --real-agent-timeout)
      [[ $# -ge 2 ]] || { printf 'missing value for --real-agent-timeout\n' >&2; exit 2; }
      export E2E_REAL_AGENT_TIMEOUT="$2"
      shift 2
      ;;
    --real-agent-timeout=*)
      export E2E_REAL_AGENT_TIMEOUT="${1#--real-agent-timeout=}"
      shift
      ;;
    --keep-real-agent-fixture)
      export E2E_KEEP_REAL_AGENT_FIXTURE=1
      shift
      ;;
    --)
      shift
      tests+=("$@")
      break
      ;;
    -*)
      printf 'unknown option: %s\n' "$1" >&2
      usage >&2
      exit 2
      ;;
    *)
      tests+=("$1")
      shift
      ;;
  esac
done

tmp=$(mktemp -d "${TMPDIR:-/tmp}/agent-landlock-e2e.XXXXXX")
cleanup() {
  rm -rf "$tmp"
}
trap cleanup EXIT

bin="${AGENT_LANDLOCK_BIN:-}"
if [[ -z "$bin" ]]; then
  bin="$tmp/agent-landlock"
  GOCACHE="${GOCACHE:-/tmp/agent-landlock-gocache}" go build -buildvcs=false -o "$bin" ./cmd/agent-landlock
fi

if [[ ${#tests[@]} -eq 0 ]]; then
  mapfile -t tests < <(find test/e2e/tests -maxdepth 1 -type f -name '[0-9][0-9]-*.sh' | sort)
fi

should_skip_test() {
  local test_file=$1
  local skip
  for skip in "${skip_tests[@]}"; do
    if [[ "$test_file" == "$skip" || "$(basename "$test_file")" == "$skip" ]]; then
      return 0
    fi
    if [[ "$skip" != */* && "$(basename "$test_file")" == "$skip".sh ]]; then
      return 0
    fi
  done
  return 1
}

for test_file in "${tests[@]}"; do
  if [[ ! -f "$test_file" && -f "test/e2e/tests/$test_file" ]]; then
    test_file="test/e2e/tests/$test_file"
  fi
  if should_skip_test "$test_file"; then
    printf '[agent-landlock-e2e] SKIP %s (--skip)\n' "$(basename "$test_file")" >&2
    continue
  fi
  printf '[agent-landlock-e2e] RUN %s\n' "$(basename "$test_file")" >&2
  AGENT_LANDLOCK_BIN="$bin" bash "$test_file"
  printf '[agent-landlock-e2e] OK  %s\n' "$(basename "$test_file")" >&2
done

printf '[agent-landlock-e2e] all tests passed\n' >&2
