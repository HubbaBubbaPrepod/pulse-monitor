package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	_ "modernc.org/sqlite"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

//go:embed storage_schema/*.sql
var storageSchemas embed.FS

type sqliteStore struct {
	writer, reader *sql.DB
	path           string
	closeOnce      sync.Once
	closeErr       error
}

func (s *sqliteStore) Close() error {
	if s == nil {
		return nil
	}
	s.closeOnce.Do(func() {
		if s.reader != nil {
			s.closeErr = s.reader.Close()
		}
		if s.writer != nil {
			s.closeErr = errors.Join(s.closeErr, s.writer.Close())
		}
	})
	return s.closeErr
}
func openSQLite(dir string) (result *sqliteStore, err error) {
	path, err := filepath.Abs(filepath.Join(dir, "pulse.db"))
	if err != nil {
		return nil, err
	}
	if info, e := os.Lstat(path); e == nil && info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("database symlink is not allowed")
	}
	path = filepath.ToSlash(path)
	if filepath.VolumeName(path) != "" {
		path = "/" + path
	}
	uri := url.URL{Scheme: "file", Path: path}
	q := url.Values{}
	for _, v := range []string{"busy_timeout(5000)", "foreign_keys(1)", "synchronous(FULL)"} {
		q.Add("_pragma", v)
	}
	uri.RawQuery = q.Encode()
	s := &sqliteStore{path: filepath.Join(dir, "pulse.db")}
	defer func() {
		if err != nil {
			s.Close()
		}
	}()
	s.writer, err = sql.Open("sqlite", uri.String())
	if err != nil {
		return nil, err
	}
	s.writer.SetMaxOpenConns(1)
	s.writer.SetMaxIdleConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err = s.writer.PingContext(ctx); err != nil {
		return nil, err
	}
	if err = os.Chmod(filepath.Join(dir, "pulse.db"), 0600); err != nil {
		return nil, err
	}
	var mode string
	if err = s.writer.QueryRowContext(ctx, "PRAGMA journal_mode=WAL").Scan(&mode); err != nil {
		return nil, err
	}
	if mode != "wal" {
		return nil, errors.New("SQLite WAL could not be enabled")
	}
	// Recycle an oversized journal after a completed checkpoint; this is not
	// a hard quota and cannot truncate pages pinned by an active reader.
	if _, err = s.writer.ExecContext(ctx, "PRAGMA journal_size_limit=16777216"); err != nil {
		return nil, err
	}
	var version string
	if err = s.writer.QueryRowContext(ctx, "SELECT sqlite_version()").Scan(&version); err != nil {
		return nil, err
	}
	var major, minor, patch int
	if _, err = fmt.Sscanf(version, "%d.%d.%d", &major, &minor, &patch); err != nil {
		return nil, err
	}
	if major < 3 || (major == 3 && (minor < 51 || (minor == 51 && patch < 3))) {
		return nil, fmt.Errorf("SQLite %s lacks required WAL fix", version)
	}
	if err = s.applySchema(ctx); err != nil {
		return nil, err
	}
	s.reader, err = sql.Open("sqlite", uri.String())
	if err != nil {
		return nil, err
	}
	s.reader.SetMaxOpenConns(4)
	s.reader.SetMaxIdleConns(4)
	s.reader.SetConnMaxIdleTime(5 * time.Minute)
	if err = s.reader.PingContext(ctx); err != nil {
		return nil, err
	}
	return s, nil
}
func (s *sqliteStore) applySchema(ctx context.Context) error {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS schema_migrations(name TEXT PRIMARY KEY,checksum TEXT NOT NULL)"); err != nil {
		return err
	}
	entries, err := storageSchemas.ReadDir("storage_schema")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		raw, err := storageSchemas.ReadFile("storage_schema/" + entry.Name())
		if err != nil {
			return err
		}
		canonical := strings.ReplaceAll(string(raw), "\r\n", "\n")
		sum := sha256.Sum256([]byte(canonical))
		digest := hex.EncodeToString(sum[:])
		var old string
		err = tx.QueryRowContext(ctx, "SELECT checksum FROM schema_migrations WHERE name=?", entry.Name()).Scan(&old)
		if err == nil {
			if old != digest {
				// Early Windows builds embedded CRLF. Accept only that exact
				// equivalent and store the portable LF checksum for later builds.
				windowsSum := sha256.Sum256([]byte(strings.ReplaceAll(canonical, "\n", "\r\n")))
				if old != hex.EncodeToString(windowsSum[:]) {
					return fmt.Errorf("migration checksum mismatch: %s", entry.Name())
				}
				if _, err = tx.ExecContext(ctx, "UPDATE schema_migrations SET checksum=? WHERE name=?", digest, entry.Name()); err != nil {
					return err
				}
			}
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if _, err = tx.ExecContext(ctx, string(raw)); err != nil {
			return fmt.Errorf("migration %s: %w", entry.Name(), err)
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO schema_migrations(name,checksum) VALUES(?,?)", entry.Name(), digest); err != nil {
			return err
		}
	}
	return tx.Commit()
}
