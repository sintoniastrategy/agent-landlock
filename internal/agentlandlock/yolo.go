package agentlandlock

import (
	"fmt"
	"path/filepath"
)

func forceAgentYolo(cmd []string, agent string, env map[string]string, noYolo bool) ([]string, error) {
	if noYolo || agent == "" || !KnownAgents[agent] {
		return cmd, nil
	}
	if agent == "gemini" {
		env["GEMINI_SANDBOX"] = "false"
	}
	hasYolo, err := scanYoloArgs(agent, cmd[1:])
	if err != nil {
		return nil, err
	}
	injected := []string{}
	if !hasYolo {
		injected = append(injected, agentYoloArgs[agent]...)
	}
	if agent == "gemini" && !hasBoolFlag(cmd[1:], "--skip-trust") {
		injected = append(injected, "--skip-trust")
	}
	if len(injected) == 0 {
		return cmd, nil
	}
	out := []string{cmd[0]}
	out = append(out, injected...)
	out = append(out, cmd[1:]...)
	return out, nil
}

func inferAgent(cmd []string) string {
	if len(cmd) == 0 {
		return ""
	}
	name := filepath.Base(cmd[0])
	if KnownAgents[name] {
		return name
	}
	return ""
}

func scanYoloArgs(agent string, args []string) (bool, error) {
	yolo := false
	for i := 0; i < len(args); i++ {
		token := args[i]
		if token == "--" {
			break
		}
		switch agent {
		case "claude":
			if token == "--dangerously-skip-permissions" {
				yolo = true
				continue
			}
			if value, next, ok := optionValue(args, i, "--permission-mode"); ok {
				if value != "bypassPermissions" {
					return false, exitError(ExitUsage, fmt.Sprintf("claude must run YOLO; refusing --permission-mode %s", value))
				}
				yolo = true
				i = next - 1
			}
		case "codex":
			if token == "--dangerously-bypass-approvals-and-sandbox" {
				yolo = true
				continue
			}
			if value, next, ok := optionValueAny(args, i, "--ask-for-approval", "-a"); ok {
				if value != "never" {
					return false, exitError(ExitUsage, fmt.Sprintf("codex must run YOLO; refusing approval mode %s", value))
				}
				i = next - 1
				continue
			}
			if value, next, ok := optionValueAny(args, i, "--sandbox", "-s"); ok {
				if value != "danger-full-access" {
					return false, exitError(ExitUsage, fmt.Sprintf("codex must run YOLO; refusing sandbox %s", value))
				}
				i = next - 1
			}
		case "gemini":
			if token == "--yolo" {
				yolo = true
				continue
			}
			if value, next, ok := optionValue(args, i, "--approval-mode"); ok {
				if value != "yolo" {
					return false, exitError(ExitUsage, fmt.Sprintf("gemini must run YOLO; refusing --approval-mode %s", value))
				}
				yolo = true
				i = next - 1
			}
		}
	}
	return yolo, nil
}

func optionValueAny(args []string, index int, flags ...string) (string, int, bool) {
	for _, flag := range flags {
		if value, next, ok := optionValue(args, index, flag); ok {
			return value, next, true
		}
	}
	return "", index, false
}

func optionValue(args []string, index int, flag string) (string, int, bool) {
	token := args[index]
	if token == flag {
		if index+1 >= len(args) {
			return "", index, true
		}
		return args[index+1], index + 2, true
	}
	prefix := flag + "="
	if len(token) > len(prefix) && token[:len(prefix)] == prefix {
		return token[len(prefix):], index + 1, true
	}
	return "", index, false
}

func hasBoolFlag(args []string, flag string) bool {
	for _, token := range args {
		if token == "--" {
			return false
		}
		if token == flag {
			return true
		}
	}
	return false
}
