// Package arbeitnow fetches job postings from the Arbeitnow public API.
package arbeitnow

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/mykola-petrychenko/jobradar/internal/core"
)

const endpoint = "https://www.arbeitnow.com/api/job-board-api"

// Client downloads postings from Arbeitnow.
type Client struct {
	http *http.Client
}

// New returns a Client with a sane request timeout.
func New() *Client {
	return &Client{http: &http.Client{Timeout: 15 * time.Second}}
}

// page mirrors one API response page: data + link to the next page.
type page struct {
	Data  []json.RawMessage `json:"data"`
	Links struct {
		Next string `json:"next"`
	} `json:"links"`
}

func (c *Client) fetchPage(ctx context.Context, url string) (*page, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %s", resp.Status)
	}

	var p page
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &p, nil
}

// Fetch downloads postings newer than sinceUnix, walking pages from
// newest to oldest and stopping at the first posting at or below it.
func (c *Client) Fetch(ctx context.Context, sinceUnix int64) ([]core.Posting, error) {
	var postings []core.Posting
	url := endpoint

	for page := 1; url != ""; page++ {
		p, err := c.fetchPage(ctx, url)
		if err != nil {
			return nil, fmt.Errorf("page %d: %w", page, err)
		}

		for _, raw := range p.Data {
			var meta struct {
				Slug      string `json:"slug"`
				CreatedAt int64  `json:"created_at"`
			}
			if err := json.Unmarshal(raw, &meta); err != nil || meta.Slug == "" {
				continue
			}
			if meta.CreatedAt <= sinceUnix {
				return postings, nil
			}
			postings = append(postings, core.Posting{
				Source:   "arbeitnow",
				SourceID: meta.Slug,
				Raw:      raw,
			})
		}

		url = p.Links.Next
		time.Sleep(3 * time.Second)
	}
	return postings, nil
}
