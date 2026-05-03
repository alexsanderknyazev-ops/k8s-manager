package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMigrationTimeout_default(t *testing.T) {
	t.Setenv("MIGRATE_TIMEOUT", "")
	if d := migrationTimeout(); d != 2*time.Minute {
		t.Fatalf("got %v", d)
	}
}

func TestMigrationTimeout_envValid(t *testing.T) {
	t.Setenv("MIGRATE_TIMEOUT", "90s")
	if d := migrationTimeout(); d != 90*time.Second {
		t.Fatalf("got %v", d)
	}
}

func TestMigrationTimeout_envInvalidFallsBack(t *testing.T) {
	t.Setenv("MIGRATE_TIMEOUT", "not-a-duration")
	if d := migrationTimeout(); d != 2*time.Minute {
		t.Fatalf("got %v", d)
	}
}

func TestListMigrations_sortsAndParses(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "002_second.up.sql"), []byte("SELECT 1;"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "001_first.up.sql"), []byte("SELECT 1;"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := listMigrations(dir, ".up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 || out[0].version != 1 || out[1].version != 2 {
		t.Fatalf("got %+v", out)
	}

	if err := os.WriteFile(filepath.Join(dir, "badname.up.sql"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = listMigrations(dir, ".up.sql")
	if err == nil {
		t.Fatal("expected error for invalid migration filename")
	}
}
