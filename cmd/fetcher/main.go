// Command fetcher downloads job postings from configured sources
// and stores them in PostgreSQL. It is designed to run once per
// invocation, typically on a schedule.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/mykola-petrychenko/jobradar/internal/arbeitnow"
	"github.com/mykola-petrychenko/jobradar/internal/fetch"
	"github.com/mykola-petrychenko/jobradar/internal/httpclient"
	"github.com/mykola-petrychenko/jobradar/internal/postgres"
)

func main() {
	if err := loadEnvFile(); err != nil {
		fmt.Fprintln(os.Stderr, "load .env:", err)
		os.Exit(1)
	}

	logger := newLogger()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, logger); err != nil {
		logger.Error("fetch failed", "err", err)
		stop()
		os.Exit(1)
	}
}

func run(ctx context.Context, logger *slog.Logger) error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	connectCtx, cancel := context.WithTimeout(ctx, cfg.dbConnectTimeout)
	defer cancel()

	store, err := postgres.New(connectCtx, cfg.databaseURL)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer store.Close()

	client := httpclient.New(logger, httpclient.Options{DumpDir: cfg.debugDumpDir})
	source := arbeitnow.New(logger, client)
	fetcher := fetch.New(logger, source, store, fetch.Options{})

	res, runErr := fetcher.Run(ctx)

	switch {
	case runErr == nil:
		logger.Info("fetch finished", "result", res)
		return nil
	case errors.Is(runErr, context.Canceled):
		logger.Info("fetch interrupted by shutdown signal", "result", res)
		return nil
	default:
		return fmt.Errorf("fetch aborted after %d pages: %w", res.Pages, runErr)
	}
}
