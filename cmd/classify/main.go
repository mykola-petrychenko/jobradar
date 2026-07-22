package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/mykola-petrychenko/jobradar/internal/claude"
	"github.com/mykola-petrychenko/jobradar/internal/postgres"
	"github.com/mykola-petrychenko/jobradar/internal/util"
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
	if err := godotenv.Load(); err != nil {
		logger.Info("no .env file, using environment")
	}

	store, err := postgres.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		logger.Error("connect to postgres", "err", err)
		os.Exit(1)
	}
	defer store.Close()

	client := claude.New()

	postings, err := store.UnclassifiedPostings(ctx)
	if err != nil {
		logger.Error("load postings", "err", err)
		os.Exit(1)
	}

	start := time.Now()
	total, itCount := 0, 0
	var totalIn, totalOut int64

	for _, p := range postings {
		logger.Info("sending", "posting_id", p.ID)

		res, err := client.Classify(ctx, util.CleanHTML(p.Description))
		if err != nil {
			logger.Error("classify failed", "posting_id", p.ID, "err", err)
			continue
		}

		if err := store.SaveClassification(ctx, p.ID,
			res.IsIT, res.Explanation, res.Thinking, res.Model, res.InputTokens, res.OutputTokens); err != nil {
			logger.Error("save failed", "posting_id", p.ID, "err", err)
			continue
		}
		logger.Info("done",
			"posting_id", p.ID,
			"is_it", res.IsIT,
			"in", res.InputTokens,
			"out", res.OutputTokens,
			"thinking_tokens", res.ThinkingTokens,
			"answer_tokens", res.OutputTokens-res.ThinkingTokens,
			"URL", p.URL,
		)

		total++
		totalIn += res.InputTokens
		totalOut += res.OutputTokens
		if res.IsIT {
			itCount++
		}
	}

	logger.Info("classify run finished",
		"total", total,
		"marked_it", itCount,
		"total_in_tokens", totalIn,
		"total_out_tokens", totalOut,
		"duration", time.Since(start).Round(time.Second),
	)
	return nil
}
