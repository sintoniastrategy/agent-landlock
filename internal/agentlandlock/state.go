package agentlandlock

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

type Grant struct {
	Path      string `json:"path"`
	CreatedAt string `json:"created_at"`
	ExpiresAt string `json:"expires_at,omitempty"`
}

type stateLock struct {
	file *os.File
}

func lockState() (*stateLock, error) {
	dir, err := ensureStateDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "state.lock")
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		file.Close()
		return nil, err
	}
	return &stateLock{file: file}, nil
}

func (l *stateLock) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	err1 := syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	err2 := l.file.Close()
	if err1 != nil {
		return err1
	}
	return err2
}

func readGrants() ([]Grant, error) {
	path, err := grantsFile()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var grants []Grant
	if len(data) == 0 {
		return nil, nil
	}
	if err := json.Unmarshal(data, &grants); err != nil {
		return nil, fmt.Errorf("read grants: %w", err)
	}
	return grants, nil
}

func writeGrants(grants []Grant) error {
	dir, err := ensureStateDir()
	if err != nil {
		return err
	}
	path, err := grantsFile()
	if err != nil {
		return err
	}
	if len(grants) == 0 {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	data, err := json.MarshalIndent(grants, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(dir, ".grants-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	ok = true
	return nil
}

func activeGrants(now time.Time) ([]Grant, []Grant, error) {
	grants, err := readGrants()
	if err != nil {
		return nil, nil, err
	}
	var active []Grant
	var expired []Grant
	for _, grant := range grants {
		if grant.ExpiresAt == "" {
			active = append(active, grant)
			continue
		}
		when, err := time.Parse(time.RFC3339, grant.ExpiresAt)
		if err != nil || when.After(now) {
			active = append(active, grant)
			continue
		}
		expired = append(expired, grant)
	}
	return active, expired, nil
}

func upsertGrant(path string, expiresAt string) error {
	lock, err := lockState()
	if err != nil {
		return err
	}
	defer lock.Close()
	grants, err := readGrants()
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	row := Grant{Path: path, CreatedAt: now, ExpiresAt: expiresAt}
	replaced := false
	var kept []Grant
	for _, grant := range grants {
		if grant.Path == path {
			kept = append(kept, row)
			replaced = true
			continue
		}
		kept = append(kept, grant)
	}
	if !replaced {
		kept = append(kept, row)
	}
	return writeGrants(kept)
}

func removeGrant(path string) (bool, error) {
	lock, err := lockState()
	if err != nil {
		return false, err
	}
	defer lock.Close()
	grants, err := readGrants()
	if err != nil {
		return false, err
	}
	removed := false
	var kept []Grant
	for _, grant := range grants {
		if grant.Path == path {
			removed = true
			continue
		}
		kept = append(kept, grant)
	}
	if !removed {
		return false, nil
	}
	return true, writeGrants(kept)
}

func cleanupExpiredGrants() (int, error) {
	lock, err := lockState()
	if err != nil {
		return 0, err
	}
	defer lock.Close()
	active, expired, err := activeGrants(time.Now().UTC())
	if err != nil {
		return 0, err
	}
	if len(expired) == 0 {
		return 0, nil
	}
	return len(expired), writeGrants(active)
}
