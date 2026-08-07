// Package fetch walks the pages of a job source and stores the postings
// it finds. It owns the fetch loop and the decision when to stop.
package fetch

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/mykola-petrychenko/jobradar/internal/job"
)

const defaultStopAfterKnownPages = 5

// Source is what fetch requires from a job board.
// Implementations must be stateless between calls: everything FetchPage
// needs arrives in its arguments.
type Source interface {
	// Name identifies the source in logs and in Posting.Source.
	Name() string

	// FetchPage downloads one page. An empty url means the first page;
	// an empty nextURL in the return values means there are no more pages.
	FetchPage(ctx context.Context, pageNum int, url string) (postings []job.Posting, nextURL string, err error)
}

// Store is what fetch requires from a storage backend.
type Store interface {
	// InsertPage saves one page of postings and reports how many were new.
	InsertPage(ctx context.Context, postings []job.Posting) (int, error)
}

// Options configures a single fetch run.
type Options struct {
	// StopAfterKnownPages stops the run after that many consecutive pages
	// produced no new postings. Zero means the default.
	StopAfterKnownPages int
}

// Result summarizes what one fetch run did.
type Result struct {
	Source      string
	Pages       int
	NewInDB     int
	AlreadyInDB int
	StopReason  string
	Duration    time.Duration
}

// LogValue renders Result as a group of slog fields.
func (r Result) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("source", r.Source),
		slog.Int("pages", r.Pages),
		slog.Int("inserted", r.NewInDB),
		slog.Int("already_had", r.AlreadyInDB),
		slog.String("reason", r.StopReason),
		slog.Duration("duration", r.Duration.Round(time.Second)),
	)
}

// Fetcher connects a job source to a storage backend.
// Create it with New, start it with Run.
type Fetcher struct {
	logger *slog.Logger
	src    Source
	store  Store
	opts   Options
}

// New builds a Fetcher and applies option defaults.
func New(logger *slog.Logger, src Source, store Store, opts Options) *Fetcher {
	if opts.StopAfterKnownPages == 0 {
		opts.StopAfterKnownPages = defaultStopAfterKnownPages
	}

	return &Fetcher{logger: logger, src: src, store: store, opts: opts}
}

// Run performs one full fetch: it walks the source page by page,
// stores each page, and stops early once several consecutive pages
// contain nothing new.
func (f *Fetcher) Run(ctx context.Context) (Result, error) {
	f.logger.Info("fetch started",
		"source", f.src.Name(),
		"stop_after_known_pages", f.opts.StopAfterKnownPages,
	)

	start := time.Now()
	res := Result{Source: f.src.Name(), StopReason: "all pages fetched"}

	var url string
	pagesWithoutNew := 0
	seen := make(map[string]bool)

	for pageNum := 1; ; pageNum++ {
		fetchStart := time.Now()
		postings, nextURL, err := f.src.FetchPage(ctx, pageNum, url)
		if err != nil {
			res.StopReason = stopReason(ctx)
			res.Duration = time.Since(start)
			return res, fmt.Errorf("fetch page %d: %w", pageNum, err)
		}
		fetchTook := time.Since(fetchStart)
		res.Pages++

		fresh := make([]job.Posting, 0, len(postings))
		repeated := 0
		for _, p := range postings {
			if seen[p.SourceID] {
				repeated++
				continue
			}
			seen[p.SourceID] = true
			fresh = append(fresh, p)
		}

		saveStart := time.Now()
		newInDB, err := f.store.InsertPage(ctx, fresh)
		if err != nil {
			res.StopReason = stopReason(ctx)
			res.Duration = time.Since(start)
			return res, fmt.Errorf("save page %d: %w", pageNum, err)
		}
		saveTook := time.Since(saveStart)

		alreadyInDB := len(fresh) - newInDB
		res.NewInDB += newInDB
		res.AlreadyInDB += alreadyInDB

		if newInDB > 0 {
			pagesWithoutNew = 0
		} else {
			pagesWithoutNew++
		}

		f.logger.Debug("page done",
			"page", pageNum,
			"got", len(postings),
			"duplicate_in_run", repeated,
			"inserted", newInDB,
			"already_had", alreadyInDB,
			"empty_pages_streak", pagesWithoutNew,
			"total_inserted", res.NewInDB,
			"fetch_ms", fetchTook.Milliseconds(),
			"save_ms", saveTook.Milliseconds(),
		)

		if pagesWithoutNew >= f.opts.StopAfterKnownPages {
			res.StopReason = "stopped early: last pages had nothing new"
			break
		}
		if nextURL == "" {
			break
		}
		url = nextURL
	}

	if ctx.Err() != nil {
		res.StopReason = "canceled"
	}

	res.Duration = time.Since(start)
	return res, nil
}

func stopReason(ctx context.Context) string {
	if ctx.Err() != nil {
		return "canceled"
	}
	return "failed"
}
