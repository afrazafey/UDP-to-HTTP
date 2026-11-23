package storage

import (
	"context"
	"database/sql"
	"errors"
	"github.com/udphttp-proxy/udphttp-proxy/internal/stream"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPersistenceAndAtomicUpsert(t *testing.T) {
	ctx := context.Background()
	filename := "proxy #.db"
	if runtime.GOOS != "windows" {
		filename = "proxy ?#.db"
	}
	path := filepath.Join(t.TempDir(), "state", filename)
	s, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Put(ctx, "theme", `{"dark":true}`); err != nil {
		t.Fatal(err)
	}
	if err = s.Put(ctx, "theme", `{"dark":false}`); err != nil {
		t.Fatal(err)
	}
	if err = s.Put(ctx, "theme", `invalid json`); err == nil {
		t.Fatal("invalid JSON accepted")
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	value, err := s.Get(ctx, "theme")
	if err != nil || value != `{"dark":false}` {
		t.Fatalf("value %s error %v", value, err)
	}
	if _, err = s.Get(ctx, "missing"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0600 {
			t.Fatal("database permissions are not private")
		}
	}
}

func TestPhase1MigrationAndChannelPersistence(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "upgrade.db")
	// Construct the actual old schema, not the new Open path.
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE settings(key TEXT PRIMARY KEY,value TEXT NOT NULL CHECK(json_valid(value)),updated_at TEXT NOT NULL DEFAULT 'old'); INSERT INTO settings(key,value) VALUES('phase1','{"preserved":true}'); PRAGMA user_version=1;`)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	s, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	value, err := s.Get(ctx, "phase1")
	if err != nil || value != `{"preserved":true}` {
		t.Fatal("Phase 1 data lost", err)
	}
	c := stream.Channel{ID: "news", Name: "News", SourceType: "multicast", SourceAddress: "239.1.1.1", SourcePort: 1234, Interface: "eth0", Enabled: true}
	if err = s.SeedChannels(ctx, []stream.Channel{c}); err != nil {
		t.Fatal(err)
	}
	items, err := s.ListChannels(ctx)
	if err != nil || len(items) != 1 {
		t.Fatal(items, err)
	}
	c.Name = "Updated"
	if err = s.SaveChannel(ctx, c, false); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	items, err = s.ListChannels(ctx)
	if err != nil || len(items) != 1 || items[0].Name != "Updated" {
		t.Fatal("channel not persisted", items, err)
	}
	if err = s.DeleteChannel(ctx, c.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.SeedChannels(ctx, []stream.Channel{c}); err != nil {
		t.Fatal(err)
	}
	items, err = s.ListChannels(ctx)
	if err != nil || len(items) != 0 {
		t.Fatal("deleted seed reappeared", items, err)
	}
}
func TestFutureSchemaRejected(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "proxy.db")
	s, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.ExecContext(ctx, "PRAGMA user_version=99"); err != nil {
		t.Fatal(err)
	}
	s.Close()
	if next, err := Open(ctx, path); err == nil {
		next.Close()
		t.Fatal("newer schema accepted")
	}
}
func TestCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if s, err := Open(ctx, filepath.Join(t.TempDir(), "proxy.db")); err == nil {
		s.Close()
		t.Fatal("canceled context ignored")
	}
}
