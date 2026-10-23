package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestHistoryOwnDeadlineIncludesWaitingForReader(t *testing.T) {
	s, err := openSQLite(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var held []*sql.Conn
	for i := 0; i < 4; i++ {
		c, err := s.reader.Conn(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		held = append(held, c)
	}
	defer func() {
		for _, c := range held {
			_ = c.Close()
		}
	}()
	parent, cancel := context.WithTimeout(context.Background(), 7*time.Second)
	defer cancel()
	start := time.Now()
	_, err = s.queryLogs(parent, "node", "", "", "", "", 0, 100, "")
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > 6*time.Second {
		t.Fatalf("history did not enforce its own deadline: %s %v", time.Since(start), err)
	}
}

func TestCheckpointObservesPinnedReaderAndRecovers(t *testing.T) {
	s, err := openSQLite(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	if err = s.appendLogs(ctx, []logEntry{{ID: "seed", ServerID: "node", Level: "INFO", Message: "seed"}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	tx, err := s.reader.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	var count int
	if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM collected_logs").Scan(&count); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		if err = s.appendLogs(ctx, []logEntry{{ID: fmt.Sprint("live-", i), ServerID: "node", Level: "INFO", Message: "live"}}, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	stats, err := s.checkpoint(ctx)
	if err != nil || stats.Frames <= stats.Checkpointed || stats.Bytes <= 0 {
		t.Fatalf("pinned WAL not observed: %+v %v", stats, err)
	}
	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	stats, err = s.checkpoint(ctx)
	if err != nil || stats.Busy != 0 || stats.Frames != stats.Checkpointed {
		t.Fatalf("reader did not release checkpoint: %+v %v", stats, err)
	}
}
