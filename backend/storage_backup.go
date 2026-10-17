package main

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"os"
	"path/filepath"
)

func (s *sqliteStore) Backup(ctx context.Context, destination string) error {
	return snapshotSQLite(ctx, s.writer, destination)
}
func snapshotSQLite(ctx context.Context, db *sql.DB, destination string) (resultErr error) {
	path, err := filepath.Abs(destination)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		os.Remove(path)
		return err
	}
	defer func() {
		if resultErr != nil {
			os.Remove(path)
		}
	}()
	if _, err = db.ExecContext(ctx, "VACUUM INTO ?", path); err != nil {
		return err
	}
	f, err = os.OpenFile(path, os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	return errors.Join(f.Sync(), f.Close())
}
func backupExistingSQLite(ctx context.Context, dir, destination string) error {
	path, err := filepath.Abs(filepath.Join(dir, "pulse.db"))
	if err != nil {
		return err
	}
	if _, err = os.Stat(path); err != nil {
		return err
	}
	name := filepath.ToSlash(path)
	if filepath.VolumeName(path) != "" {
		name = "/" + name
	}
	uri := url.URL{Scheme: "file", Path: name}
	q := url.Values{"mode": {"ro"}, "_pragma": {"busy_timeout(5000)"}}
	uri.RawQuery = q.Encode()
	db, err := sql.Open("sqlite", uri.String())
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	return snapshotSQLite(ctx, db, destination)
}
