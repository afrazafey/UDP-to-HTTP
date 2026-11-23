package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/udphttp-proxy/udphttp-proxy/internal/stream"

	_ "modernc.org/sqlite"
)

type Store struct{ db *sql.DB }

func Open(ctx context.Context, path string) (*Store, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(filepath.Dir(abs), 0700); err != nil {
		return nil, err
	}
	if info, e := os.Lstat(abs); e == nil && (!info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0) {
		return nil, errors.New("database must be a regular file, not a symlink")
	}
	f, err := os.OpenFile(abs, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = f.Close(); err != nil {
		return nil, err
	}
	if err = os.Chmod(abs, 0600); err != nil {
		return nil, err
	}
	// Construct the URI, never interpolate user input as SQLite query options.
	uriPath := filepath.ToSlash(abs)
	if filepath.VolumeName(abs) != "" {
		uriPath = "/" + uriPath
	}
	u := url.URL{Scheme: "file", Path: uriPath}
	q := url.Values{}
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "foreign_keys(1)")
	u.RawQuery = q.Encode()
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err = s.migrate(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("initialize SQLite: %w", err)
	}
	return s, nil
}
func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, "PRAGMA journal_mode=WAL"); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var version int
	if err = tx.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if version > 3 {
		return fmt.Errorf("database schema %d is newer than supported schema 3", version)
	}
	if version == 0 {
		if _, err = tx.ExecContext(ctx, `CREATE TABLE settings (key TEXT PRIMARY KEY, value TEXT NOT NULL CHECK(json_valid(value)), updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))); PRAGMA user_version=1;`); err != nil {
			return err
		}
	}
	if version < 2 {
		if _, err = tx.ExecContext(ctx, `CREATE TABLE channels (id TEXT PRIMARY KEY, configuration TEXT NOT NULL CHECK(json_valid(configuration)), updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))); PRAGMA user_version=2;`); err != nil {
			return err
		}
	}
	if version < 3 {
		if _, err = tx.ExecContext(ctx, `CREATE TABLE server_settings (id INTEGER PRIMARY KEY CHECK(id=1), revision INTEGER NOT NULL, accepted_revision INTEGER NOT NULL, desired TEXT NOT NULL CHECK(json_valid(desired)), accepted TEXT NOT NULL CHECK(json_valid(accepted)), last_error TEXT NOT NULL DEFAULT ''); CREATE TABLE operational_events (id INTEGER PRIMARY KEY AUTOINCREMENT, time TEXT NOT NULL, level TEXT NOT NULL, component TEXT NOT NULL, stream_id TEXT NOT NULL, message TEXT NOT NULL, context TEXT NOT NULL CHECK(json_valid(context))); CREATE INDEX events_time ON operational_events(time); CREATE INDEX events_stream ON operational_events(stream_id,id); PRAGMA user_version=3;`); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) ListChannels(ctx context.Context) ([]stream.Channel, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT configuration FROM channels ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []stream.Channel{}
	for rows.Next() {
		var data string
		if err = rows.Scan(&data); err != nil {
			return nil, err
		}
		var c stream.Channel
		if err = json.Unmarshal([]byte(data), &c); err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, rows.Err()
}
func (s *Store) SaveChannel(ctx context.Context, c stream.Channel, create bool) error {
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	query := "UPDATE channels SET configuration=?,updated_at=strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE id=?"
	if create {
		query = "INSERT INTO channels(configuration,id) VALUES(?,?)"
	}
	result, err := s.db.ExecContext(ctx, query, string(data), c.ID)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n == 0 {
		return stream.ErrNotFound
	}
	return err
}
func (s *Store) DeleteChannel(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM channels WHERE id=?", id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n == 0 {
		return stream.ErrNotFound
	}
	return err
}

// SeedChannels imports YAML only once. Deleted channels never reappear on restart.
func (s *Store) SeedChannels(ctx context.Context, channels []stream.Channel) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var marker string
	err = tx.QueryRowContext(ctx, "SELECT value FROM settings WHERE key='channels_seeded'").Scan(&marker)
	if err == nil {
		return tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	for _, c := range channels {
		data, e := json.Marshal(c)
		if e != nil {
			return e
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO channels(id,configuration) VALUES(?,?) ON CONFLICT(id) DO NOTHING", c.ID, string(data)); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO settings(key,value) VALUES('channels_seeded','true')"); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }
func (s *Store) Close() error                   { return s.db.Close() }
func (s *Store) Put(ctx context.Context, key string, value string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=strftime('%Y-%m-%dT%H:%M:%fZ','now')`, key, value)
	return err
}
func (s *Store) Get(ctx context.Context, key string) (string, error) {
	var v string
	err := s.db.QueryRowContext(ctx, "SELECT value FROM settings WHERE key=?", key).Scan(&v)
	return v, err
}
