//go:build windows

package service

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/udphttp-proxy/udphttp-proxy/internal/config"
	"github.com/udphttp-proxy/udphttp-proxy/internal/events"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
	"io"
	"os"
	"path/filepath"
	"time"
)

const Name = "UDPHTTPProxy"

type handler struct {
	args  []string
	logs  io.Writer
	serve func(context.Context, []string, io.Writer) error
}

func (h *handler) Execute(_ []string, requests <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	status <- svc.Status{State: svc.StartPending, WaitHint: 15000}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- h.serve(ctx, h.args, h.logs) }()
	current := svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	status <- current
	for {
		select {
		case err := <-done:
			if err != nil {
				_ = json.NewEncoder(h.logs).Encode(map[string]any{"time": time.Now().UTC(), "level": "ERROR", "msg": "service stopped with an error", "component": "service", "event_type": "service_failure", "error_category": "startup_or_runtime", "error": events.Redact(err.Error())})
				return true, 1
			}
			return false, 0
		case change, ok := <-requests:
			if !ok {
				cancel()
				return true, 2
			}
			switch change.Cmd {
			case svc.Interrogate:
				status <- current
			case svc.Stop, svc.Shutdown:
				cancel()
				current = svc.Status{State: svc.StopPending, WaitHint: 30000}
				status <- current
				ticker := time.NewTicker(time.Second)
				timer := time.NewTimer(35 * time.Second)
				defer ticker.Stop()
				defer timer.Stop()
				for {
					select {
					case err := <-done:
						if err != nil {
							return true, 1
						}
						return false, 0
					case <-ticker.C:
						current.CheckPoint++
						status <- current
					case <-timer.C:
						return true, 3
					}
				}
			}
		}
	}
}
func waitState(ctx context.Context, s *mgr.Service, want svc.State) error {
	timer := time.NewTimer(40 * time.Second)
	defer timer.Stop()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		state, err := s.Query()
		if err != nil {
			return err
		}
		if state.State == want {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return errors.New("service transition timed out")
		case <-ticker.C:
		}
	}
}
func Command(ctx context.Context, args []string, out io.Writer, serve func(context.Context, []string, io.Writer) error) error {
	if len(args) == 0 {
		return errors.New("service requires install, run, start, stop, restart, status, or uninstall")
	}
	root := filepath.Join(os.Getenv("ProgramData"), "UDPHTTPProxy")
	if !filepath.IsAbs(root) {
		return errors.New("ProgramData must be absolute")
	}
	f := flag.NewFlagSet("service", flag.ContinueOnError)
	f.SetOutput(out)
	path := f.String("config", filepath.Join(root, "config.yaml"), "absolute YAML path")
	logs := f.String("log-dir", filepath.Join(root, "logs"), "absolute service log directory")
	if err := f.Parse(args[1:]); err != nil {
		return err
	}
	if f.NArg() != 0 {
		return errors.New("unexpected service arguments")
	}
	if !filepath.IsAbs(*path) || !filepath.IsAbs(*logs) {
		return errors.New("service paths must be absolute")
	}
	if args[0] == "run" {
		isService, err := svc.IsWindowsService()
		if err != nil {
			return err
		}
		if !isService {
			return errors.New("service run must be launched by the Windows Service Control Manager")
		}
		writer, err := NewRotatingWriter(filepath.Join(*logs, "service.jsonl"), 20<<20, 3)
		if err != nil {
			return err
		}
		defer writer.Close()
		return svc.Run(Name, &handler{args: []string{"--config", *path}, logs: writer, serve: serve})
	}
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	if args[0] == "install" {
		if _, err = config.Load(*path, os.Getenv); err != nil {
			return err
		}
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		s, err := m.CreateService(Name, exe, mgr.Config{DisplayName: "UDPHTTP Proxy", Description: "Shared multicast/unicast MPEG-TS to HTTP proxy; local management", StartType: mgr.StartAutomatic, ServiceStartName: `NT AUTHORITY\LocalService`}, "service", "run", "--config", *path, "--log-dir", *logs)
		if err != nil {
			return err
		}
		defer s.Close()
		if err = s.SetRecoveryActions([]mgr.RecoveryAction{{Type: mgr.ServiceRestart, Delay: 5 * time.Second}, {Type: mgr.ServiceRestart, Delay: 30 * time.Second}, {Type: mgr.NoAction}}, 3600); err != nil {
			_ = s.Delete() // Roll back only the registration created by this call.
			return err
		}
		if err = s.SetRecoveryActionsOnNonCrashFailures(true); err != nil {
			_ = s.Delete()
			return err
		}
		fmt.Fprintln(out, "Service installed with automatic startup; use service start")
		return nil
	}
	s, err := m.OpenService(Name)
	if err != nil {
		return err
	}
	defer s.Close()
	switch args[0] {
	case "status":
		status, err := s.Query()
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "state=%d win32_exit=%d service_exit=%d\n", status.State, status.Win32ExitCode, status.ServiceSpecificExitCode)
		return nil
	case "start":
		if err = s.Start(); err != nil {
			return err
		}
		return waitState(ctx, s, svc.Running)
	case "stop", "restart":
		status, err := s.Query()
		if err != nil {
			return err
		}
		if status.State != svc.Stopped {
			if _, err = s.Control(svc.Stop); err != nil {
				return err
			}
			if err = waitState(ctx, s, svc.Stopped); err != nil {
				return err
			}
		}
		if args[0] == "restart" {
			if err = s.Start(); err != nil {
				return err
			}
			return waitState(ctx, s, svc.Running)
		}
		return nil
	case "uninstall":
		status, err := s.Query()
		if err != nil {
			return err
		}
		if status.State != svc.Stopped {
			return errors.New("stop service before uninstall; configuration and data are preserved")
		}
		return s.Delete()
	default:
		return errors.New("unknown service command")
	}
}
