package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSQLiteLegacyImportResumesWithoutChangingSources(t *testing.T) {
	dir := t.TempDir()
	raw := `{"id":"legacy","serverId":"node","time":"2026-10-01T00:00:00Z","level":"INFO","source":"systemd","message":"preserved"}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "logs.jsonl"), []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	db, err := openSQLite(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for range 2 {
		if err := db.migrateLegacy(context.Background(), dir, nil); err != nil {
			t.Fatal(err)
		}
	}
	page, err := db.queryLogs(context.Background(), "node", "", "", "", "", 0, 100, "")
	if err != nil || page.Total != 1 {
		t.Fatalf("import: %+v %v", page, err)
	}
	saved, err := os.ReadFile(filepath.Join(dir, "logs.jsonl"))
	if err != nil || string(saved) != raw {
		t.Fatal("source changed", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "legacy-sqlite-backup", "logs.jsonl")); err != nil {
		t.Fatal(err)
	}
	var received int64
	if err := db.reader.QueryRow("SELECT received_ns FROM collected_logs").Scan(&received); err != nil {
		t.Fatal(err)
	}
	if time.Since(time.Unix(0, received)) > time.Minute {
		t.Fatal("remote/source time used as retention receipt")
	}
}
func TestSQLiteLegacyImportRejectsCorruptionWithoutActivation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "logs.jsonl")
	if err := os.WriteFile(path, []byte("{broken\n"), 0600); err != nil {
		t.Fatal(err)
	}
	db, err := openSQLite(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	err = db.migrateLegacy(context.Background(), dir, nil)
	if err == nil || !strings.Contains(err.Error(), "logs.jsonl") {
		t.Fatalf("corruption not diagnosed: %v", err)
	}
	var count int
	if err := db.reader.QueryRow("SELECT count(*) FROM storage_meta WHERE key='active'").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("partial import activated")
	}
}

func TestSQLitePartialImportRejectsChangedLegacySource(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "logs.jsonl")
	if err := os.WriteFile(path, []byte("{broken\n"), 0600); err != nil {
		t.Fatal(err)
	}
	db, err := openSQLite(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.migrateLegacy(context.Background(), dir, nil); err == nil {
		t.Fatal("expected failed import")
	}
	if err := os.WriteFile(path, []byte("{changed\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := db.migrateLegacy(context.Background(), dir, nil); err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("source change ignored: %v", err)
	}
}

func TestSQLiteImportResumesAtCommittedBatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "records.jsonl")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 1001; i++ {
		fmt.Fprintln(f, `{"id":"`+fmt.Sprint(i)+`"}`)
	}
	f.Close()
	db, err := openSQLite(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	calls := 0
	insert := func(ctx context.Context, tx *sql.Tx, raw []byte, _ int64) error {
		calls++
		if calls == 502 {
			return errors.New("simulated stop")
		}
		_, err := tx.ExecContext(ctx, "INSERT INTO storage_meta(key,value) VALUES(?,?) ON CONFLICT DO NOTHING", string(raw), "test")
		return err
	}
	if err := db.importLines(context.Background(), dir, "records.jsonl", insert); err == nil {
		t.Fatal("stop was not simulated")
	}
	var count int
	if err := db.reader.QueryRow("SELECT imported FROM import_progress WHERE name='records.jsonl'").Scan(&count); err != nil || count != 500 {
		t.Fatalf("uncommitted cursor: %d %v", count, err)
	}
	calls = 10000
	if err := db.importLines(context.Background(), dir, "records.jsonl", insert); err != nil {
		t.Fatal(err)
	}
	if err := db.reader.QueryRow("SELECT count(*) FROM storage_meta").Scan(&count); err != nil || count != 1001 {
		t.Fatalf("resume lost/duplicated records: %d %v", count, err)
	}
}
