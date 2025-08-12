package main

import (
	"context"
	"errors"
	"github.com/dns4acme/dns4acme"
	"github.com/dns4acme/dns4acme/internal/config"
	"github.com/dns4acme/dns4acme/lang/E"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	cfg := dns4acme.NewConfig()
	configParser := config.New(cfg)
	if len(os.Args) == 2 && (os.Args[1] == "-h" || os.Args[1] == "--help") {
		_, _ = os.Stdout.Write([]byte("Usage: ./dns4acme [OPTIONS]\n\nOptions:\n"))
		_, _ = os.Stdout.Write(configParser.CLIHelp())
		os.Exit(0)
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := configParser.ApplyDefaults(); err != nil {
		fatal(logger, err)
	}
	if err := configParser.ApplyEnv("DNS4ACME_", os.Environ()); err != nil {
		fatal(logger, err)
	}
	if err := configParser.ApplyCMD(os.Args); err != nil {
		fatal(logger, err)
	}
	ctx := context.Background()
	srv, err := dns4acme.New(ctx, cfg, os.Stdout)
	if err != nil {
		fatal(logger, err)
	}
	runningSrv, err := srv.Start(ctx)
	if err != nil {
		fatal(logger, err)
	}
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-done
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := runningSrv.Stop(ctx); err != nil {
		fatal(logger, err)
	}
}

func fatal(logger *slog.Logger, err error) {
	var typedErr E.Error
	var attrs []any
	if errors.As(err, &typedErr) {
		attrs = typedErr.GetAttrs().AnySlice()
	}
	logger.Error(err.Error(), attrs...)
	os.Exit(1)
}
