package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/deervery/raku-sika-hub/internal/app"
	"github.com/deervery/raku-sika-hub/internal/config"
	"github.com/deervery/raku-sika-hub/internal/printer/qlbackend"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	// CUPS runs the same binary as the backend for rakuql:// queues, through
	// the wrapper raku-sika-ops installs in /usr/lib/cups/backend.
	if len(os.Args) > 1 && os.Args[1] == "cups-backend" {
		os.Exit(qlbackend.Main(os.Args[2:]))
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	a, err := app.New(cfg, version, commit, buildDate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create app: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	go func() {
		<-ctx.Done()
		a.Stop()
	}()

	fmt.Println("raku-sika-hub running (press Ctrl+C to stop)")
	if err := a.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
