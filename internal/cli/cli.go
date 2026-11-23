package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/udphttp-proxy/udphttp-proxy/internal/buildinfo"
	"github.com/udphttp-proxy/udphttp-proxy/internal/config"
	"github.com/udphttp-proxy/udphttp-proxy/internal/network"
)

const usage = "Usage: udphttp-proxy serve [--config path] | version | config validate [--config path] | interfaces\n"

func Run(ctx context.Context, args []string, out, errOut io.Writer, getenv func(string) string) error {
	if len(args) == 0 {
		fmt.Fprint(out, usage)
		return errors.New("a command is required")
	}
	switch args[0] {
	case "help", "--help", "-h":
		fmt.Fprint(out, usage)
		return nil
	case "version":
		if len(args) != 1 {
			return errors.New("version takes no arguments")
		}
		fmt.Fprintf(out, "udphttp-proxy %s (commit %s, built %s)\n", buildinfo.Version, buildinfo.Commit, buildinfo.Date)
		return nil
	case "interfaces":
		if len(args) != 1 {
			return errors.New("interfaces takes no arguments")
		}
		items, err := network.Discover()
		if err != nil {
			return err
		}
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		return encoder.Encode(items)
	case "config":
		if len(args) < 2 || args[1] != "validate" {
			return errors.New("usage: udphttp-proxy config validate [--config path]")
		}
		_, err := loadFlags("config validate", args[2:], errOut, getenv)
		if err != nil {
			return err
		}
		fmt.Fprintln(out, "Configuration is valid (syntax and policy). Interface availability is checked by serve.")
		return nil
	case "serve":
		return serve(ctx, args[1:], errOut, getenv)
	default:
		return fmt.Errorf("unknown command %q; %s", args[0], usage)
	}
}
func loadFlags(name string, args []string, errOut io.Writer, getenv func(string) string) (config.Config, error) {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(errOut)
	defaultPath := ""
	if getenv != nil {
		defaultPath = getenv("UDPHTTP_CONFIG")
	}
	path := flags.String("config", defaultPath, "YAML configuration file (otherwise safe development defaults)")
	if err := flags.Parse(args); err != nil {
		return config.Config{}, err
	}
	if flags.NArg() != 0 {
		return config.Config{}, errors.New("unexpected positional arguments")
	}
	return config.Load(*path, getenv)
}

func Main(ctx context.Context, args []string) int {
	if err := Run(ctx, args, os.Stdout, os.Stderr, os.Getenv); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		slog.New(slog.NewJSONHandler(os.Stderr, nil)).Error("command failed", "error", err)
		return 1
	}
	return 0
}
