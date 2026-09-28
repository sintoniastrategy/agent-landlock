package agentlandlock

import (
	"fmt"
	"strconv"
	"strings"
)

func forceCodexYolo(cmd []string) ([]string, error) {
	out := append([]string{cmd[0]}, agentYoloArgs["codex"]...)
	for i := 1; i < len(cmd); i++ {
		token := cmd[i]
		if token == "--" {
			out = append(out, cmd[i:]...)
			break
		}
		switch token {
		case "--yolo", "--dangerously-bypass-approvals-and-sandbox", "--no-daemon", "--dangerously-bypass-hook-trust":
			continue
		case "--full-auto", "--approve-for-me":
			return nil, codexYoloConflict(token)
		}
		if _, _, ok := optionValue(cmd, i, "--remote"); ok {
			return nil, codexYoloConflict(token)
		}
		if value, next, ok := codexOptionValue(cmd, i, "--sandbox", "-s"); ok {
			if value != "danger-full-access" {
				return nil, codexYoloConflict(token)
			}
			i = next - 1
			continue
		}
		if value, next, ok := codexOptionValue(cmd, i, "--ask-for-approval", "-a"); ok {
			if value != "never" {
				return nil, codexYoloConflict(token)
			}
			i = next - 1
			continue
		}
		if value, next, ok := codexOptionValue(cmd, i, "--config", "-c"); ok {
			if next <= i || value == "" {
				return nil, exitError(ExitUsage, "codex: missing --config value")
			}
			key, setting, _ := strings.Cut(value, "=")
			key = codexConfigString(key)
			setting = codexConfigString(setting)
			expected := ""
			switch key {
			case "sandbox_mode":
				expected = "danger-full-access"
			case "approval_policy":
				expected = "never"
			case "approvals_reviewer":
				expected = "user"
			}
			if strings.HasPrefix(key, "approval_policy.") || expected != "" && setting != expected {
				return nil, codexYoloConflict("--config " + value)
			}
			if expected == "" {
				out = append(out, cmd[i:next]...)
			}
			i = next - 1
			continue
		}
		out = append(out, token)
		switch token {
		case "--model", "-m", "--profile", "-p", "--cd", "-C", "--image", "-i", "--enable", "--disable", "--local-provider", "--add-dir", "--output-schema", "--output-last-message", "-o", "--color", "--thread-source", "--listen", "--code-mode-host", "--remote-auth-token-env", "--ws-auth", "--ws-token-file", "--ws-token-sha256", "--ws-shared-secret-file", "--ws-issuer", "--ws-audience", "--ws-max-clock-skew-seconds":
			if i+1 < len(cmd) {
				i++
				out = append(out, cmd[i])
			}
		}
	}
	return out, nil
}

func codexOptionValue(args []string, index int, long, short string) (string, int, bool) {
	if value, next, ok := optionValueAny(args, index, long, short); ok {
		return value, next, true
	}
	if value, ok := strings.CutPrefix(args[index], short); ok && value != "" {
		return value, index + 1, true
	}
	return "", index, false
}

func codexConfigString(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 && value[0] == '\'' && value[len(value)-1] == '\'' {
		return value[1 : len(value)-1]
	}
	if unquoted, err := strconv.Unquote(value); err == nil {
		return unquoted
	}
	return value
}

func codexYoloConflict(argument string) error {
	return exitError(ExitUsage, fmt.Sprintf("codex must run locally with full YOLO; refusing %s (use --no-yolo to opt out)", argument))
}
