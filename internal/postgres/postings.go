package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/mykola-petrychenko/jobradar/internal/job"
)

// InsertPage saves one page of postings in a single transaction and
// reports how many rows were actually inserted. Postings whose
// (source, source_id) already exists are skipped by ON CONFLICT and
// counted as not new.
func (s *Store) InsertPage(ctx context.Context, postings []job.Posting) (int, error) {
	if len(postings) == 0 {
		return 0, nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	batch := &pgx.Batch{}
	for _, p := range postings {
		batch.Queue(
			`INSERT INTO postings (source, source_id, raw)
			 VALUES ($1, $2, $3)
			 ON CONFLICT (source, source_id) DO NOTHING`,
			p.Source, p.SourceID, p.Raw)
	}

	br := tx.SendBatch(ctx, batch)

	inserted := 0
	for i := range postings {
		tag, err := br.Exec()
		if err != nil {
			_ = br.Close()
			return 0, fmt.Errorf("insert %s/%s: %w",
				postings[i].Source, postings[i].SourceID, err)
		}
		if tag.RowsAffected() == 1 {
			inserted++
		}
	}

	if err := br.Close(); err != nil {
		return 0, fmt.Errorf("close batch: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	return inserted, nil
}
