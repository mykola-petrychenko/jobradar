// Command fetcher collects job postings and stores them in PostgreSQL.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/mykola-petrychenko/jobradar/internal/arbeitnow"
	"github.com/mykola-petrychenko/jobradar/internal/postgres"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	if err := run(ctx, logger); err != nil {
		logger.Error("run failed", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, logger *slog.Logger) error {
	godotenv.Load()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return errors.New("DATABASE_URL is not set")
	}

	connectCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	store, err := postgres.New(connectCtx, dsn)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer store.Close()

	logger.Info("database connected")

	since, err := store.LatestCreatedAt(ctx, "arbeitnow")
	if err != nil {
		return fmt.Errorf("latest created_at: %w", err)
	}
	monthAgo := time.Now().AddDate(0, 0, -3).Unix()
	if since < monthAgo {
		since = monthAgo
	}

	client := arbeitnow.New()

	logger.Info("fetch started", "source", "arbeitnow")
	start := time.Now()

	postings, err := client.Fetch(ctx, since)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		return fmt.Errorf("fetch arbeitnow: %w", err)
	}

	inserted := 0
	for _, p := range postings {
		ok, err := store.Insert(ctx, p)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				logger.Info("run interrupted", "inserted_so_far", inserted)
				return err
			}
			return fmt.Errorf("store posting: %w", err)
		}
		if ok {
			inserted++
		}
	}

	logger.Info("fetch finished",
		"source", "arbeitnow",
		"fetched", len(postings),
		"inserted", inserted,
		"duration", time.Since(start).Round(time.Second),
	)
	return nil
}
