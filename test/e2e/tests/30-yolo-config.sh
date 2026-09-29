#!/usr/bin/env bash
set -Eeuo pipefail

source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
trap cleanup_fixture EXIT

new_fixture
require_landlock

bin_dir="$FIXTURE/bin"
mkdir -p "$bin_dir"
cat >"$bin_dir/claude" <<'SH'
#!/usr/bin/env bash
set -Eeuo pipefail
python3 - "$PROJECT_OUT" "$@" <<'PY'
import json
import pathlib
import stat
import sys

root = pathlib.Path(sys.argv[1])
args = sys.argv[2:]
(root / "claude.args").write_text(json.dumps(args))
if "--settings" in args:
    settings = pathlib.Path(args[args.index("--settings") + 1])
    (root / "claude.settings-path").write_text(str(settings))
    (root / "claude.settings").write_text(settings.read_text())
    assert stat.S_IMODE(settings.stat().st_mode) == 0o600
PY
if [[ "${LANDLOCK_WAIT:-}" == "1" ]]; then
  exec sleep 60
fi
SH
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
chmod +x "$bin_dir/claude" "$bin_dir/codex" "$bin_dir/gemini"

printf '%s\n' '{"sandbox":{"enabled":true},"permissions":{"defaultMode":"plan"},"env":{"PRIVATE_VALUE":"kept-private"}}' > "$PROJECT/claude-custom.json"
PROJECT_OUT="$PROJECT" PATH="$bin_dir:$PATH" \
  "$AGENT_LANDLOCK_BIN" -d "$PROJECT" claude -- --settings claude-custom.json -p prompt
claude_args=$(cat "$PROJECT/claude.args")
assert_contains "$claude_args" "--dangerously-skip-permissions"
assert_contains "$claude_args" "bypassPermissions"
assert_not_contains "$claude_args" "kept-private"
assert_not_exists "$(cat "$PROJECT/claude.settings-path")"
python3 - "$PROJECT/claude.settings" "$PROJECT/claude-custom.json" <<'PY'
import json
import sys

with open(sys.argv[1]) as stream:
    settings = json.load(stream)
assert settings["sandbox"]["enabled"] is False
assert settings["sandbox"]["allowUnsandboxedCommands"] is True
assert settings["permissions"]["defaultMode"] == "bypassPermissions"
assert settings["skipDangerousModePermissionPrompt"] is True
assert settings["env"]["PRIVATE_VALUE"] == "kept-private"
with open(sys.argv[2]) as stream:
    original = json.load(stream)
assert original["sandbox"]["enabled"] is True
assert original["permissions"]["defaultMode"] == "plan"
PY

PROJECT_OUT="$PROJECT" PATH="$bin_dir:$PATH" \
  "$AGENT_LANDLOCK_BIN" --no-yolo -d "$PROJECT" claude -- -p prompt
claude_no_yolo=$(cat "$PROJECT/claude.args")
assert_not_contains "$claude_no_yolo" "--dangerously-skip-permissions"
assert_not_contains "$claude_no_yolo" "--permission-mode"
assert_not_contains "$claude_no_yolo" "--settings"

rm "$PROJECT/claude.settings-path"
LANDLOCK_WAIT=1 PROJECT_OUT="$PROJECT" PATH="$bin_dir:$PATH" \
  "$AGENT_LANDLOCK_BIN" -d "$PROJECT" claude -- -p prompt &
claude_pid=$!
wait_until 10 test -s "$PROJECT/claude.settings-path"
claude_settings_path=$(cat "$PROJECT/claude.settings-path")
kill -TERM "$claude_pid"
claude_status=0
wait "$claude_pid" || claude_status=$?
[[ "$claude_status" == "143" ]] || fail "Claude signal exit was $claude_status"
assert_not_exists "$claude_settings_path"

PROJECT_OUT="$PROJECT" PATH="$bin_dir:$PATH" \
  "$AGENT_LANDLOCK_BIN" -d "$PROJECT" codex -- exec prompt
codex_args=$(cat "$PROJECT/codex.args")
assert_contains "$codex_args" "--dangerously-bypass-approvals-and-sandbox"
assert_not_contains "$codex_args" "--dangerously-bypass-hook-trust"
assert_contains "$codex_args" "--no-daemon"
assert_contains "$codex_args" 'sandbox_mode="danger-full-access"'
assert_contains "$codex_args" 'approval_policy="never"'
assert_contains "$codex_args" 'approvals_reviewer="user"'
assert_contains "$codex_args" "exec"

PROJECT_OUT="$PROJECT" PATH="$bin_dir:$PATH" \
  "$AGENT_LANDLOCK_BIN" --yolo-max -d "$PROJECT" codex -- exec prompt
codex_yolo_max=$(cat "$PROJECT/codex.args")
assert_contains "$codex_yolo_max" "--dangerously-bypass-approvals-and-sandbox"
assert_contains "$codex_yolo_max" "--dangerously-bypass-hook-trust"
assert_contains "$codex_yolo_max" "--no-daemon"
assert_contains "$codex_yolo_max" 'sandbox_mode="danger-full-access"'
assert_contains "$codex_yolo_max" 'approval_policy="never"'
assert_contains "$codex_yolo_max" 'approvals_reviewer="user"'
assert_contains "$codex_yolo_max" "exec"

PROJECT_OUT="$PROJECT" PATH="$bin_dir:$PATH" \
  "$AGENT_LANDLOCK_BIN" -d "$PROJECT" run --yolo-max -- codex app-server --stdio
codex_yolo_max_run=$(cat "$PROJECT/codex.args")
assert_contains "$codex_yolo_max_run" "--dangerously-bypass-hook-trust"
assert_contains "$codex_yolo_max_run" "app-server"

rm "$PROJECT/codex.args"
conflict_status=0
PROJECT_OUT="$PROJECT" PATH="$bin_dir:$PATH" \
  "$AGENT_LANDLOCK_BIN" --yolo-max -d "$PROJECT" codex --no-yolo -- exec prompt \
  >"$PROJECT/yolo-conflict.out" 2>&1 || conflict_status=$?
[[ "$conflict_status" == "2" ]] || fail "YOLO flag conflict exit was $conflict_status"
assert_contains "$(cat "$PROJECT/yolo-conflict.out")" "--no-yolo and --yolo-max cannot be combined"
assert_not_exists "$PROJECT/codex.args"

PROJECT_OUT="$PROJECT" PATH="$bin_dir:$PATH" \
  "$AGENT_LANDLOCK_BIN" --no-yolo -d "$PROJECT" codex -- exec prompt
codex_no_yolo=$(cat "$PROJECT/codex.args")
assert_not_contains "$codex_no_yolo" "--dangerously-bypass-approvals-and-sandbox"
assert_not_contains "$codex_no_yolo" "--dangerously-bypass-hook-trust"
assert_not_contains "$codex_no_yolo" "--no-daemon"
assert_not_contains "$codex_no_yolo" "sandbox_mode"
assert_not_contains "$codex_no_yolo" "approval_policy"

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
