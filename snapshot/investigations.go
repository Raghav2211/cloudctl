package snapshot

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// StoredInvestigation is one `ctl investigate` run persisted to the local
// store — the roadmap's Phase 7 ask for investigations to be a durable
// audit trail, not a one-shot terminal report. StepsJSON/EvidenceJSON are
// left as raw JSON for the caller to unmarshal into whatever concrete type
// it expects (investigate.StepRecord / evidence.Evidence) — this package
// has no opinion on those shapes, matching StoredResource's convention.
type StoredInvestigation struct {
	ID                         int64
	Question                   string
	StoppedReason              string
	Conclusion                 string
	Summary                    string
	SummaryUnavailable         string
	Recommendations            string
	RecommendationsUnavailable string
	StepsJSON                  []byte
	EvidenceJSON               []byte
	CreatedAt                  string
}

// InvestigationSummary is the lightweight row returned by ListInvestigations
// — enough to pick which one to replay via GetInvestigation, without paying
// for every stored step/evidence blob just to list them.
type InvestigationSummary struct {
	ID            int64
	Question      string
	StoppedReason string
	CreatedAt     string
}

// SaveInvestigation persists a completed investigation under provider (e.g.
// "aws") and returns its ID, for `ctl investigate <provider> show <id>` to
// replay later.
func (s *Store) SaveInvestigation(ctx context.Context, provider string, inv StoredInvestigation) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO investigations (
			provider, question, stopped_reason, conclusion, summary, summary_unavailable,
			recommendations, recommendations_unavailable, steps_json, evidence_json, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		provider, inv.Question, inv.StoppedReason, inv.Conclusion, inv.Summary, inv.SummaryUnavailable,
		inv.Recommendations, inv.RecommendationsUnavailable, string(inv.StepsJSON), string(inv.EvidenceJSON),
		time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return 0, fmt.Errorf("saving investigation: %w", err)
	}
	return res.LastInsertId()
}

// ListInvestigations returns the most recent investigations for provider,
// newest first, capped at limit.
func (s *Store) ListInvestigations(ctx context.Context, provider string, limit int) ([]InvestigationSummary, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, question, stopped_reason, created_at FROM investigations
		 WHERE provider = ? ORDER BY id DESC LIMIT ?`,
		provider, limit)
	if err != nil {
		return nil, fmt.Errorf("listing investigations: %w", err)
	}
	defer rows.Close()

	var out []InvestigationSummary
	for rows.Next() {
		var it InvestigationSummary
		if err := rows.Scan(&it.ID, &it.Question, &it.StoppedReason, &it.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning investigation row: %w", err)
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// GetInvestigation returns one investigation by ID, or an error if none
// exists with that ID.
func (s *Store) GetInvestigation(ctx context.Context, id int64) (*StoredInvestigation, error) {
	var it StoredInvestigation
	var stepsJSON, evidenceJSON string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, question, stopped_reason, conclusion, summary, summary_unavailable,
		        recommendations, recommendations_unavailable, steps_json, evidence_json, created_at
		 FROM investigations WHERE id = ?`, id,
	).Scan(&it.ID, &it.Question, &it.StoppedReason, &it.Conclusion, &it.Summary, &it.SummaryUnavailable,
		&it.Recommendations, &it.RecommendationsUnavailable, &stepsJSON, &evidenceJSON, &it.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no investigation found with ID %d", id)
	}
	if err != nil {
		return nil, fmt.Errorf("finding investigation %d: %w", id, err)
	}
	it.StepsJSON = []byte(stepsJSON)
	it.EvidenceJSON = []byte(evidenceJSON)
	return &it, nil
}
