package service

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Bounded JSON files are the Windows service logging strategy. No external
// wrapper or Event Log registration is needed. Journald remains native on Linux.
type RotatingWriter struct {
	mu       sync.Mutex
	path     string
	maxBytes int64
	copies   int
	file     *os.File
	size     int64
}

func NewRotatingWriter(path string, maxBytes int64, copies int) (*RotatingWriter, error) {
	if maxBytes < 1024 || copies < 1 || copies > 10 {
		return nil, fmt.Errorf("invalid log retention")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	w := &RotatingWriter{path: path, maxBytes: maxBytes, copies: copies}
	if err := w.open(); err != nil {
		return nil, err
	}
	return w, nil
}
func (w *RotatingWriter) open() error {
	f, e := os.OpenFile(w.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	info, e := f.Stat()
	if e != nil {
		f.Close()
		return e
	}
	w.file = f
	w.size = info.Size()
	return nil
}
func (w *RotatingWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return 0, os.ErrClosed
	}
	if int64(len(data)) > w.maxBytes {
		return 0, fmt.Errorf("log record exceeds file budget")
	}
	if w.size+int64(len(data)) > w.maxBytes {
		if e := w.file.Close(); e != nil {
			return 0, e
		}
		w.file = nil
		if e := os.Remove(fmt.Sprintf("%s.%d", w.path, w.copies)); e != nil && !os.IsNotExist(e) {
			return 0, e
		}
		for i := w.copies - 1; i >= 1; i-- {
			if e := os.Rename(fmt.Sprintf("%s.%d", w.path, i), fmt.Sprintf("%s.%d", w.path, i+1)); e != nil && !os.IsNotExist(e) {
				return 0, e
			}
		}
		if e := os.Rename(w.path, w.path+".1"); e != nil {
			return 0, e
		}
		if e := w.open(); e != nil {
			return 0, e
		}
	}
	n, e := w.file.Write(data)
	w.size += int64(n)
	return n, e
}
func (w *RotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}
