package main

import (
	"errors"
	"path/filepath"
)

var errStorageLow = errors.New("monitor volume has less than 128 MiB free")

func (s *sqliteStore) ensureReportSpace() error {
	free, err := availableStorageBytes(filepath.Dir(s.path))
	if err != nil {
		return err
	}
	// Preserve operating headroom instead of consuming the last free blocks
	// with authenticated telemetry. The agent retains and retries its receipt.
	if free < 128*1024*1024 {
		return errStorageLow
	}
	return nil
}
