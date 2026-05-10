package agentlsm

import (
	"path/filepath"
	"testing"
	"time"
)

func TestGrantStateUsesAgentLSMDir(t *testing.T) {
	td := t.TempDir()
	t.Setenv("XDG_STATE_HOME", td)
	path, err := grantsFile()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(td, "agent-lsm", "grants.json")
	if path != want {
		t.Fatalf("grantsFile = %s, want %s", path, want)
	}
}

func TestUpsertRemoveGrant(t *testing.T) {
	td := t.TempDir()
	t.Setenv("XDG_STATE_HOME", filepath.Join(td, "state"))
	target := filepath.Join(td, "work")
	if err := upsertGrant(target, ""); err != nil {
		t.Fatal(err)
	}
	grants, err := readGrants()
	if err != nil {
		t.Fatal(err)
	}
	if len(grants) != 1 || grants[0].Path != target {
		t.Fatalf("grants = %#v", grants)
	}
	removed, err := removeGrant(target)
	if err != nil {
		t.Fatal(err)
	}
	if !removed {
		t.Fatal("expected grant to be removed")
	}
	grants, err = readGrants()
	if err != nil {
		t.Fatal(err)
	}
	if len(grants) != 0 {
		t.Fatalf("grants after remove = %#v", grants)
	}
}

func TestActiveGrantsSplitsExpired(t *testing.T) {
	td := t.TempDir()
	t.Setenv("XDG_STATE_HOME", filepath.Join(td, "state"))
	now := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
	grants := []Grant{
		{Path: "/live", CreatedAt: now.Format(time.RFC3339)},
		{Path: "/expired", CreatedAt: now.Format(time.RFC3339), ExpiresAt: now.Add(-time.Second).Format(time.RFC3339)},
	}
	if err := writeGrants(grants); err != nil {
		t.Fatal(err)
	}
	active, expired, err := activeGrants(now)
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 1 || active[0].Path != "/live" {
		t.Fatalf("active = %#v", active)
	}
	if len(expired) != 1 || expired[0].Path != "/expired" {
		t.Fatalf("expired = %#v", expired)
	}
}
