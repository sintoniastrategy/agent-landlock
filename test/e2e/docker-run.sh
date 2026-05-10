#!/usr/bin/env bash
set -Eeuo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/../.."

IMAGE="${E2E_AGENT_LANDLOCK_IMAGE:-agent-landlock-e2e:local}"
DOCKERFILE="${E2E_AGENT_LANDLOCK_DOCKERFILE:-test/e2e/Dockerfile}"
AGENT_LANDLOCK_BIN="${AGENT_LANDLOCK_BIN:-}"

find_engine() {
  if [[ -n "${E2E_ENGINE:-}" ]]; then
    printf '%s\n' "$E2E_ENGINE"
    return
  fi
  if command -v docker >/dev/null 2>&1; then
    printf 'docker\n'
    return
  fi
  if command -v podman >/dev/null 2>&1; then
    printf 'podman\n'
    return
  fi
  printf 'docker or podman is required\n' >&2
  exit 127
}

real_path() {
  if command -v realpath >/dev/null 2>&1; then
    realpath "$1"
  else
    python3 -c 'import os, sys; print(os.path.abspath(sys.argv[1]))' "$1"
  fi
}

stage_binary() {
  local tmp=$1
  local source=$2
  if [[ -z "$source" ]]; then
    source="$tmp/agent-landlock"
    GOCACHE="${GOCACHE:-/tmp/agent-landlock-gocache}" go build -buildvcs=false -o "$source" ./cmd/agent-landlock
    printf '%s\n' "$source"
    return
  fi
  install -m 0755 "$(real_path "$source")" "$tmp/agent-landlock"
  printf '%s\n' "$tmp/agent-landlock"
}

usage() {
  cat <<'EOF'
usage:
  test/e2e/docker-run.sh build
  test/e2e/docker-run.sh test [TEST ...]
  test/e2e/docker-run.sh shell
  test/e2e/docker-run.sh clean

environment:
  AGENT_LANDLOCK_BIN=/path/to/agent-landlock        subject under test; default builds ./cmd/agent-landlock
  E2E_ENGINE=docker|podman                container engine override
  E2E_AGENT_LANDLOCK_IMAGE=agent-landlock-e2e:local image tag
  E2E_SKIP_BUILD=1                        skip image build before test/shell
EOF
}

cmd="${1:-test}"
if [[ $# -gt 0 ]]; then
  shift
fi

case "$cmd" in
  -h|--help|help)
    usage
    exit 0
    ;;
esac

engine=$(find_engine)
tmp=$(mktemp -d "${TMPDIR:-/tmp}/agent-landlock-docker.XXXXXX")
cleanup() {
  rm -rf "$tmp"
}
trap cleanup EXIT

case "$cmd" in
  build)
    "$engine" build -t "$IMAGE" -f "$DOCKERFILE" test/e2e
    ;;
  test)
    if [[ "${E2E_SKIP_BUILD:-0}" != "1" ]]; then
      "$engine" build -t "$IMAGE" -f "$DOCKERFILE" test/e2e
    fi
    subject=$(stage_binary "$tmp" "$AGENT_LANDLOCK_BIN")
    run_flags=(--rm --security-opt no-new-privileges:false --security-opt seccomp=unconfined -v "${subject}:/src/agent-landlock:ro")
    if [[ -t 1 ]]; then
      run_flags+=(-t)
    fi
    "$engine" run "${run_flags[@]}" -e AGENT_LANDLOCK_SOURCE=/src/agent-landlock "$IMAGE" "$@"
    ;;
  shell)
    if [[ "${E2E_SKIP_BUILD:-0}" != "1" ]]; then
      "$engine" build -t "$IMAGE" -f "$DOCKERFILE" test/e2e
    fi
    subject=$(stage_binary "$tmp" "$AGENT_LANDLOCK_BIN")
    run_flags=(--rm --security-opt no-new-privileges:false --security-opt seccomp=unconfined -v "${subject}:/src/agent-landlock:ro" --entrypoint /bin/bash)
    if [[ -t 1 ]]; then
      run_flags+=(-it)
    fi
    "$engine" run "${run_flags[@]}" -e AGENT_LANDLOCK_SOURCE=/src/agent-landlock "$IMAGE"
    ;;
  clean)
    "$engine" rmi "$IMAGE"
    ;;
  *)
    usage >&2
    exit 2
    ;;
esac
