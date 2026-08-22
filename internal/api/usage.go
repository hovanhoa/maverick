package api

import (
	"context"
	"time"

	"github.com/hovanhoa/llmgateway/internal/model"
)

// startOfMonthUTC returns midnight UTC on the 1st of t's month.
func startOfMonthUTC(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

func (r *Resolver) teamUsage(ctx context.Context, teamID string, since *time.Time) (*model.UsageSummary, error) {
	if err := requireTeamMember(ctx, teamID); err != nil {
		return nil, err
	}

	from := startOfMonthUTC(time.Now())
	if since != nil {
		from = *since
	}

	summary, err := r.deps.Database.SumTeamUsage(ctx, teamID, from)
	if err != nil {
		return nil, err
	}

	return &model.UsageSummary{
		RequestCount:     summary.RequestCount,
		PromptTokens:     summary.PromptTokens,
		CompletionTokens: summary.CompletionTokens,
		TotalTokens:      summary.TotalTokens,
		CostUsd:          summary.CostUSD,
	}, nil
}
