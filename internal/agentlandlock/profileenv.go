package agentlandlock

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const (
	claudeEnvBegin = "# agent-landlock:claude-env begin"
	claudeEnvEnd   = "# agent-landlock:claude-env end"
)

// profileEnvScanFiles are home-relative rc files scanned for a user-managed
// CLAUDE_CONFIG_DIR export. Heal only ever writes to the first entry.
var profileEnvScanFiles = []string{".profile", ".bash_profile", ".bashrc", ".zshenv", ".zprofile", ".zshrc"}

func claudeEnvBlock() string {
	return claudeEnvBegin + "\n" +
		`export CLAUDE_CONFIG_DIR="$HOME/.claude"` + "\n" +
		claudeEnvEnd + "\n"
}

// checkClaudeProfileEnv reports whether plain (non-landlocked) shells export
// CLAUDE_CONFIG_DIR, so plain claude runs share the bridged config that
// agent-landlock injects for sandboxed runs.
//
// Status values: "ok" (managed block in ~/.profile is current),
// "ok (user-managed)" (export found outside a managed block),
// "stale", "partial", "missing".
func checkClaudeProfileEnv() (string, string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}
	profilePath := filepath.Join(home, profileEnvScanFiles[0])
	profileContent, err := readFileIfExists(profilePath)
	if err != nil {
		return profilePath, "", err
	}
	managed := managedBlockStatus(profileContent, claudeEnvBlock(), claudeEnvBegin, claudeEnvEnd)
	if managed == "ok" {
		return profilePath, "ok", nil
	}
	for _, name := range profileEnvScanFiles {
		path := filepath.Join(home, name)
		content, err := readFileIfExists(path)
		if err != nil {
			return profilePath, "", err
		}
		if hasUserManagedClaudeEnv(content) {
			return path, "ok (user-managed)", nil
		}
	}
	return profilePath, managed, nil
}

// healClaudeProfileEnv upserts the managed CLAUDE_CONFIG_DIR block into
// ~/.profile unless an export is already in effect.
func healClaudeProfileEnv(dryRun bool) (string, string, error) {
	path, status, err := checkClaudeProfileEnv()
	if err != nil {
		return path, "", err
	}
	if status == "ok" || status == "ok (user-managed)" {
		return path, status, nil
	}
	if err := refuseSymlink(path, "refusing symlinked shell profile"); err != nil {
		return path, "", err
	}
	if dryRun {
		return path, "would heal", nil
	}
	mode := os.FileMode(0o644)
	var content string
	if st, err := os.Stat(path); err == nil {
		if st.IsDir() {
			return path, "", exitError(ExitSafety, "shell profile path is a directory: "+path)
		}
		mode = st.Mode().Perm()
		data, err := os.ReadFile(path)
		if err != nil {
			return path, "", err
		}
		content = string(data)
	} else if !errors.Is(err, os.ErrNotExist) {
		return path, "", err
	}
	next := upsertManagedBlock(content, claudeEnvBlock(), claudeEnvBegin, claudeEnvEnd)
	if err := writeFileAtomic(path, []byte(next), mode); err != nil {
		return path, "", err
	}
	return path, "healed", nil
}

// checkClaudeConfigSplit reports "split" when the legacy ~/.claude.json was
// written after the bridged ~/.claude/.claude.json, meaning plain claude runs
// still use (and diverge) the legacy config. Detect-only; never healed.
func checkClaudeConfigSplit() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	legacy, err := os.Stat(filepath.Join(home, ".claude.json"))
	if errors.Is(err, os.ErrNotExist) {
		return "ok", nil
	}
	if err != nil {
		return "", err
	}
	bridged, err := os.Stat(filepath.Join(home, ".claude", ".claude.json"))
	if errors.Is(err, os.ErrNotExist) {
		return "ok", nil
	}
	if err != nil {
		return "", err
	}
	if legacy.ModTime().After(bridged.ModTime()) {
		return "split", nil
	}
	return "ok", nil
}

// healClaudeConfigSymlink replaces the legacy ~/.claude.json with a symlink to
// the bridged ~/.claude/.claude.json so plain claude runs resolve to the same
// config that sandboxed runs write, even when CLAUDE_CONFIG_DIR is not
// exported. A diverged legacy config (a real split) is never clobbered.
//
// Status values: "ok" (already a symlink to the bridged config), "healed",
// "would heal" (dry-run), "missing" (no config to link), "split" (legacy
// diverged; merge first), "skipped (foreign symlink)".
func healClaudeConfigSymlink(dryRun bool) (string, string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}
	src := filepath.Join(home, ".claude.json")
	dst := filepath.Join(home, ".claude", ".claude.json")

	srcInfo, err := os.Lstat(src)
	srcExists := err == nil
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return src, "", err
	}

	// Already a symlink: adopt our own target, never touch a foreign one.
	if srcExists && srcInfo.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(src)
		if err != nil {
			return src, "", err
		}
		if target == dst {
			return src, "ok", nil
		}
		return src, "skipped (foreign symlink)", nil
	}

	dstExists := false
	if _, err := os.Stat(dst); err == nil {
		dstExists = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return src, "", err
	}

	switch {
	case !srcExists && !dstExists:
		return src, "missing", nil
	case srcExists && srcInfo.IsDir():
		return src, "", exitError(ExitSafety, "Claude home config is a directory: "+src)
	case srcExists && dstExists:
		// Both are regular files; refuse to discard a diverged legacy config.
		split, err := checkClaudeConfigSplit()
		if err != nil {
			return src, "", err
		}
		if split == "split" {
			return src, "split", nil
		}
	}

	if dryRun {
		return src, "would heal", nil
	}

	// Ensure the bridged config exists, bridging the legacy file if needed.
	if !dstExists {
		if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
			return src, "", err
		}
		if srcExists {
			if err := copyFileAtomic(src, dst, 0o600); err != nil {
				return src, "", err
			}
		}
	}
	if srcExists {
		if err := os.Remove(src); err != nil {
			return src, "", err
		}
	}
	if err := os.Symlink(dst, src); err != nil {
		return src, "", err
	}
	return src, "healed", nil
}

func readFileIfExists(path string) (string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// hasUserManagedClaudeEnv reports whether content mentions CLAUDE_CONFIG_DIR
// on a non-comment line outside the managed block.
func hasUserManagedClaudeEnv(content string) bool {
	inManaged := false
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		switch trimmed {
		case claudeEnvBegin:
			inManaged = true
			continue
		case claudeEnvEnd:
			inManaged = false
			continue
		}
		if inManaged || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.Contains(trimmed, "CLAUDE_CONFIG_DIR") {
			return true
		}
	}
	return false
}
