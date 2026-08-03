// Package arbeitnow fetches job postings from the Arbeitnow public API.
package arbeitnow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/mykola-petrychenko/jobradar/internal/core"
	"golang.org/x/time/rate"
)

const endpoint = "https://www.arbeitnow.com/api/job-board-api"
const Source = "arbeitnow"
const requestInterval = 3 * time.Second

type Client struct {
	http    *http.Client
	logger  *slog.Logger
	limiter *rate.Limiter
}

func New(logger *slog.Logger) *Client {
	return &Client{
		http:    &http.Client{Timeout: 15 * time.Second},
		logger:  logger,
		limiter: rate.NewLimiter(rate.Every(requestInterval), 1),
	}
}

type page struct {
	Data  []json.RawMessage `json:"data"`
	Links struct {
		Next string `json:"next"`
	} `json:"links"`
}

type PageFunc func(ctx context.Context, postings []core.Posting) error

var ErrDone = errors.New("stop fetching")

func (c *Client) Fetch(ctx context.Context, onPage PageFunc) error {
	seen := make(map[string]bool)
	url := endpoint

	for pageNum := 1; url != ""; pageNum++ {
		pageStart := time.Now()

		if err := c.limiter.Wait(ctx); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return err
			}
			return fmt.Errorf("rate limit wait: %w", err)
		}

		p, err := c.fetchPageRetrying(ctx, url, pageNum)
		if err != nil {
			return fmt.Errorf("page %d: %w", pageNum, err)
		}

		var batch []core.Posting
		var dup int

		for _, raw := range p.Data {
			var meta struct {
				Slug string `json:"slug"`
			}
			if err := json.Unmarshal(raw, &meta); err != nil || meta.Slug == "" {
				c.logger.Warn("json.Unmarshal error", "err", err, "slug", meta.Slug)
				continue
			}
			if seen[meta.Slug] {
				dup++
				continue
			}
			seen[meta.Slug] = true

			batch = append(batch, core.Posting{
				Source:   Source,
				SourceID: meta.Slug,
				Raw:      raw,
			})
		}

		c.logger.Info("fetched",
			"page", pageNum,
			"received", len(p.Data),
			"new", len(batch),
			"duplicates", dup,
			"has_next", p.Links.Next != "",
			"duration", time.Since(pageStart).Round(time.Millisecond),
		)

		if err := onPage(ctx, batch); err != nil {
			if errors.Is(err, ErrDone) {
				c.logger.Info("stopping: caller signaled done", "page", pageNum)
				return nil
			}
			return fmt.Errorf("page %d: %w", pageNum, err)
		}

		url = p.Links.Next
	}
	return nil
}

func (c *Client) fetchPage(ctx context.Context, url string, pageNum int) (*page, error) {
	var facts connFacts
	ctx = withConnTrace(ctx, &facts)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	dumpExchange(pageNum, req, resp, facts, bodyBytes)

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, &retryableError{
			status:     resp.StatusCode,
			retryAfter: resp.Header.Get("Retry-After"),
		}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %s", resp.Status)
	}

	var p page
	if err := json.Unmarshal(bodyBytes, &p); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &p, nil
}

func (c *Client) fetchPageRetrying(ctx context.Context, url string, pageNum int) (*page, error) {
	const maxAttempts = 5

	for attempt := 0; ; attempt++ {
		p, err := c.fetchPage(ctx, url, pageNum)

		var retryable *retryableError
		if err == nil {
			return p, nil
		}
		if !errors.As(err, &retryable) || attempt == maxAttempts-1 {
			return nil, err
		}

		wait := retryable.wait(attempt)
		c.logger.Warn("transient error, pausing before retry",
			"page", pageNum, "attempt", attempt+1, "wait", wait, "err", err)

		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

type retryableError struct {
	status     int
	retryAfter string
}

func (e *retryableError) Error() string {
	return fmt.Sprintf("transient error, status %d", e.status)
}

func (e *retryableError) wait(attempt int) time.Duration {
	if e.retryAfter != "" {
		if secs, err := strconv.Atoi(e.retryAfter); err == nil {
			return time.Duration(secs) * time.Second
		}
	}
	return time.Duration(1<<attempt) * time.Second // 1s, 2s, 4s, 8s...
}
