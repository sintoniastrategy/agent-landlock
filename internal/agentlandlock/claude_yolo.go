package agentlandlock

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

func forceClaudeYolo(cmd []string) ([]string, error) {
	settings := "{}"
	var args []string
	positional := false
	for i := 1; i < len(cmd); i++ {
		token := cmd[i]
		if token == "--" {
			args = append(args, cmd[i:]...)
			break
		}
		name, _, _ := strings.Cut(token, "=")
		switch name {
		case "--dangerously-skip-permissions", "--allow-dangerously-skip-permissions":
			if token != name {
				return nil, claudeYoloConflict(name)
			}
			continue
		case "--restricted", "--cloud", "--teleport", "--environment", "--bg", "--background":
			return nil, claudeYoloConflict(name)
		case "--permission-mode", "--settings":
			value, next, ok := optionValue(cmd, i, name)
			if !ok || next <= i || value == "" || strings.HasPrefix(value, "--") {
				return nil, exitError(ExitUsage, "claude: missing "+name+" value")
			}
			if name == "--permission-mode" {
				if value != "bypassPermissions" {
					return nil, claudeYoloConflict(name + " " + value)
				}
			} else {
				settings = value
			}
			i = next - 1
			continue
		}
		if !positional && !strings.HasPrefix(token, "-") {
			positional = true
			switch token {
			case "agents", "attach", "respawn", "ultrareview":
				return nil, claudeYoloConflict(token)
			}
		}
		args = append(args, token)
		switch token {
		case "--model", "--fallback-model", "--agent", "--agents", "--append-system-prompt", "--append-system-prompt-file", "--system-prompt", "--system-prompt-file", "--output-format", "--input-format", "--json-schema", "--max-budget-usd", "--max-turns", "--effort", "--debug-file", "--session-id", "--setting-sources", "--plugin-dir", "--plugin-url", "--permission-prompts", "--permission-prompt-tool", "--name", "-n", "--autocompact", "--client-data-url", "--managed-settings", "--project-config-root", "--remote-control-session-name-prefix", "--system-prompt-snapshot", "--allowedTools", "--allowed-tools", "--disallowedTools", "--disallowed-tools", "--tools", "--add-dir", "--mcp-config", "--betas", "--file":
			if i+1 < len(cmd) {
				i++
				args = append(args, cmd[i])
			}
		}
	}
	out := append([]string{cmd[0]}, agentYoloArgs["claude"]...)
	out = append(out, "--settings", settings)
	return append(out, args...), nil
}

func prepareClaudeYoloSettings(cmd []string, workdir string, dryRun bool) ([]string, func(), error) {
	index := len(agentYoloArgs["claude"]) + 2
	value := cmd[index]
	data := []byte(value)
	if !strings.HasPrefix(strings.TrimSpace(value), "{") {
		path := value
		if !filepath.IsAbs(path) {
			path = filepath.Join(workdir, path)
		}
		var err error
		data, err = os.ReadFile(path)
		if err != nil {
			return nil, nil, exitError(ExitUsage, "claude: cannot read --settings file: "+err.Error())
		}
	}
	settings := map[string]json.RawMessage{}
	if err := json.Unmarshal(data, &settings); err != nil || settings == nil {
		return nil, nil, exitError(ExitUsage, "claude: --settings must contain a JSON object")
	}
	for key, overrides := range map[string]map[string]json.RawMessage{
		"sandbox":     {"enabled": json.RawMessage("false"), "allowUnsandboxedCommands": json.RawMessage("true")},
		"permissions": {"defaultMode": json.RawMessage(`"bypassPermissions"`)},
	} {
		section := map[string]json.RawMessage{}
		if raw, ok := settings[key]; ok {
			if err := json.Unmarshal(raw, &section); err != nil || section == nil {
				return nil, nil, exitError(ExitUsage, "claude: --settings "+key+" must be a JSON object")
			}
		}
		for name, raw := range overrides {
			section[name] = raw
		}
		encoded, err := json.Marshal(section)
		if err != nil {
			return nil, nil, err
		}
		settings[key] = encoded
	}
	settings["skipDangerousModePermissionPrompt"] = json.RawMessage("true")
	data, err := json.Marshal(settings)
	if err != nil {
		return nil, nil, err
	}
	out := slices.Clone(cmd)
	if dryRun {
		out[index] = "<temporary Claude settings: sandbox=off, permissions=bypassPermissions>"
		return out, func() {}, nil
	}
	file, err := os.CreateTemp("/tmp", "agent-landlock-claude-settings-*.json")
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() { _ = os.Remove(file.Name()) }
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		cleanup()
		return nil, nil, err
	}
	if err := file.Close(); err != nil {
		cleanup()
		return nil, nil, err
	}
	out[index] = file.Name()
	return out, cleanup, nil
}

func claudeYoloConflict(argument string) error {
	return exitError(ExitUsage, fmt.Sprintf("claude must run locally with full YOLO; refusing %s (use --no-yolo to opt out)", argument))
}
