package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/udphttp-proxy/udphttp-proxy/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := cli.Main(ctx, os.Args[1:])
	stop()
	os.Exit(code)
}
