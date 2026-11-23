//go:build windows

package service

import (
	"context"
	"golang.org/x/sys/windows/svc"
	"io"
	"testing"
	"time"
)

func TestWindowsServiceGracefulStopHandler(t *testing.T) {
	h := &handler{logs: io.Discard, serve: func(ctx context.Context, _ []string, _ io.Writer) error { <-ctx.Done(); return nil }}
	requests := make(chan svc.ChangeRequest, 1)
	status := make(chan svc.Status, 8)
	done := make(chan uint32, 1)
	go func() { _, code := h.Execute(nil, requests, status); done <- code }()
	deadline := time.After(2 * time.Second)
	running := false
	for !running {
		select {
		case s := <-status:
			running = s.State == svc.Running
		case <-deadline:
			t.Fatal("service handler did not start")
		}
	}
	requests <- svc.ChangeRequest{Cmd: svc.Stop}
	select {
	case code := <-done:
		if code != 0 {
			t.Fatal(code)
		}
	case <-deadline:
		t.Fatal("service handler did not stop")
	}
}
