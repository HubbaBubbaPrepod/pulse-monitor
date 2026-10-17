package main

import (
	"context"
	"database/sql"
	"encoding/json"
)

func insertEvents(ctx context.Context, tx *sql.Tx, events []monitorEvent) error {
	for _, event := range events {
		raw, err := json.Marshal(event)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO monitor_events(id,server_id,body) VALUES(?,?,?) ON CONFLICT(id) DO NOTHING", event.ID, event.ServerID, raw); err != nil {
			return err
		}
	}
	return nil
}
func (s *sqliteStore) saveEvents(ctx context.Context, events []monitorEvent) error {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = insertEvents(ctx, tx, events); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *sqliteStore) loadEvents(ctx context.Context) ([]monitorEvent, error) {
	rows, err := s.reader.QueryContext(ctx, "SELECT body FROM (SELECT row_no,body FROM monitor_events ORDER BY row_no DESC LIMIT 500) ORDER BY row_no")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []monitorEvent{}
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		var event monitorEvent
		if err = json.Unmarshal(raw, &event); err != nil {
			return nil, err
		}
		out = append(out, event)
	}
	return out, rows.Err()
}
func (s *sqliteStore) ackEvents(ctx context.Context, id string) (bool, error) {
	query := `UPDATE monitor_events SET body=json_set(body,'$.acknowledged',json('true')) WHERE id=?`
	args := []any{id}
	if id == "all" {
		query = `UPDATE monitor_events SET body=json_set(body,'$.acknowledged',json('true'))`
		args = nil
	}
	result, err := s.writer.ExecContext(ctx, query, args...)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count > 0, err
}
