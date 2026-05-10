#!/usr/bin/env bash
set -Eeuo pipefail

source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

if [[ "${E2E_SKIP_REAL_AGENTS:-0}" == "1" || "${E2E_REAL_AGENTS:-1}" == "0" ]]; then
  log "SKIP 90-real-agents.sh (real-agent tests disabled)"
  exit 0
fi

REAL_TMP=$(mktemp -d "${TMPDIR:-/tmp}/agent-landlock-real.XXXXXX")
cleanup_real_fixture() {
  if [[ "${E2E_KEEP_REAL_AGENT_FIXTURE:-0}" == "1" ]]; then
    log "keeping real-agent fixture: $REAL_TMP"
    return
  fi
  rm -rf "$REAL_TMP"
}
trap cleanup_real_fixture EXIT

# Do not call new_fixture here: this suite intentionally preserves the host
# HOME/XDG auth state so real Claude/Codex/Gemini credentials remain available.
PROJECT="$REAL_TMP/project"
OUTSIDE="$REAL_TMP/outside"
mkdir -p "$PROJECT" "$OUTSIDE"
require_landlock

REAL_AGENT_TIMEOUT="${E2E_REAL_AGENT_TIMEOUT:-60}"
REAL_AGENT_TOOLS="${E2E_REAL_AGENT_TOOLS:-claude,codex,gemini}"
REAL_AGENT_RAN=0

shell_quote() {
  printf '%q' "$1"
}

join_boundary_command() {
  local inside=$1
  local outside=$2
  local result=$3
  local err=$4
  local token=$5
  local script
  script='set -Eeuo pipefail
inside=$1
outside=$2
result=$3
err=$4
token=$5
printf "%s\n" "$token" > "$inside"
if { printf "%s\n" "$token" > "$outside"; } 2>"$err"; then
  printf "OUTSIDE_WRITE_SUCCEEDED\n" > "$result"
else
  printf "OUTSIDE_WRITE_DENIED\n" > "$result"
fi'
  printf 'bash -lc %s _ %s %s %s %s %s' \
    "$(shell_quote "$script")" \
    "$(shell_quote "$inside")" \
    "$(shell_quote "$outside")" \
    "$(shell_quote "$result")" \
    "$(shell_quote "$err")" \
    "$(shell_quote "$token")"
}

prompt_for() {
  local tool=$1
  local command=$2
  cat <<EOF
This is an automated agent-landlock real-agent e2e test for ${tool}.

Run this exact shell command once, then stop. Do not ask for confirmation. Do not
modify any other files. It intentionally attempts one allowed write and one
write outside the allowed workspace so the wrapper can verify the boundary.

\`\`\`sh
${command}
\`\`\`
EOF
}

agent_unavailable_output() {
  local output=$1
  grep -Eiq \
    'not logged in|please run /login|not authenticated|authentication required|login required|no api key|api key|no token|oauth|ECONNREFUSED|ENOTFOUND|FailedToOpenSocket|Connection error|Unable to connect|network' \
    <<<"$output" || grep -Eiq \
    'Read-only file system|failed to initialize in-process app-server client' \
    <<<"$output"
}

run_real_agent() {
  local tool=$1
  local inside="$PROJECT/${tool}-inside.txt"
  local outside="$OUTSIDE/${tool}-outside.txt"
  local result="$PROJECT/${tool}-result.txt"
  local err="$PROJECT/${tool}-outside.err"
  local stdout="$PROJECT/${tool}.stdout"
  local stderr="$PROJECT/${tool}.stderr"
  local token="agent-landlock-${tool}-$(date +%s)-$$"
  local command prompt rc output

  if ! command -v "$tool" >/dev/null 2>&1; then
    fail "real ${tool} command not found; use --skip-real-agents or --real-agent-tools to disable it"
  fi

  command=$(join_boundary_command "$inside" "$outside" "$result" "$err" "$token")
  prompt=$(prompt_for "$tool" "$command")
  rm -f "$inside" "$outside" "$result" "$err" "$stdout" "$stderr"

  log "RUN real ${tool} boundary prompt"
  set +e
  case "$tool" in
    claude)
      timeout "$REAL_AGENT_TIMEOUT" "$AGENT_LANDLOCK_BIN" -d "$PROJECT" \
        claude --strict-mcp-config --mcp-config '{"mcpServers":{}}' --disable-slash-commands \
        -p "$prompt" >"$stdout" 2>"$stderr"
      ;;
    codex)
      timeout "$REAL_AGENT_TIMEOUT" "$AGENT_LANDLOCK_BIN" -d "$PROJECT" \
        codex exec --skip-git-repo-check --color never "$prompt" >"$stdout" 2>"$stderr"
      ;;
    gemini)
      timeout "$REAL_AGENT_TIMEOUT" "$AGENT_LANDLOCK_BIN" -d "$PROJECT" \
        gemini -p "$prompt" --output-format text >"$stdout" 2>"$stderr"
      ;;
    *)
      fail "unknown real agent tool: $tool"
      ;;
  esac
  rc=$?
  set -e
  output="$(cat "$stdout" "$stderr" 2>/dev/null || true)"

  if [[ ! -f "$result" ]]; then
    if [[ "$rc" -eq 124 ]]; then
      fail "real ${tool} timed out after ${REAL_AGENT_TIMEOUT}s; stdout=$stdout stderr=$stderr"
    fi
    if agent_unavailable_output "$output"; then
      fail "real ${tool} auth/network/startup unavailable; use --skip-real-agents or --real-agent-tools to disable it; stdout=$stdout stderr=$stderr"
    fi
    fail "real ${tool} did not produce boundary result; rc=$rc stdout=$stdout stderr=$stderr"
  fi

  assert_file "$inside"
  assert_contains "$(cat "$inside")" "$token"
  assert_contains "$(cat "$result")" "OUTSIDE_WRITE_DENIED"
  assert_not_exists "$outside"
  assert_contains "$(cat "$err")" "Permission denied"
  REAL_AGENT_RAN=$((REAL_AGENT_RAN + 1))
  log "OK real ${tool} boundary prompt"
}

IFS=',' read -r -a tools <<<"$REAL_AGENT_TOOLS"
for tool in "${tools[@]}"; do
  tool="${tool//[[:space:]]/}"
  [[ -n "$tool" ]] || continue
  run_real_agent "$tool"
done

if [[ "$REAL_AGENT_RAN" -eq 0 ]]; then
  fail "no real agents ran"
fi
