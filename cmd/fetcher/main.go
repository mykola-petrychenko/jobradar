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

	"github.com/mykola-petrychenko/jobradar/internal/fetch"
	"github.com/mykola-petrychenko/jobradar/internal/httpclient"
	"github.com/mykola-petrychenko/jobradar/internal/postgres"
	"github.com/mykola-petrychenko/jobradar/internal/source/arbeitnow"
	"github.com/mykola-petrychenko/jobradar/internal/source/arbeitsagentur"
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

	sources := []fetch.Source{
		arbeitnow.New(logger, httpclient.New(logger, httpclient.Options{
			DumpDir: cfg.debugDumpDir,
		})),
		arbeitsagentur.New(logger, httpclient.New(logger, httpclient.Options{
			DumpDir: cfg.debugDumpDir,
			Headers: arbeitsagentur.Headers(),
		})),
	}

	var errs []error
	for _, src := range sources {
		fetcher := fetch.New(logger, src, store, fetch.Options{})

		res, runErr := fetcher.Run(ctx)
		switch {
		case runErr == nil:
			logger.Info("fetch finished", "result", res)
		case errors.Is(runErr, context.Canceled):
			logger.Info("fetch interrupted by shutdown signal", "result", res)
			return errors.Join(errs...)
		default:
			errs = append(errs, fmt.Errorf("%s: fetch aborted after %d pages: %w",
				src.Name(), res.Pages, runErr))
		}
	}
	return errors.Join(errs...)
}
