// Package arbeitnow implements a fetch.Source for the public
// Arbeitnow job board API.
package arbeitnow

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/mykola-petrychenko/jobradar/internal/httpclient"
	"github.com/mykola-petrychenko/jobradar/internal/job"
)

const (
	endpoint   = "https://www.arbeitnow.com/api/job-board-api"
	sourceName = "arbeitnow"
)

// Source knows how to talk to the Arbeitnow API: the endpoint,
// the response format, and where the next-page link lives.
type Source struct {
	http   *httpclient.Client
	logger *slog.Logger
}

// New builds a Source that performs requests through the given client.
func New(logger *slog.Logger, http *httpclient.Client) *Source {
	return &Source{http: http, logger: logger}
}

// Name identifies this source in logs and stored postings.
func (s *Source) Name() string { return sourceName }

type page struct {
	Data  []json.RawMessage `json:"data"`
	Links struct {
		Next string `json:"next"`
	} `json:"links"`
}

// FetchPage downloads and parses one page of the Arbeitnow API.
// It is stateless: everything it needs arrives in the arguments.
func (s *Source) FetchPage(ctx context.Context, pageNum int, url string) ([]job.Posting, string, error) {
	if url == "" {
		url = endpoint
	}

	start := time.Now()

	body, err := s.http.Get(ctx, url, fmt.Sprintf("page-%03d", pageNum))
	if err != nil {
		return nil, "", err
	}

	var p page
	if err := json.Unmarshal(body, &p); err != nil {
		head := body
		if len(head) > 50 {
			head = head[:50]
		}
		return nil, "", fmt.Errorf("decode page body (%d bytes, starts with %q): %w",
			len(body), head, err)
	}

	postings, skipped := parsePage(&p)

	s.logger.Debug("page fetched",
		"page", pageNum,
		"got", len(p.Data),
		"usable", len(postings),
		"malformed", skipped.malformed,
		"no_id", skipped.noID,
		"has_next", p.Links.Next != "",
		"fetch_ms", time.Since(start).Milliseconds(),
	)

	return postings, p.Links.Next, nil
}

type skipCounts struct {
	malformed int
	noID      int
}

func parsePage(p *page) ([]job.Posting, skipCounts) {
	postings := make([]job.Posting, 0, len(p.Data))
	var skipped skipCounts

	for _, raw := range p.Data {
		var meta struct {
			Slug string `json:"slug"`
		}
		if err := json.Unmarshal(raw, &meta); err != nil {
			skipped.malformed++
			continue
		}
		if meta.Slug == "" {
			skipped.noID++
			continue
		}
		postings = append(postings, job.Posting{
			Source:   sourceName,
			SourceID: meta.Slug,
			Raw:      raw,
		})
	}
	return postings, skipped
}
