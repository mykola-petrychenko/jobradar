package postgres

import "context"

// ClassifyPosting is one posting pulled for the primary is_it decision.
type ClassifyPosting struct {
	ID          int64
	Description string
	URL         string
}

// UnclassifiedPostings returns postings with no classification row yet
func (s *Store) UnclassifiedPostings(ctx context.Context) ([]ClassifyPosting, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT p.id,
		       COALESCE(p.raw->>'description', ''),
		       COALESCE(p.raw->>'url', '')
		FROM postings p
		JOIN title_screenings t ON t.posting_id = p.id
		LEFT JOIN classifications c ON c.posting_id = p.id
		WHERE t.verdict IN ('it', 'unsure')
		AND c.posting_id IS NULL
		ORDER BY p.id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]ClassifyPosting, 0)
	for rows.Next() {
		var p ClassifyPosting
		if err := rows.Scan(&p.ID, &p.Description, &p.URL); err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

func (s *Store) SaveClassification(ctx context.Context, postingID int64,
	isIT bool, explanation, thinking, model string, inputTokens, outputTokens int64) error {

	_, err := s.pool.Exec(ctx, `
		INSERT INTO classifications
			(posting_id, is_it, explanation, thinking, model, input_tokens, output_tokens)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, postingID, isIT, explanation, thinking, model, inputTokens, outputTokens)
	return err
}
