package db

import (
	"context"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/hovanhoa/llmgateway/internal/model"
	"github.com/hovanhoa/llmgateway/pkg/core/encoding"
	"github.com/hovanhoa/llmgateway/pkg/core/errors"
)

// RecordUsageEvent persists a single, immutable usage_event row. If
// event.ID is empty, one is generated.
func (db *Database) RecordUsageEvent(ctx context.Context, event *model.UsageEvent) error {
	if event.ID == "" {
		event.ID = encoding.NewRandomIdentifier("usage")
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}

	query, args, err := db.GetSQLClient().Builder().
		Insert("usage_event").
		Columns("id", "request_id", "account_id", "team_id", "provider", "model", "prompt_tokens", "completion_tokens", "total_tokens", "cost_usd", "created_at").
		Values(event.ID, event.RequestID, event.AccountID, event.TeamID, event.Provider, event.Model, event.PromptTokens, event.CompletionTokens, event.TotalTokens, event.CostUSD, event.CreatedAt).
		ToSql()
	if err != nil {
		return errors.Wrap(err, "build insert usage_event query")
	}

	if _, err := db.GetSQLClient().Runner().Exec(ctx, query, args...); err != nil {
		return errors.Wrap(err, "error executing %s [args: %v]", query, args)
	}

	return nil
}

// UsageSummary aggregates usage_event rows. Mirrors the GraphQL
// model.UsageSummary shape without depending on the generated package.
type UsageSummary struct {
	RequestCount     int
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	CostUSD          float64
}

// SumTeamUsage aggregates usage_event rows for a team since the given time.
func (db *Database) SumTeamUsage(ctx context.Context, teamID string, since time.Time) (UsageSummary, error) {
	query, args, err := db.GetSQLClient().Builder().
		Select(
			"COUNT(*)",
			"COALESCE(SUM(prompt_tokens), 0)",
			"COALESCE(SUM(completion_tokens), 0)",
			"COALESCE(SUM(total_tokens), 0)",
			"COALESCE(SUM(cost_usd), 0)",
		).
		From("usage_event").
		Where(sq.Eq{"team_id": teamID}).
		Where(sq.GtOrEq{"created_at": since}).
		ToSql()
	if err != nil {
		return UsageSummary{}, errors.Wrap(err, "build sum usage_event query")
	}

	var summary UsageSummary
	row := db.GetSQLClient().Runner().QueryRow(ctx, query, args...)
	if err := row.Scan(&summary.RequestCount, &summary.PromptTokens, &summary.CompletionTokens, &summary.TotalTokens, &summary.CostUSD); err != nil {
		return UsageSummary{}, errors.Wrap(err, "error executing %s [args: %v]", query, args)
	}

	return summary, nil
}
