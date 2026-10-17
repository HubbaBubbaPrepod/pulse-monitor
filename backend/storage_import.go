package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

func (s *sqliteStore) migrateLegacy(ctx context.Context, dir string, servers []server) error {
	var active string
	err := s.reader.QueryRowContext(ctx, "SELECT value FROM storage_meta WHERE key='active'").Scan(&active)
	if err == nil && active == "1" {
		return nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	backup := filepath.Join(dir, "legacy-sqlite-backup")
	if err := os.MkdirAll(backup, 0700); err != nil {
		return err
	}
	names := []string{"servers.json", "account.json", "events.json", "logs.jsonl"}
	for _, node := range servers {
		names = append(names, filepath.Join("agents", node.ID+".json"), filepath.Join("agents", node.ID+".jsonl"))
	}
	for _, name := range names {
		if err := backupLegacyFile(ctx, filepath.Join(dir, name), filepath.Join(backup, name)); err != nil {
			return fmt.Errorf("backup %s: %w", name, err)
		}
	}
	if err := s.importLines(ctx, backup, "logs.jsonl", func(ctx context.Context, tx *sql.Tx, raw []byte, offset int64) error {
		var entry logEntry
		if err := json.Unmarshal(raw, &entry); err != nil {
			return err
		}
		if entry.ID == "" {
			hash := sha256.Sum256(append([]byte(fmt.Sprint(offset)), raw...))
			entry.ID = "legacy-" + hex.EncodeToString(hash[:])
		}
		return insertLogs(ctx, tx, []logEntry{entry}, time.Now().UTC())
	}); err != nil {
		return err
	}
	for _, node := range servers {
		name := filepath.Join("agents", node.ID+".jsonl")
		if err := s.importLines(ctx, backup, name, func(ctx context.Context, tx *sql.Tx, raw []byte, _ int64) error {
			var point sample
			if err := json.Unmarshal(raw, &point); err != nil {
				return err
			}
			stamp, err := time.Parse(time.RFC3339Nano, point.At)
			if err != nil {
				return err
			}
			_, err = tx.ExecContext(ctx, "INSERT INTO metric_samples(server_id,at_ns,body) VALUES(?,?,?) ON CONFLICT DO NOTHING", node.ID, stamp.UnixNano(), raw)
			return err
		}); err != nil {
			return err
		}
	}
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, node := range servers {
		raw, err := os.ReadFile(filepath.Join(backup, "agents", node.ID+".json"))
		if err == nil {
			var n agentNode
			if err := json.Unmarshal(raw, &n); err != nil {
				return fmt.Errorf("agent %s: %w", node.ID, err)
			}
			if _, err = tx.ExecContext(ctx, "INSERT INTO agent_nodes(id,body) VALUES(?,?) ON CONFLICT DO NOTHING", node.ID, raw); err != nil {
				return err
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO log_retention(server_id,days) VALUES(?,0) ON CONFLICT DO NOTHING", node.ID); err != nil {
			return err
		}
	}
	raw, err := os.ReadFile(filepath.Join(backup, "events.json"))
	if err == nil {
		var events []monitorEvent
		if err := json.Unmarshal(raw, &events); err != nil {
			return fmt.Errorf("events.json: %w", err)
		}
		for _, event := range events {
			body, err := json.Marshal(event)
			if err != nil {
				return err
			}
			if _, err = tx.ExecContext(ctx, "INSERT INTO monitor_events(id,server_id,body) VALUES(?,?,?) ON CONFLICT DO NOTHING", event.ID, event.ServerID, body); err != nil {
				return err
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO storage_meta(key,value) VALUES('active','1') ON CONFLICT(key) DO UPDATE SET value='1'"); err != nil {
		return err
	}
	return tx.Commit()
}
func backupLegacyFile(ctx context.Context, source, target string) error {
	f, err := os.Open(source)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := os.Stat(target); err == nil {
		originalHash := sha256.New()
		if _, err := io.Copy(originalHash, f); err != nil {
			return err
		}
		saved, err := os.Open(target)
		if err != nil {
			return err
		}
		defer saved.Close()
		backupHash := sha256.New()
		if _, err := io.Copy(backupHash, saved); err != nil {
			return err
		}
		if hex.EncodeToString(originalHash.Sum(nil)) != hex.EncodeToString(backupHash.Sum(nil)) {
			return errors.New("legacy source changed since backup; restore or reconcile before resuming migration")
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(target), ".backup-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	if err = tmp.Chmod(0600); err != nil {
		return err
	}
	buf := make([]byte, 64*1024)
	for {
		if err = ctx.Err(); err != nil {
			return err
		}
		n, readErr := f.Read(buf)
		if n > 0 {
			if _, err = tmp.Write(buf[:n]); err != nil {
				return err
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	if err = tmp.Sync(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), target)
}
func (s *sqliteStore) importLines(ctx context.Context, dir, name string, insert func(context.Context, *sql.Tx, []byte, int64) error) error {
	f, err := os.Open(filepath.Join(dir, name))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	var offset, count int64
	err = s.reader.QueryRowContext(ctx, "SELECT offset,imported FROM import_progress WHERE name=?", name).Scan(&offset, &count)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if _, err = f.Seek(offset, io.SeekStart); err != nil {
		return err
	}
	reader := bufio.NewReaderSize(f, 64*1024)
	for {
		tx, err := s.writer.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		eof := false
		for i := 0; i < 500; i++ {
			raw, readErr := reader.ReadBytes('\n')
			if readErr != nil && !errors.Is(readErr, io.EOF) {
				tx.Rollback()
				return readErr
			}
			if len(raw) > 2*1024*1024 {
				tx.Rollback()
				return fmt.Errorf("%s record exceeds import limit", name)
			}
			if len(raw) > 0 {
				if err = insert(ctx, tx, raw, offset); err != nil {
					tx.Rollback()
					return fmt.Errorf("%s at byte %d: %w", name, offset, err)
				}
				offset += int64(len(raw))
				count++
			}
			if errors.Is(readErr, io.EOF) {
				eof = true
				break
			}
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO import_progress(name,offset,imported) VALUES(?,?,?) ON CONFLICT(name) DO UPDATE SET offset=excluded.offset,imported=excluded.imported", name, offset, count); err != nil {
			tx.Rollback()
			return err
		}
		if err = tx.Commit(); err != nil {
			return err
		}
		if eof {
			return nil
		}
	}
}
