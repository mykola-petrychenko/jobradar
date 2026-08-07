// Package httpclient performs rate-limited HTTP GETs with retries.
package httpclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"golang.org/x/time/rate"
)

const (
	defaultRequestInterval = 5 * time.Second
	defaultTimeout         = 30 * time.Second
	defaultMaxAttempts     = 5
	defaultUserAgent       = "jobradar/1.0 (+https://github.com/mykola-petrychenko/jobradar)"
)

// Client is an HTTP client with rate limiting and automatic retries
// for transient failures.
type Client struct {
	http        *http.Client
	logger      *slog.Logger
	limiter     *rate.Limiter
	dumper      Dumper
	maxAttempts int
	userAgent   string
}

// Options configures a Client. The zero value of every field selects
// a sensible default.
type Options struct {
	// RequestInterval is the minimum delay between requests.
	// Zero or negative means defaultRequestInterval.
	RequestInterval time.Duration

	// Timeout limits one request, including reading the body.
	// Zero or negative means defaultTimeout.
	Timeout time.Duration

	// MaxAttempts counts the first try plus retries.
	// Zero or negative means defaultMaxAttempts.
	MaxAttempts int

	// DumpDir enables writing each request and response to that directory.
	// Empty disables dumping.
	DumpDir string

	// UserAgent identifies this client to servers.
	// Empty means defaultUserAgent.
	UserAgent string
}

// New builds a Client, applying defaults for any unset option.
func New(logger *slog.Logger, opts Options) *Client {
	interval := opts.RequestInterval
	if interval <= 0 {
		interval = defaultRequestInterval
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	attempts := opts.MaxAttempts
	if attempts <= 0 {
		attempts = defaultMaxAttempts
	}

	var dumper Dumper = noopDumper{}
	if opts.DumpDir != "" {
		dumper = fileDumper{dir: opts.DumpDir, logger: logger}
	}

	userAgent := opts.UserAgent
	if userAgent == "" {
		userAgent = defaultUserAgent
	}

	return &Client{
		http:        &http.Client{Timeout: timeout},
		logger:      logger,
		limiter:     rate.NewLimiter(rate.Every(interval), 1),
		dumper:      dumper,
		maxAttempts: attempts,
		userAgent:   userAgent,
	}
}

// Get fetches url and returns the response body. label identifies the
// request in log records and dump file names.
func (c *Client) Get(ctx context.Context, url, label string) ([]byte, error) {
	for attempt := 1; ; attempt++ {
		if err := c.limiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("waiting for rate limiter: %w", err)
		}

		body, err := c.get(ctx, url, label)
		if err == nil {
			return body, nil
		}

		var retryable *retryableError
		if !errors.As(err, &retryable) {
			return nil, err
		}
		if attempt == c.maxAttempts {
			return nil, fmt.Errorf("giving up after %d attempts: %w", c.maxAttempts, err)
		}

		wait := retryable.wait(attempt - 1)
		c.logger.Warn("retrying after transient failure",
			"url", url,
			"attempt", attempt,
			"max_attempts", c.maxAttempts,
			"wait_s", wait.Seconds(),
			"err", err,
		)

		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (c *Client) get(ctx context.Context, url, label string) ([]byte, error) {
	trace := newConnTrace()
	traceCtx := withConnTrace(ctx, trace)

	req, err := http.NewRequestWithContext(traceCtx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}

	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, &retryableError{cause: err}
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	c.dumper.Dump(label, req, resp, trace, body)

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, &retryableError{
			status:     resp.StatusCode,
			retryAfter: resp.Header.Get("Retry-After"),
		}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %s for %s", resp.Status, url)
	}
	return body, nil
}
