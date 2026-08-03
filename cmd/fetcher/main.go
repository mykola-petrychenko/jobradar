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
	"github.com/mykola-petrychenko/jobradar/internal/core"
	"github.com/mykola-petrychenko/jobradar/internal/postgres"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	if err := godotenv.Load(); err != nil {
		logger.Warn("no .env file, using environment", "err", err)
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		logger.Error("DATABASE_URL is not set")
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, logger, dsn); err != nil {
		logger.Error("run failed", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, logger *slog.Logger, dsn string) error {
	connectCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	store, err := postgres.New(connectCtx, dsn)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer store.Close()

	client := arbeitnow.New(logger)
	logger.Info("fetch started")
	start := time.Now()

	const stopAfterEmptyPages = 4

	var totalInserted, totalAlreadyInDB, totalFailed, emptyStreak int

	err = client.Fetch(ctx, func(ctx context.Context, postings []core.Posting) error {
		saveStart := time.Now()
		var newInDB, alreadyInDB, failed int

		for _, p := range postings {
			ok, err := store.Insert(ctx, p)
			if err != nil {
				logger.Warn("insert failed", "source_id", p.SourceID, "err", err)
				failed++
				continue
			}
			if ok {
				newInDB++
			} else {
				alreadyInDB++
			}
		}

		totalInserted += newInDB
		totalAlreadyInDB += alreadyInDB
		totalFailed += failed

		if newInDB == 0 && len(postings) > 0 {
			emptyStreak++
		} else {
			emptyStreak = 0
		}

		logger.Info("db save",
			"new_in_db", newInDB,
			"already_in_db", alreadyInDB,
			"failed", failed,
			"total_new_in_db", totalInserted,
			"empty_streak", emptyStreak,
			"duration", time.Since(saveStart).Round(time.Millisecond),
		)

		if emptyStreak >= stopAfterEmptyPages {
			logger.Info("no new postings for consecutive pages, stopping early",
				"empty_streak", emptyStreak)
			return arbeitnow.ErrDone
		}

		return ctx.Err()
	})

	logger.Info("fetch finished",
		"new_in_db", totalInserted,
		"already_in_db", totalAlreadyInDB,
		"failed", totalFailed,
		"duration", time.Since(start).Round(time.Second),
	)

	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			logger.Info("fetch canceled")
			return nil
		}
		return fmt.Errorf("fetch arbeitnow: %w", err)
	}
	return nil
}
