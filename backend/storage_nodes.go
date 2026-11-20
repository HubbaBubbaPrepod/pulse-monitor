package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

func (s *sqliteStore) saveNode(ctx context.Context, id string, n *agentNode) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	raw, err := json.Marshal(n)
	if err != nil {
		return err
	}
	_, err = s.writer.ExecContext(ctx, "INSERT INTO agent_nodes(id,body) VALUES(?,?) ON CONFLICT(id) DO UPDATE SET body=excluded.body", id, raw)
	return err
}
func (s *sqliteStore) loadNodes(ctx context.Context, servers []server) (map[string]*agentNode, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	out := map[string]*agentNode{}
	for _, server := range servers {
		var raw []byte
		err := s.reader.QueryRowContext(ctx, "SELECT body FROM agent_nodes WHERE id=?", server.ID).Scan(&raw)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, err
		}
		n := new(agentNode)
		if err = json.Unmarshal(raw, n); err != nil {
			return nil, err
		}
		n.Seen = time.Time{}
		n.Restored = true
		rows, err := s.reader.QueryContext(ctx, "SELECT body FROM metric_samples WHERE server_id=? AND at_ns>=? ORDER BY at_ns", server.ID, time.Now().Add(-24*time.Hour).UnixNano())
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var body []byte
			if err = rows.Scan(&body); err != nil {
				rows.Close()
				return nil, err
			}
			var point sample
			if err = json.Unmarshal(body, &point); err != nil {
				rows.Close()
				return nil, err
			}
			n.History = append(n.History, point)
		}
		if err = rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		if err = rows.Close(); err != nil {
			return nil, err
		}
		out[server.ID] = n
	}
	return out, nil
}
func (s *sqliteStore) commitReport(ctx context.Context, id string, n *agentNode, saveSample bool, logs []logEntry, events []monitorEvent) error {
	if err := s.ensureReportSpace(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = insertLogs(ctx, tx, logs, n.Seen); err != nil {
		return err
	}
	if saveSample {
		point := n.Report.Sample
		if len(n.History) > 0 {
			point = n.History[len(n.History)-1]
		}
		raw, err := json.Marshal(point)
		if err != nil {
			return err
		}
		stamp, err := time.Parse(time.RFC3339Nano, point.At)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO metric_samples(server_id,at_ns,body) VALUES(?,?,?) ON CONFLICT(server_id,at_ns) DO UPDATE SET body=excluded.body", id, stamp.UnixNano(), raw); err != nil {
			return err
		}
	}
	if err = insertEvents(ctx, tx, events); err != nil {
		return err
	}
	raw, err := json.Marshal(n)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO agent_nodes(id,body) VALUES(?,?) ON CONFLICT(id) DO UPDATE SET body=excluded.body", id, raw); err != nil {
		return err
	}
	return tx.Commit()
}
