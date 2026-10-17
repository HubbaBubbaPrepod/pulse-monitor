package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func insertLogs(ctx context.Context, tx *sql.Tx, rows []logEntry, received time.Time) error {
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO collected_logs(id,server_id,source_ns,received_ns,level,source,category,search,body) VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO NOTHING`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, entry := range rows {
		if entry.ID == "" {
			var id [16]byte
			if _, err := rand.Read(id[:]); err != nil {
				return err
			}
			entry.ID = hex.EncodeToString(id[:])
		}
		if entry.Time == "" {
			entry.Time = received.UTC().Format(time.RFC3339Nano)
		}
		stamp, err := time.Parse(time.RFC3339Nano, entry.Time)
		if err != nil {
			return fmt.Errorf("log %s timestamp: %w", entry.ID, err)
		}
		if entry.DataSource == "" {
			entry.DataSource = "demo"
			if strings.HasPrefix(entry.ID, entry.ServerID+"-") {
				entry.DataSource = "agent"
			}
		}
		body, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		if _, err = stmt.ExecContext(ctx, entry.ID, entry.ServerID, stamp.UnixNano(), received.UnixNano(), entry.Level, entry.Source, logCategory(entry), strings.ToLower(entry.Source+" "+entry.Message+" "+entry.Time), body); err != nil {
			return err
		}
	}
	return nil
}
func (s *sqliteStore) appendLogs(ctx context.Context, rows []logEntry, received time.Time) error {
	if len(rows) == 0 {
		return nil
	}
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = insertLogs(ctx, tx, rows, received); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *sqliteStore) queryLogs(ctx context.Context, id, level, source, query, before string, offset, limit int, category string) (logPage, error) {
	out := logPage{Items: []logEntry{}}
	where := "server_id=?"
	args := []any{id}
	for _, filter := range []struct{ col, value string }{{"level", level}, {"source", source}, {"category", category}} {
		if filter.value != "" {
			where += " AND " + filter.col + "=?"
			args = append(args, filter.value)
		}
	}
	if query != "" {
		where += " AND instr(search,?)>0"
		args = append(args, strings.ToLower(query))
	}
	if before != "" {
		stamp, err := time.Parse(time.RFC3339Nano, before)
		if err != nil {
			return out, err
		}
		where += " AND source_ns<=?"
		args = append(args, stamp.UnixNano())
	}
	tx, err := s.reader.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM collected_logs WHERE "+where, args...).Scan(&out.Total); err != nil {
		return out, err
	}
	pageArgs := append(append([]any{}, args...), limit, offset)
	rows, err := tx.QueryContext(ctx, "SELECT body FROM collected_logs WHERE "+where+" ORDER BY row_no DESC LIMIT ? OFFSET ?", pageArgs...)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			return out, err
		}
		var entry logEntry
		if err = json.Unmarshal(raw, &entry); err != nil {
			return out, err
		}
		out.Items = append(out.Items, entry)
	}
	if err = rows.Err(); err != nil {
		return out, err
	}
	if err = rows.Close(); err != nil {
		return out, err
	}
	return out, tx.Commit()
}
