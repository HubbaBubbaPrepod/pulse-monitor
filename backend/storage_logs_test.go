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

func TestReaderConnectionsRejectWrites(t *testing.T) {
	s, err := openSQLite(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var held []*sql.Conn
	defer func() {
		for _, c := range held {
			c.Close()
		}
	}()
	for i := 0; i < 4; i++ {
		c, err := s.reader.Conn(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		held = append(held, c)
		if _, err = c.ExecContext(t.Context(), "INSERT INTO storage_meta(key,value) VALUES(?,?)", fmt.Sprint(i), "reader must not write"); err == nil {
			t.Fatalf("reader connection %d allowed a write", i)
		}
	}
	if _, err = s.writer.ExecContext(t.Context(), "INSERT INTO storage_meta(key,value) VALUES('writer','allowed')"); err != nil {
		t.Fatalf("writer lost its permissions: %v", err)
	}
}

func TestStartupHistoryLoadHasOwnDeadline(t *testing.T) {
	s, err := openSQLite(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var held []*sql.Conn
	for i := 0; i < 4; i++ {
		c, err := s.reader.Conn(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		held = append(held, c)
	}
	release := time.AfterFunc(7*time.Second, func() {
		for _, c := range held {
			c.Close()
		}
	})
	defer release.Stop()
	defer func() {
		for _, c := range held {
			c.Close()
		}
	}()
	start := time.Now()
	_, err = s.loadNodes(context.Background(), []server{{ID: "node"}})
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > 6*time.Second {
		t.Fatalf("startup history did not enforce its own deadline: %s %v", time.Since(start), err)
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

func TestBusyHistoryCancellationReleasesWALDuringLiveWrites(t *testing.T) {
	s, err := openSQLite(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	seed := make([]logEntry, 512)
	for i := range seed {
		seed[i] = logEntry{ID: fmt.Sprint("seed-", i), ServerID: "node", Level: "INFO", Message: "history row"}
	}
	if err := s.appendLogs(t.Context(), seed, time.Now()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	tx, err := s.reader.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	var count int
	if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM collected_logs").Scan(&count); err != nil {
		t.Fatal(err)
	}
	// Amplify actual history-table reads without a huge fixture or a sleep-based
	// race: this aggregate cannot finish before the small live-write workload.
	queryDone := make(chan error, 1)
	go func() {
		var total int64
		queryDone <- tx.QueryRowContext(ctx, `SELECT sum(length(a.search)+length(b.search)+length(c.search)+length(d.search)) FROM collected_logs a CROSS JOIN collected_logs b CROSS JOIN collected_logs c CROSS JOIN collected_logs d`).Scan(&total)
	}()
	for i := 0; i < 20; i++ {
		if err = s.appendLogs(t.Context(), []logEntry{{ID: fmt.Sprint("live-", i), ServerID: "node", Level: "INFO", Message: "live report"}}, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	stats, err := s.checkpoint(t.Context())
	if err != nil || stats.Frames <= stats.Checkpointed {
		t.Fatalf("reader did not pin concurrent writes: %+v %v", stats, err)
	}
	cancel()
	select {
	case err := <-queryDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("history query ignored cancellation: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("canceled history query did not release the reader")
	}
	_ = tx.Rollback()
	// database/sql may finish the context-triggered rollback asynchronously.
	deadline := time.Now().Add(3 * time.Second)
	for {
		stats, err = s.checkpoint(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if stats.Frames == stats.Checkpointed {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("canceled history kept WAL pinned: %+v", stats)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err = s.reader.QueryRowContext(t.Context(), "SELECT count(*) FROM collected_logs").Scan(&count); err != nil || count != 532 {
		t.Fatalf("live writes or reader recovery lost: count=%d err=%v", count, err)
	}
}
