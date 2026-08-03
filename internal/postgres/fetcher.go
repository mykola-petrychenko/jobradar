// Queries used by cmd/fetcher: inserting new postings and tracking the
// per-source cursor used to avoid re-fetching old data.
package postgres

import (
	"context"
	"fmt"

	"github.com/mykola-petrychenko/jobradar/internal/core"
)

// Insert saves one posting. Duplicates (same Source+SourceID) are
// silently skipped. It reports whether a new row was inserted.
func (s *Store) Insert(ctx context.Context, p core.Posting) (bool, error) {
	tag, err := s.pool.Exec(ctx,
		`INSERT INTO postings (source, source_id, raw)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (source, source_id) DO NOTHING`,
		p.Source, p.SourceID, p.Raw)
	if err != nil {
		return false, fmt.Errorf("insert %s/%s: %w", p.Source, p.SourceID, err)
	}
	return tag.RowsAffected() == 1, nil
}

// Queries used by cmd/fetcher: inserting new postings and tracking the
// per-source cursor used to avoid re-fetching old data.
func (s *Store) LatestCreatedAt(ctx context.Context, source string) (int64, error) {
	var ts int64
	err := s.pool.QueryRow(ctx,
		`SELECT COALESCE(MAX((raw->>'created_at')::bigint), 0)
		 FROM postings WHERE source = $1`, source).Scan(&ts)
	if err != nil {
		return 0, fmt.Errorf("latest created_at for %s: %w", source, err)
	}
	return ts, nil
}
