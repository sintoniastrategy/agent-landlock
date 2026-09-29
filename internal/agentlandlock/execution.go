package agentlandlock

import (
	"fmt"
	"os"
	"path/filepath"
)

func agentExecutionDenied(env map[string]string, workdir string) ([]string, error) {
	home := env["HOME"]
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return nil, err
		}
	}
	paths := []string{filepath.Join(home, ".codex"), filepath.Join(home, ".claude")}
	for _, key := range []string{"CODEX_HOME", "CLAUDE_CONFIG_DIR"} {
		if custom := env[key]; custom != "" {
			paths = append(paths, custom)
		}
	}
	for i, path := range paths {
		if !filepath.IsAbs(path) {
			path = filepath.Join(workdir, path)
		}
		resolved, err := resolveExecutionPath(path)
		if err != nil {
			return nil, fmt.Errorf("resolve non-executable agent directory %q: %w", path, err)
		}
		paths[i] = resolved
	}
	return uniquePruned(paths), nil
}

func resolveExecutionPath(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return resolved, nil
	}
	if !os.IsNotExist(err) || filepath.Dir(path) == path {
		return "", err
	}
	if info, statErr := os.Lstat(path); statErr == nil && info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(path)
		if err != nil {
			return "", err
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(path), target)
		}
		return resolveExecutionPath(target)
	}
	parent, err := resolveExecutionPath(filepath.Dir(path))
	if err != nil {
		return "", err
	}
	return filepath.Join(parent, filepath.Base(path)), nil
}

func executableRoots(root string, denied []string) ([]string, error) {
	split := false
	for _, path := range denied {
		if pathContains(path, root) {
			return nil, nil
		}
		if pathContains(root, path) {
			split = true
		}
	}
	if !split {
		return []string{root}, nil
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var allowed []string
	for _, entry := range entries {
		if !entry.IsDir() && !entry.Type().IsRegular() {
			continue
		}
		paths, err := executableRoots(filepath.Join(root, entry.Name()), denied)
		if err != nil {
			return nil, err
		}
		allowed = append(allowed, paths...)
	}
	return allowed, nil
}
