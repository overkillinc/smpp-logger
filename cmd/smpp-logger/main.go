package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/overkillinc/smpp-logger/internal/config"
	"github.com/overkillinc/smpp-logger/internal/logging"
	"github.com/overkillinc/smpp-logger/internal/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fatal(err)
	}

	logger, err := logging.New(os.Stdout, cfg.LogFormat)
	if err != nil {
		fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv := server.New(cfg, logger)
	if err := srv.ListenAndServe(ctx); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
