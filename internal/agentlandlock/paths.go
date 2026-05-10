package agentlandlock

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func stateDir() (string, error) {
	if base := os.Getenv("XDG_STATE_HOME"); base != "" {
		return filepath.Join(base, StateName), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", StateName), nil
}

func grantsFile() (string, error) {
	dir, err := stateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "grants.json"), nil
}

func ensureStateDir() (string, error) {
	dir, err := stateDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

func resolveExistingDir(raw string) (string, error) {
	if raw == "" {
		return "", fmt.Errorf("empty path")
	}
	expanded := expandHome(raw)
	abs, err := filepath.Abs(expanded)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	st, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if !st.IsDir() {
		return "", fmt.Errorf("not a directory: %s", resolved)
	}
	return filepath.Clean(resolved), nil
}

func resolvePathForStatus(raw string) (string, error) {
	if raw == "" {
		raw = "."
	}
	abs, err := filepath.Abs(expandHome(raw))
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return filepath.Clean(resolved), nil
	}
	return filepath.Clean(abs), nil
}

func expandHome(path string) string {
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func safetyCheckWritable(path string, cfg Config, force bool) error {
	clean := filepath.Clean(path)
	home, _ := os.UserHomeDir()
	if home != "" {
		if resolvedHome, err := filepath.EvalSymlinks(home); err == nil {
			home = resolvedHome
		}
		if clean == filepath.Clean(home) && !force {
			return exitError(ExitSafety, fmt.Sprintf("refusing to make all of $HOME writable: %s", clean))
		}
	}
	for _, raw := range cfg.SafetyDenyPaths {
		denied := filepath.Clean(expandHome(raw))
		if resolved, err := filepath.EvalSymlinks(denied); err == nil {
			denied = filepath.Clean(resolved)
		}
		if clean == denied {
			return exitError(ExitSafety, fmt.Sprintf("refusing writable access to safety path: %s", clean))
		}
		if denied != string(filepath.Separator) && pathContains(denied, clean) {
			return exitError(ExitSafety, fmt.Sprintf("refusing writable access under safety path: %s", denied))
		}
	}
	return nil
}

func pathContains(root, path string) bool {
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	if root == path {
		return true
	}
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func uniquePruned(paths []string) []string {
	seen := map[string]bool{}
	var cleaned []string
	for _, path := range paths {
		c := filepath.Clean(path)
		if c == "" || seen[c] {
			continue
		}
		seen[c] = true
		cleaned = append(cleaned, c)
	}
	for i := 0; i < len(cleaned); i++ {
		for j := 0; j < len(cleaned); j++ {
			if i == j {
				continue
			}
			if pathContains(cleaned[j], cleaned[i]) {
				cleaned = append(cleaned[:i], cleaned[i+1:]...)
				i--
				break
			}
		}
	}
	return cleaned
}

func systemWritableRoots(paths []string) []string {
	var out []string
	for _, raw := range paths {
		path, err := resolveExistingDir(raw)
		if err != nil {
			continue
		}
		out = append(out, path)
	}
	return uniquePruned(out)
}

func ensureAgentStateDirs(agent string, noAgentState bool, dryRun bool) ([]string, error) {
	if noAgentState {
		return nil, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	var out []string
	for _, rel := range agentStateDirs[agent] {
		path := filepath.Join(home, rel)
		if err := refuseSymlink(path, "refusing symlinked agent state directory"); err != nil {
			return nil, err
		}
		if dryRun {
			if resolved, err := resolveExistingDir(path); err == nil {
				out = append(out, resolved)
			} else {
				out = append(out, filepath.Clean(path))
			}
			continue
		}
		if err := os.MkdirAll(path, 0o700); err != nil {
			return nil, err
		}
		resolved, err := resolveExistingDir(path)
		if err != nil {
			return nil, err
		}
		out = append(out, resolved)
	}
	return out, nil
}
