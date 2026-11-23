package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/udphttp-proxy/udphttp-proxy/internal/stream"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestV3MetadataMigrationPreservesRecordsAndFailureRollback(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "upgrade.db")
	s, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	old := `{"id":"trace-urban","name":"TRACE URBAN","source_type":"multicast","source_address":"239.1.1.61","source_port":1234,"interface":"ens18","enabled":true}`
	if _, err = s.db.Exec(`INSERT INTO channels(id,configuration) VALUES('trace-urban',?); INSERT INTO settings(key,value) VALUES('preserve','{"safe":true}'); PRAGMA user_version=3;`, old); err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec(`CREATE TRIGGER migration_failure BEFORE UPDATE ON channels BEGIN SELECT RAISE(ABORT,'simulated migration failure'); END;`); err != nil {
		t.Fatal(err)
	}
	s.Close()
	if migrated, err := Open(ctx, path); err == nil {
		migrated.Close()
		t.Fatal("migration unexpectedly succeeded")
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	var version int
	db.QueryRow("PRAGMA user_version").Scan(&version)
	var stored string
	db.QueryRow("SELECT configuration FROM channels").Scan(&stored)
	if version != 3 || stored != old {
		t.Fatal("failed migration changed existing records", version, stored)
	}
	if _, err = db.Exec("DROP TRIGGER migration_failure"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	s, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	items, err := s.ListChannels(ctx)
	if err != nil || len(items) != 1 || items[0].Name != "TRACE URBAN" || items[0].PreferredPlaybackPort != "primary" || items[0].SourceAddress != "239.1.1.61" {
		t.Fatal(items, err)
	}
	value, err := s.Get(ctx, "preserve")
	if err != nil || value != `{"safe":true}` {
		t.Fatal("settings lost", value, err)
	}
}

func TestWALBusyTimeoutAndConcurrentWriter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "locks.db")
	ctx := context.Background()
	a, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	var mode string
	var timeout int
	if err = a.db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil || mode != "wal" {
		t.Fatal(mode, err)
	}
	if err = b.db.QueryRow("PRAGMA busy_timeout").Scan(&timeout); err != nil || timeout != 5000 {
		t.Fatal(timeout, err)
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`INSERT INTO settings(key,value) VALUES('lock','true')`); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- b.Put(ctx, "writer", "true") }()
	select {
	case err := <-done:
		t.Fatal("writer failed to wait for lock", err)
	case <-time.After(20 * time.Millisecond):
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(6 * time.Second):
		t.Fatal("writer did not recover from contention")
	}
}
func TestAtomicChannelBatchRollback(t *testing.T) {
	s, err := Open(context.Background(), filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err = s.db.Exec(`CREATE TRIGGER reject_batch BEFORE INSERT ON channels WHEN NEW.id='reject' BEGIN SELECT RAISE(ABORT,'rejected'); END;`); err != nil {
		t.Fatal(err)
	}
	if err = s.ApplyChannels(context.Background(), []stream.Channel{{ID: "good", Name: "Good"}, {ID: "reject", Name: "Reject"}}, nil); err == nil {
		t.Fatal("failed batch accepted")
	}
	items, err := s.ListChannels(context.Background())
	if err != nil || len(items) != 0 {
		t.Fatal("partial import persisted", items, err)
	}
}
func TestOnlineBackupVerifyRestoreAndNoOverwrite(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	s, err := Open(ctx, filepath.Join(root, "live.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	c := stream.Channel{ID: "trace", Name: "TRACE", Group: "MUSIC", TVGID: "67"}
	if err = s.SaveChannel(ctx, c, true); err != nil {
		t.Fatal(err)
	}
	if err = s.Put(ctx, "preserve", `{"answer":42}`); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(root, "backup.db")
	info, err := s.Backup(ctx, backup)
	if err != nil || info.Schema != 4 || info.Channels != 1 || len(info.SHA256) != 64 {
		t.Fatal(info, err)
	}
	if _, err = s.Backup(ctx, backup); err == nil {
		t.Fatal("existing backup overwritten")
	}
	restored := filepath.Join(root, "restored.db")
	other, err := RestoreBackup(ctx, backup, restored)
	if err != nil || other.SHA256 != info.SHA256 {
		t.Fatal(other, err)
	}
	if _, err = RestoreBackup(ctx, backup, restored); err == nil {
		t.Fatal("existing restore destination overwritten")
	}
	reopened, err := Open(ctx, restored)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	items, err := reopened.ListChannels(ctx)
	if err != nil || len(items) != 1 || items[0].TVGID != "67" {
		t.Fatal(items, err)
	}
	value, _ := reopened.Get(ctx, "preserve")
	if value != `{"answer":42}` {
		t.Fatal(value)
	}
	bad := filepath.Join(root, "bad.db")
	os.WriteFile(bad, []byte("not sqlite"), 0600)
	if _, err = VerifyBackup(ctx, bad); err == nil {
		t.Fatal("corrupt backup accepted")
	}
}
func TestBackupDoesNotMigrateSource(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "v3.db")
	s, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.db.Exec("PRAGMA user_version=3")
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = OpenForBackup(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	info, err := s.Backup(context.Background(), filepath.Join(root, "old-backup.db"))
	if err != nil || info.Schema != 3 {
		data, _ := json.Marshal(info)
		t.Fatal(string(data), err)
	}
}
