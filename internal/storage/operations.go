package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/udphttp-proxy/udphttp-proxy/internal/events"
	"github.com/udphttp-proxy/udphttp-proxy/internal/operations"
)

func (s *Store) InitServerSettings(ctx context.Context, value operations.Settings) error {
	data := operations.Encode(value)
	_, err := s.db.ExecContext(ctx, `INSERT INTO server_settings(id,revision,accepted_revision,desired,accepted) VALUES(1,0,0,?,?) ON CONFLICT(id) DO UPDATE SET desired=CASE WHEN revision=0 THEN excluded.desired ELSE desired END,accepted=CASE WHEN revision=0 THEN excluded.accepted ELSE accepted END`, data, data)
	return err
}
func (s *Store) ReadServerSettings(ctx context.Context) (operations.Record, error) {
	var r operations.Record
	var desired, accepted string
	err := s.db.QueryRowContext(ctx, "SELECT revision,accepted_revision,desired,accepted,last_error FROM server_settings WHERE id=1").Scan(&r.Revision, &r.AcceptedRevision, &desired, &accepted, &r.LastError)
	if err != nil {
		return r, err
	}
	if err = json.Unmarshal([]byte(desired), &r.Desired); err != nil {
		return r, err
	}
	err = json.Unmarshal([]byte(accepted), &r.Accepted)
	return r, err
}
func (s *Store) WriteServerSettings(ctx context.Context, revision int64, value operations.Settings) (operations.Record, error) {
	result, err := s.db.ExecContext(ctx, "UPDATE server_settings SET desired=?, revision=revision+1,last_error='' WHERE id=1 AND revision=?", operations.Encode(value), revision)
	if err != nil {
		return operations.Record{}, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return operations.Record{}, err
	}
	if n == 0 {
		return operations.Record{}, operations.ErrRevision
	}
	return s.ReadServerSettings(ctx)
}
func (s *Store) AcceptServerSettings(ctx context.Context, revision int64) error {
	result, err := s.db.ExecContext(ctx, "UPDATE server_settings SET accepted=desired,accepted_revision=revision,last_error='' WHERE id=1 AND revision=?", revision)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		return operations.ErrRevision
	}
	return nil
}
func (s *Store) RollbackServerSettings(ctx context.Context, message string) (operations.Record, error) {
	_, err := s.db.ExecContext(ctx, "UPDATE server_settings SET desired=accepted,revision=revision+1,accepted_revision=revision+1,last_error=? WHERE id=1", events.Redact(message))
	if err != nil {
		return operations.Record{}, err
	}
	return s.ReadServerSettings(ctx)
}
func (s *Store) AppendEvent(ctx context.Context, e events.Entry, count, days int) error {
	data, err := json.Marshal(e.Context)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "INSERT INTO operational_events(time,level,component,stream_id,message,context) VALUES(?,?,?,?,?,?)", e.Time.UTC().Format(time.RFC3339Nano), e.Level, e.Component, e.StreamID, e.Message, string(data)); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM operational_events WHERE id NOT IN (SELECT id FROM operational_events ORDER BY id DESC LIMIT ?) OR time < ?`, count, time.Now().UTC().AddDate(0, 0, -days).Format(time.RFC3339Nano)); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) QueryEvents(ctx context.Context, f events.Filter) ([]events.Entry, int, error) {
	clauses := []string{"1=1"}
	args := []any{}
	if f.Search != "" {
		clauses = append(clauses, "(instr(lower(message),lower(?))>0 OR instr(lower(context),lower(?))>0)")
		args = append(args, f.Search, f.Search)
	}
	if f.Level != "" {
		clauses = append(clauses, "level=?")
		args = append(args, f.Level)
	}
	if f.StreamID != "" {
		clauses = append(clauses, "stream_id=?")
		args = append(args, f.StreamID)
	}
	if !f.Since.IsZero() {
		clauses = append(clauses, "time>=?")
		args = append(args, f.Since.UTC().Format(time.RFC3339Nano))
	}
	if !f.Until.IsZero() {
		clauses = append(clauses, "time<=?")
		args = append(args, f.Until.UTC().Format(time.RFC3339Nano))
	}
	where := strings.Join(clauses, " AND ")
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT count(*) FROM operational_events WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if f.Limit < 1 || f.Limit > 1000 {
		return nil, 0, fmt.Errorf("invalid event limit")
	}
	if f.Offset < 0 {
		return nil, 0, fmt.Errorf("invalid offset")
	}
	args = append(args, f.Limit, f.Offset)
	rows, err := s.db.QueryContext(ctx, "SELECT id,time,level,component,stream_id,message,context FROM operational_events WHERE "+where+" ORDER BY id DESC LIMIT ? OFFSET ?", args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []events.Entry{}
	for rows.Next() {
		var e events.Entry
		var timestamp, data string
		if err = rows.Scan(&e.ID, &timestamp, &e.Level, &e.Component, &e.StreamID, &e.Message, &data); err != nil {
			return nil, 0, err
		}
		e.Time, err = time.Parse(time.RFC3339Nano, timestamp)
		if err != nil {
			return nil, 0, err
		}
		if err = json.Unmarshal([]byte(data), &e.Context); err != nil {
			return nil, 0, err
		}
		items = append(items, e)
	}
	return items, total, rows.Err()
}
