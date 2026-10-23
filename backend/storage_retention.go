package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"
)

func (s *sqliteStore) setRetention(ctx context.Context, id string, days int) error {
	if !validID.MatchString(id) || days < 0 || days > 3650 {
		return errors.New("invalid log retention")
	}
	_, err := s.writer.ExecContext(ctx, "INSERT INTO log_retention(server_id,days) VALUES(?,?) ON CONFLICT(server_id) DO UPDATE SET days=excluded.days", id, days)
	return err
}
func (s *sqliteStore) retentionPolicies(ctx context.Context) (map[string]int, error) {
	rows, err := s.reader.QueryContext(ctx, "SELECT server_id,days FROM log_retention")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var id string
		var days int
		if err = rows.Scan(&id, &days); err != nil {
			return nil, err
		}
		out[id] = days
	}
	return out, rows.Err()
}
func (s *sqliteStore) cleanupLogs(ctx context.Context, id string, now time.Time, batchSize int) (int64, error) {
	if batchSize < 1 || batchSize > 1000 {
		return 0, errors.New("invalid cleanup batch")
	}
	result, err := s.writer.ExecContext(ctx, `DELETE FROM collected_logs WHERE row_no IN (SELECT l.row_no FROM collected_logs l JOIN log_retention r ON r.server_id=l.server_id WHERE l.server_id=? AND r.days>0 AND l.received_ns < ? - r.days*86400000000000 ORDER BY l.received_ns LIMIT ?)`, id, now.UnixNano(), batchSize)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (s *sqliteStore) cleanupTelemetry(ctx context.Context, id string, now time.Time) error {
	// The UI restores only the latest 24h of samples and 500 notifications.
	// Bound each write and release the single writer between statements.
	_, err := s.writer.ExecContext(ctx, `DELETE FROM metric_samples WHERE (server_id,at_ns) IN (SELECT server_id,at_ns FROM metric_samples WHERE server_id=? AND at_ns<? ORDER BY at_ns LIMIT 1000)`, id, now.Add(-24*time.Hour).UnixNano())
	if err != nil {
		return err
	}
	_, err = s.writer.ExecContext(ctx, `DELETE FROM monitor_events WHERE row_no IN (SELECT row_no FROM monitor_events WHERE row_no<(SELECT row_no FROM monitor_events ORDER BY row_no DESC LIMIT 1 OFFSET 499) ORDER BY row_no LIMIT 1000)`)
	return err
}
func (a *app) retentionRoutes(mux *http.ServeMux) {
	mux.HandleFunc("PUT /api/servers/{id}/log-retention", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Days *int `json:"days"`
		}
		if err := decodeJSON(w, r, &input); err != nil || input.Days == nil || *input.Days < 0 || *input.Days > 3650 {
			apiError(w, 400, "Invalid log retention days")
			return
		}
		a.writeMu.Lock()
		defer a.writeMu.Unlock()
		id := r.PathValue("id")
		if _, ok := a.findServer(id); !ok {
			apiError(w, 404, "Server not found")
			return
		}
		if err := a.storage.setRetention(r.Context(), id, *input.Days); err != nil {
			apiError(w, 500, "Could not save log retention")
			return
		}
		a.mu.Lock()
		a.retentions[id] = *input.Days
		a.mu.Unlock()
		writeJSON(w, 200, map[string]int{"days": *input.Days})
	})
}
func (a *app) runLogRetention(ctx context.Context) error {
	timer := time.NewTicker(time.Minute)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case now := <-timer.C:
			for _, server := range a.serverList() {
				if ctx.Err() != nil {
					return nil
				}
				err := a.storage.cleanupServer(ctx, server.ID, now, 2*time.Second)
				if err != nil && ctx.Err() == nil {
					slog.Warn("retention cleanup failed", "server", server.ID, "error", err)
				}
			}
			stats, err := a.storage.checkpoint(ctx)
			if err != nil && ctx.Err() == nil {
				slog.Warn("SQLite checkpoint failed", "error", err)
			} else if stats.Bytes > 64*1024*1024 && stats.Frames > stats.Checkpointed {
				slog.Warn("SQLite WAL checkpoint backlog", "wal_bytes", stats.Bytes, "frames", stats.Frames, "checkpointed", stats.Checkpointed, "busy", stats.Busy)
			} else {
				slog.Debug("SQLite WAL checkpoint", "wal_bytes", stats.Bytes, "frames", stats.Frames, "checkpointed", stats.Checkpointed)
			}
		}
	}
}

// Keep separate budgets so a log backlog cannot starve sample/event retention.
func (s *sqliteStore) cleanupServer(ctx context.Context, id string, now time.Time, logBudget time.Duration) error {
	logCtx, cancel := context.WithTimeout(ctx, logBudget)
	var logErr error
	for logCtx.Err() == nil {
		var count int64
		count, logErr = s.cleanupLogs(logCtx, id, now, 1000)
		if logErr != nil || count < 1000 {
			break
		}
	}
	if logErr == nil {
		logErr = logCtx.Err()
	}
	cancel()
	telemetryCtx, telemetryCancel := context.WithTimeout(ctx, 2*time.Second)
	defer telemetryCancel()
	return errors.Join(logErr, s.cleanupTelemetry(telemetryCtx, id, now))
}
