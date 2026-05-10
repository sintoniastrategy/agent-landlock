#!/usr/bin/env bash
set -Eeuo pipefail

source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
trap cleanup_fixture EXIT

new_fixture
require_landlock

bin_dir="$FIXTURE/bin"
mkdir -p "$bin_dir"
cat >"$bin_dir/codex" <<'SH'
#!/usr/bin/env bash
set -Eeuo pipefail
printf '%s\n' "$@" > "$PROJECT_OUT/codex.args"
SH
cat >"$bin_dir/gemini" <<'SH'
#!/usr/bin/env bash
set -Eeuo pipefail
printf '%s\n' "$@" > "$PROJECT_OUT/gemini.args"
printf '%s\n' "${GEMINI_SANDBOX:-}" > "$PROJECT_OUT/gemini.sandbox"
printf '%s\n' "${FROM_CONFIG:-}" > "$PROJECT_OUT/gemini.from-config"
SH
chmod +x "$bin_dir/codex" "$bin_dir/gemini"

PROJECT_OUT="$PROJECT" PATH="$bin_dir:$PATH" \
  "$AGENT_LANDLOCK_BIN" -d "$PROJECT" codex -- exec prompt
codex_args=$(cat "$PROJECT/codex.args")
assert_contains "$codex_args" "--dangerously-bypass-approvals-and-sandbox"
assert_contains "$codex_args" "exec"

PROJECT_OUT="$PROJECT" PATH="$bin_dir:$PATH" \
  "$AGENT_LANDLOCK_BIN" --no-yolo -d "$PROJECT" codex -- exec prompt
codex_no_yolo=$(cat "$PROJECT/codex.args")
assert_not_contains "$codex_no_yolo" "--dangerously-bypass-approvals-and-sandbox"

mkdir -p "$XDG_CONFIG_HOME/agent-landlock"
printf 'EXTRA_ENV=FROM_CONFIG=yes\n' > "$XDG_CONFIG_HOME/agent-landlock/config"
PROJECT_OUT="$PROJECT" PATH="$bin_dir:$PATH" \
  "$AGENT_LANDLOCK_BIN" -d "$PROJECT" gemini -- --prompt hi
gemini_args=$(cat "$PROJECT/gemini.args")
assert_contains "$gemini_args" "--approval-mode"
assert_contains "$gemini_args" "yolo"
assert_contains "$gemini_args" "--skip-trust"
assert_contains "$(cat "$PROJECT/gemini.sandbox")" "false"
assert_contains "$(cat "$PROJECT/gemini.from-config")" "yes"
