package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestSQLiteLifecycleAndMigrationChecksum(t *testing.T) {
	dir := t.TempDir()
	db, err := openSQLite(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	var mode, version string
	var syncMode int
	if err := db.writer.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatal(err)
	}
	if err := db.writer.QueryRowContext(ctx, "PRAGMA synchronous").Scan(&syncMode); err != nil {
		t.Fatal(err)
	}
	if err := db.writer.QueryRowContext(ctx, "SELECT sqlite_version()").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if mode != "wal" || syncMode != 2 {
		t.Fatalf("durability: %s/%d", mode, syncMode)
	}
	t.Log("SQLite", version, "path", filepath.Join(dir, "pulse.db"))
	if _, err := db.writer.ExecContext(ctx, "UPDATE schema_migrations SET checksum='changed'"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	next, err := openSQLite(dir)
	if next != nil {
		next.Close()
	}
	if err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("changed migration accepted: %v", err)
	}
}
