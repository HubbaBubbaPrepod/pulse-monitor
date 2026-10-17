package main

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestSQLiteBackupIncludesCommittedWALAndRefusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	db, err := openSQLite(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if _, err := db.writer.ExecContext(ctx, "INSERT INTO storage_meta(key,value) VALUES('proof','committed')"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "backup.db")
	if err := db.Backup(ctx, path); err != nil {
		t.Fatal(err)
	}
	saved, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer saved.Close()
	var proof string
	if err := saved.QueryRow("SELECT value FROM storage_meta WHERE key='proof'").Scan(&proof); err != nil || proof != "committed" {
		t.Fatal("backup omitted WAL data", proof, err)
	}

	second := filepath.Join(t.TempDir(), "readonly-backup.db")
	if err := backupExistingSQLite(ctx, dir, second); err != nil {
		t.Fatal("standalone readonly backup", err)
	}
	if err := db.Backup(ctx, path); err == nil {
		t.Fatal("backup overwrote existing file")
	}
	if info, err := os.Stat(path); err != nil || info.Size() == 0 {
		t.Fatal("backup missing", err)
	}
}
