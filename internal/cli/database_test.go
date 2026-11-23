package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/udphttp-proxy/udphttp-proxy/internal/storage"
	"os"
	"path/filepath"
	"testing"
)

func TestDatabaseBackupCommands(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "live.db")
	s, err := storage.Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	s.Put(t.Context(), "preserve", `{"ok":true}`)
	s.Close()
	config := filepath.Join(root, "config.yaml")
	body, _ := json.Marshal(map[string]any{"storage": map[string]any{"path": path}})
	os.WriteFile(config, body, 0600)
	backup := filepath.Join(root, "backup.db")
	restored := filepath.Join(root, "restored.db")
	for _, args := range [][]string{{"database", "backup", "--config", config, "--output", backup}, {"database", "verify", "--input", backup}, {"database", "restore", "--input", backup, "--output", restored, "--confirm-offline"}} {
		var out, errOut bytes.Buffer
		if err := Run(context.Background(), args, &out, &errOut, nil); err != nil || !json.Valid(out.Bytes()) {
			t.Fatal(args, out.String(), err)
		}
	}
	var out, errOut bytes.Buffer
	if err := Run(t.Context(), []string{"database", "restore", "--input", backup, "--output", filepath.Join(root, "unconfirmed.db")}, &out, &errOut, nil); err == nil {
		t.Fatal("unconfirmed restore accepted")
	}
}
