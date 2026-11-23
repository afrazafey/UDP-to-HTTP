package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBoundedServiceLogs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "service.jsonl")
	w, err := NewRotatingWriter(path, 1024, 2)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		if _, err = w.Write([]byte(strings.Repeat("x", 512))); err != nil {
			t.Fatal(err)
		}
	}
	w.Close()
	files, err := filepath.Glob(path + "*")
	if err != nil || len(files) != 3 {
		t.Fatal(files, err)
	}
	for _, p := range files {
		info, err := os.Stat(p)
		if err != nil || info.Size() > 1024 {
			t.Fatal(info, err)
		}
	}
}
