package main

import (
	"context"
	"os"
	"time"
)

type walStatistics struct {
	Busy, Frames, Checkpointed int
	Bytes                      int64
}

// PASSIVE never waits for readers or takes their snapshots away. Report the
// backlog so a pinned reader can be distinguished from an idle reusable WAL.
func (s *sqliteStore) checkpoint(ctx context.Context) (walStatistics, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	var stats walStatistics
	err := s.writer.QueryRowContext(ctx, "PRAGMA wal_checkpoint(PASSIVE)").Scan(&stats.Busy, &stats.Frames, &stats.Checkpointed)
	if err != nil {
		return stats, err
	}
	info, err := os.Stat(s.path + "-wal")
	if os.IsNotExist(err) {
		return stats, nil
	}
	if err != nil {
		return stats, err
	}
	stats.Bytes = info.Size()
	return stats, nil
}
