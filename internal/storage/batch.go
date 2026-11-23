package storage

import (
	"context"
	"encoding/json"
	"github.com/udphttp-proxy/udphttp-proxy/internal/stream"
)

func (s *Store) ApplyChannels(ctx context.Context, upserts []stream.Channel, deletes []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, id := range deletes {
		if _, err = tx.ExecContext(ctx, "DELETE FROM channels WHERE id=?", id); err != nil {
			return err
		}
	}
	for _, c := range upserts {
		data, e := json.Marshal(c)
		if e != nil {
			return e
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO channels(id,configuration) VALUES(?,?) ON CONFLICT(id) DO UPDATE SET configuration=excluded.configuration,updated_at=strftime('%Y-%m-%dT%H:%M:%fZ','now')`, c.ID, string(data)); err != nil {
			return err
		}
	}
	return tx.Commit()
}
