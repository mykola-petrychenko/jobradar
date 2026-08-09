// Package arbeitsagentur implements a fetch.Source for the
// Bundesagentur für Arbeit Jobsuche API.
package arbeitsagentur

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"time"

	"github.com/mykola-petrychenko/jobradar/internal/httpclient"
	"github.com/mykola-petrychenko/jobradar/internal/job"
)

const (
	endpoint    = "https://rest.arbeitsagentur.de/jobboerse/jobsuche-service/pc/v6/jobs"
	sourceName  = "arbeitsagentur"
	apiKey      = "jobboerse-jobsuche"
	pageSize    = 500
	berufsfeld  = "Softwareentwicklung und Programmierung"
	angebotsart = "1"
)

// Headers returns the request headers the API requires.
func Headers() map[string]string {
	return map[string]string{"X-API-Key": apiKey}
}

// Source knows how to talk to the Jobsuche API: the endpoint,
// the search filter, and the page/size pagination scheme.
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
	Ergebnisliste []json.RawMessage `json:"ergebnisliste"`
	MaxErgebnisse int               `json:"maxErgebnisse"`
	Messages      []apiMessage      `json:"messages"`
}

type apiMessage struct {
	Code   string `json:"code"`
	Path   string `json:"path"`
	Detail string `json:"detail"`
}

// FetchPage downloads and parses one page of the Jobsuche API.
// The API has no next-page link; a page shorter than pageSize is the last one.
func (s *Source) FetchPage(ctx context.Context, pageNum int, pageURL string) ([]job.Posting, string, error) {
	if pageURL == "" {
		pageURL = buildURL(1)
	}

	start := time.Now()

	body, err := s.http.Get(ctx, pageURL, fmt.Sprintf("%s-page-%03d", sourceName, pageNum))
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

	if len(p.Messages) > 0 {
		m := p.Messages[0]
		return nil, "", fmt.Errorf("api rejected request: %s %s: %s", m.Code, m.Path, m.Detail)
	}

	postings, skipped := parsePage(&p)

	next := ""
	if len(p.Ergebnisliste) == pageSize {
		next = buildURL(pageNum + 1)
	}

	s.logger.Debug("page fetched",
		"page", pageNum,
		"got", len(p.Ergebnisliste),
		"usable", len(postings),
		"malformed", skipped.malformed,
		"no_id", skipped.noID,
		"total_available", p.MaxErgebnisse,
		"has_next", next != "",
		"fetch_ms", time.Since(start).Milliseconds(),
	)

	return postings, next, nil
}

func buildURL(pageNum int) string {
	q := url.Values{
		"angebotsart": {angebotsart},
		"berufsfeld":  {berufsfeld},
		"size":        {strconv.Itoa(pageSize)},
		"page":        {strconv.Itoa(pageNum)},
	}
	return endpoint + "?" + q.Encode()
}

type skipCounts struct {
	malformed int
	noID      int
}

func parsePage(p *page) ([]job.Posting, skipCounts) {
	postings := make([]job.Posting, 0, len(p.Ergebnisliste))
	var skipped skipCounts

	for _, raw := range p.Ergebnisliste {
		var meta struct {
			Referenznummer string `json:"referenznummer"`
		}
		if err := json.Unmarshal(raw, &meta); err != nil {
			skipped.malformed++
			continue
		}
		if meta.Referenznummer == "" {
			skipped.noID++
			continue
		}
		postings = append(postings, job.Posting{
			Source:   sourceName,
			SourceID: meta.Referenznummer,
			Raw:      raw,
		})
	}
	return postings, skipped
}
