package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
	"testing"
)

func TestSQLiteMigrationChecksumSurvivesGitLineEndings(t *testing.T) {
	raw, err := storageSchemas.ReadFile("storage_schema/001_telemetry.sql")
	if err != nil {
		t.Fatal(err)
	}
	canonical := strings.ReplaceAll(string(raw), "\r\n", "\n")
	for _, tc := range []struct{ name, sql string }{{"LF", canonical}, {"CRLF", strings.ReplaceAll(canonical, "\n", "\r\n")}} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			db, err := openSQLite(dir)
			if err != nil {
				t.Fatal(err)
			}
			digest := sha256.Sum256([]byte(tc.sql))
			if _, err := db.writer.Exec("UPDATE schema_migrations SET checksum=?", hex.EncodeToString(digest[:])); err != nil {
				t.Fatal(err)
			}
			db.Close()
			next, err := openSQLite(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer next.Close()
			var stored string
			if err := next.reader.QueryRow("SELECT checksum FROM schema_migrations").Scan(&stored); err != nil {
				t.Fatal(err)
			}
			expected := sha256.Sum256([]byte(canonical))
			if stored != hex.EncodeToString(expected[:]) {
				t.Fatal("checksum not canonicalized")
			}
		})
	}
}

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
