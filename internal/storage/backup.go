package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
)

type BackupInfo struct {
	Schema   int    `json:"schema"`
	Channels int    `json:"channels"`
	SHA256   string `json:"sha256"`
	Bytes    int64  `json:"bytes"`
}

func reserveBackup(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	return os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
}

// Backup uses SQLite VACUUM INTO for a consistent snapshot including committed
// WAL contents, rather than copying a potentially inconsistent live main file.
func (s *Store) Backup(ctx context.Context, path string) (BackupInfo, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return BackupInfo{}, err
	}
	f, err := reserveBackup(abs)
	if err != nil {
		return BackupInfo{}, err
	}
	if err = f.Close(); err != nil {
		return BackupInfo{}, err
	}
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(abs)
		}
	}()
	if _, err = s.db.ExecContext(ctx, "VACUUM INTO ?", abs); err != nil {
		return BackupInfo{}, err
	}
	f, err = os.OpenFile(abs, os.O_RDWR, 0600)
	if err != nil {
		return BackupInfo{}, err
	}
	err = f.Sync()
	closeErr := f.Close()
	if err != nil {
		return BackupInfo{}, err
	}
	if closeErr != nil {
		return BackupInfo{}, closeErr
	}
	info, err := VerifyBackup(ctx, abs)
	if err != nil {
		return info, err
	}
	ok = true
	return info, nil
}
func VerifyBackup(ctx context.Context, path string) (BackupInfo, error) {
	var info BackupInfo
	abs, err := filepath.Abs(path)
	if err != nil {
		return info, err
	}
	stat, err := os.Lstat(abs)
	if err != nil {
		return info, err
	}
	if !stat.Mode().IsRegular() {
		return info, errors.New("backup must be a regular non-symlink file")
	}
	uriPath := filepath.ToSlash(abs)
	if filepath.VolumeName(abs) != "" {
		uriPath = "/" + uriPath
	}
	u := url.URL{Scheme: "file", Path: uriPath}
	q := url.Values{"mode": {"ro"}, "immutable": {"1"}}
	u.RawQuery = q.Encode()
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return info, err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	var check string
	if err = db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&check); err != nil {
		return info, err
	}
	if check != "ok" {
		return info, errors.New("SQLite integrity check failed")
	}
	if err = db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&info.Schema); err != nil {
		return info, err
	}
	if info.Schema < 1 || info.Schema > 4 {
		return info, errors.New("unsupported backup schema")
	}
	if info.Schema >= 2 {
		if err = db.QueryRowContext(ctx, "SELECT count(*) FROM channels").Scan(&info.Channels); err != nil {
			return info, err
		}
	}
	if info.Schema >= 3 {
		var count int
		if err = db.QueryRowContext(ctx, "SELECT count(*) FROM server_settings").Scan(&count); err != nil {
			return info, err
		}
	}
	f, err := os.Open(abs)
	if err != nil {
		return info, err
	}
	defer f.Close()
	h := sha256.New()
	info.Bytes, err = io.Copy(h, f)
	if err != nil {
		return info, err
	}
	info.SHA256 = hex.EncodeToString(h.Sum(nil))
	return info, nil
}

// Restore writes a new destination only; callers stop the service and explicitly
// select its restored database afterward. Existing DBs/WALs are never replaced.
func RestoreBackup(ctx context.Context, input, output string) (BackupInfo, error) {
	info, err := VerifyBackup(ctx, input)
	if err != nil {
		return info, err
	}
	in, err := os.Open(input)
	if err != nil {
		return info, err
	}
	defer in.Close()
	out, err := reserveBackup(output)
	if err != nil {
		return info, err
	}
	ok := false
	defer func() {
		out.Close()
		if !ok {
			_ = os.Remove(output)
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return info, err
	}
	if err = out.Sync(); err != nil {
		return info, err
	}
	if err = out.Close(); err != nil {
		return info, err
	}
	restored, err := VerifyBackup(ctx, output)
	if err != nil {
		return info, err
	}
	if restored.SHA256 != info.SHA256 {
		return info, fmt.Errorf("restored checksum differs from verified backup")
	}
	ok = true
	return restored, nil
}
