// Command titleclassify runs the title-only pre-filter over all
// postings that have not been screened yet.
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

	"github.com/mykola-petrychenko/jobradar/internal/claude"
	"github.com/mykola-petrychenko/jobradar/internal/postgres"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	if err := run(ctx, logger); err != nil {
		logger.Error("run failed", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, logger *slog.Logger) error {
	godotenv.Load()

	store, err := postgres.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return fmt.Errorf("connect to postgres: %w", err)
	}
	defer store.Close()

	ai := claude.New()

	postings, err := store.UnscreenedPostings(ctx)
	if err != nil {
		return fmt.Errorf("load postings: %w", err)
	}
	if len(postings) == 0 {
		logger.Info("nothing to classify, queue empty")
		return nil
	}

	logger.Info("run started", "queue", len(postings))
	start := time.Now()

	stats := struct {
		done, failed        int
		byVerdict           map[string]int
		inTokens, outTokens int64
	}{byVerdict: map[string]int{}}

	for _, p := range postings {
		res, err := ai.ScreenTitle(ctx, p.Title)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				logger.Info("run interrupted", "done", stats.done, "remaining", len(postings)-stats.done)
				return err
			}
			logger.Warn("screen failed", "posting_id", p.ID, "err", err)
			stats.failed++
			continue
		}

		if err := store.SaveTitleScreening(ctx, p.ID,
			res.Verdict, res.InputTokens, res.OutputTokens, res.Model); err != nil {
			if errors.Is(err, context.Canceled) {
				return err
			}
			logger.Warn("save failed", "posting_id", p.ID, "err", err)
			stats.failed++
			continue
		}

		stats.done++
		stats.byVerdict[res.Verdict]++
		stats.inTokens += res.InputTokens
		stats.outTokens += res.OutputTokens

		if stats.done%20 == 0 {
			logger.Info("progress", "processed", stats.done, "of", len(postings))
		}
	}

	logger.Info("run finished",
		"done", stats.done,
		"failed", stats.failed,
		"it", stats.byVerdict[claude.VerdictIT],
		"not_it", stats.byVerdict[claude.VerdictNotIT],
		"unsure", stats.byVerdict[claude.VerdictUnsure],
		"in_tokens", stats.inTokens,
		"out_tokens", stats.outTokens,
		"duration", time.Since(start).Round(time.Second),
	)
	return nil
}
