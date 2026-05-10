package agentlandlock

import (
	"io"
	"os"
	"path/filepath"
)

func prepareAgentEnv(agent string, env map[string]string, noAgentState bool, dryRun bool) error {
	if agent != "claude" || noAgentState || env["CLAUDE_CONFIG_DIR"] != "" {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	configDir := filepath.Join(home, ".claude")
	env["CLAUDE_CONFIG_DIR"] = configDir
	if dryRun {
		return nil
	}
	if err := refuseSymlink(configDir, "refusing symlinked Claude config directory"); err != nil {
		return err
	}
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		return err
	}
	if err := migrateClaudeHomeConfig(home, configDir); err != nil {
		return err
	}
	return nil
}

func migrateClaudeHomeConfig(home, configDir string) error {
	src := filepath.Join(home, ".claude.json")
	dst := filepath.Join(configDir, ".claude.json")
	if err := refuseSymlink(src, "refusing symlinked Claude home config"); err != nil {
		return err
	}
	if err := refuseSymlink(dst, "refusing symlinked Claude bridged config"); err != nil {
		return err
	}
	if _, err := os.Stat(dst); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	st, err := os.Stat(src)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if st.IsDir() {
		return exitError(ExitSafety, "Claude home config is a directory: "+src)
	}
	return copyFileAtomic(src, dst, 0o600)
}

func copyFileAtomic(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp, err := os.CreateTemp(filepath.Dir(dst), "."+filepath.Base(dst)+".agent-landlock-*")
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
	if _, err := io.Copy(tmp, in); err != nil {
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
	if err := os.Rename(tmpName, dst); err != nil {
		return err
	}
	cleanup = false
	return nil
}

func refuseSymlink(path, message string) error {
	st, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if st.Mode()&os.ModeSymlink != 0 {
		return exitError(ExitSafety, message+": "+path)
	}
	return nil
}
