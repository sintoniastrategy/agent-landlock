package agentlandlock

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const (
	runtimeInstructionsBegin = "<!-- agent-landlock:runtime-instructions begin -->"
	runtimeInstructionsEnd   = "<!-- agent-landlock:runtime-instructions end -->"
)

type agentInstructions struct {
	name    string // Display name: "Claude", "Codex", "Gemini"
	dir     string // Home-relative dir: ".claude", ".codex", ".gemini"
	file    string // Filename: "CLAUDE.md", "AGENTS.md", "GEMINI.md"
}

var supportedAgents = []agentInstructions{
	{name: "Claude", dir: ".claude", file: "CLAUDE.md"},
	{name: "Codex", dir: ".codex", file: "AGENTS.md"},
	{name: "Gemini", dir: ".gemini", file: "GEMINI.md"},
}

func (a agentInstructions) runtimeBlock() string {
	return runtimeInstructionsBegin + `
## agent-landlock Runtime

- This CLI can run ` + a.name + ` through ` + "`agent-landlock`" + ` with a Linux Landlock write boundary.
- The current working directory is the intended writable workspace for a sandboxed run.
- Use ` + "`agent-landlock -g PATH`" + ` or ` + "`agent-landlock grant PATH`" + ` for any additional writable project path.
- If a write fails with a permission error outside the current working directory, keep generated or edited files inside the current working directory and report the blocked outside path.
- Treat ` + "`~/.claude`" + `, ` + "`~/.codex`" + `, and ` + "`~/.gemini`" + ` as tool state, not as a general workspace.
` + runtimeInstructionsEnd + `
`
}

func (a agentInstructions) path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, a.dir, a.file), nil
}

func checkAgentInstructions(a agentInstructions) (string, string, error) {
	path, err := a.path()
	if err != nil {
		return "", "", err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return path, "missing", nil
	}
	if err != nil {
		return path, "", err
	}
	return path, managedInstructionStatus(string(data), a.runtimeBlock()), nil
}

func healAgentInstructions(a agentInstructions, dryRun bool) (string, string, error) {
	path, err := a.path()
	if err != nil {
		return "", "", err
	}
	dir := filepath.Dir(path)
	if err := refuseSymlink(dir, "refusing symlinked "+a.name+" config directory"); err != nil {
		return path, "", err
	}
	if !dryRun {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return path, "", err
		}
	}
	if err := refuseSymlink(path, "refusing symlinked "+a.name+" global instructions"); err != nil {
		return path, "", err
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		data = nil
	} else if err != nil {
		return path, "", err
	}
	block := a.runtimeBlock()
	status := managedInstructionStatus(string(data), block)
	if status == "ok" {
		return path, "ok", nil
	}
	if dryRun {
		return path, "would heal", nil
	}
	mode := os.FileMode(0o600)
	if st, err := os.Stat(path); err == nil {
		if st.IsDir() {
			return path, "", exitError(ExitSafety, a.name+" global instructions path is a directory: "+path)
		}
		mode = st.Mode().Perm()
	} else if !errors.Is(err, os.ErrNotExist) {
		return path, "", err
	}
	next := upsertManagedInstructionBlock(string(data), block)
	if err := writeFileAtomic(path, []byte(next), mode); err != nil {
		return path, "", err
	}
	return path, "healed", nil
}

func managedInstructionStatus(content, block string) string {
	return managedBlockStatus(content, block, runtimeInstructionsBegin, runtimeInstructionsEnd)
}

func managedBlockStatus(content, block, begin, endMarker string) string {
	start := strings.Index(content, begin)
	end := strings.Index(content, endMarker)
	if start == -1 && end == -1 {
		return "missing"
	}
	if start == -1 || end == -1 || end < start {
		return "partial"
	}
	end += len(endMarker)
	current := content[start:end]
	if strings.TrimSpace(current) == strings.TrimSpace(block) {
		return "ok"
	}
	return "stale"
}

func upsertManagedInstructionBlock(content, block string) string {
	return upsertManagedBlock(content, block, runtimeInstructionsBegin, runtimeInstructionsEnd)
}

func upsertManagedBlock(content, block, begin, endMarker string) string {
	start := strings.Index(content, begin)
	end := strings.Index(content, endMarker)
	if start != -1 && end != -1 && end >= start {
		end += len(endMarker)
		for end < len(content) && (content[end] == '\n' || content[end] == '\r') {
			end++
		}
		return content[:start] + block + content[end:]
	}
	if strings.TrimSpace(content) == "" {
		return block
	}
	trimmed := strings.TrimRight(content, "\r\n")
	return trimmed + "\n\n" + block
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".agent-landlock-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	cleanup = false
	return nil
}
