package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"github.com/udphttp-proxy/udphttp-proxy/internal/config"
	"github.com/udphttp-proxy/udphttp-proxy/internal/storage"
	"io"
)

func database(ctx context.Context, args []string, out, errOut io.Writer, getenv func(string) string) error {
	if len(args) == 0 {
		return errors.New("database requires backup, verify, or restore")
	}
	f := flag.NewFlagSet("database", flag.ContinueOnError)
	f.SetOutput(errOut)
	defaultPath := ""
	if getenv != nil {
		defaultPath = getenv("UDPHTTP_CONFIG")
	}
	path := f.String("config", defaultPath, "configuration for backup source")
	input := f.String("input", "", "backup to verify or restore")
	output := f.String("output", "", "new backup/restored database path (must not exist)")
	confirmed := f.Bool("confirm-offline", false, "confirm service is stopped for restoring to a new destination")
	if err := f.Parse(args[1:]); err != nil {
		return err
	}
	if f.NArg() != 0 {
		return errors.New("unexpected database arguments")
	}
	var info storage.BackupInfo
	var err error
	switch args[0] {
	case "backup":
		if *output == "" {
			return errors.New("backup requires --output")
		}
		cfg, e := config.Load(*path, getenv)
		if e != nil {
			return e
		}
		s, e := storage.OpenForBackup(ctx, cfg.Storage.Path)
		if e != nil {
			return e
		}
		defer s.Close()
		info, err = s.Backup(ctx, *output)
	case "verify":
		if *input == "" {
			return errors.New("verify requires --input")
		}
		info, err = storage.VerifyBackup(ctx, *input)
	case "restore":
		if !*confirmed || *input == "" || *output == "" {
			return errors.New("restore requires --input, --output, and --confirm-offline; existing destinations are never overwritten")
		}
		info, err = storage.RestoreBackup(ctx, *input, *output)
	default:
		return errors.New("unknown database action")
	}
	if err != nil {
		return err
	}
	return json.NewEncoder(out).Encode(info)
}
