package postgres

import "context"

// TitlePosting is one posting pulled for title screening.
type TitlePosting struct {
	ID    int64
	Title string
}

// UnscreenedPostings returns every posting that has no title screening yet.
func (s *Store) UnscreenedPostings(ctx context.Context) ([]TitlePosting, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT p.id, COALESCE(p.raw->>'title', '')
		FROM postings p
		LEFT JOIN title_screenings t ON t.posting_id = p.id
		WHERE t.posting_id IS NULL
		ORDER BY p.id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []TitlePosting
	for rows.Next() {
		var p TitlePosting
		if err := rows.Scan(&p.ID, &p.Title); err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

// SaveTitleScreening records the title verdict for one posting.
func (s *Store) SaveTitleScreening(ctx context.Context, postingID int64,
	verdict string, inputTokens, outputTokens int64, model string) error {

	_, err := s.pool.Exec(ctx, `
		INSERT INTO title_screenings (posting_id, verdict, input_tokens, output_tokens, model)
		VALUES ($1, $2, $3, $4, $5)
	`, postingID, verdict, inputTokens, outputTokens, model)
	return err
}
