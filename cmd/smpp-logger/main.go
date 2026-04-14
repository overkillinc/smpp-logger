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

	// start http UI
	httpSrv := &http.Server{Addr: cfg.UIAddr, Handler: ui.NewHandler(logger, cfg.UIUser, cfg.UIPass)}
	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintln(os.Stderr, "http server: ", err)
		}
	}()

	srv := server.New(cfg, logger)
	if err := srv.ListenAndServe(ctx); err != nil {
		// ensure http server stops
		_ = httpSrv.Shutdown(context.Background())
		fatal(err)
	}

	// shutdown http server gracefully
t_ := context.Background()
_ = httpSrv.Shutdown(t_)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
