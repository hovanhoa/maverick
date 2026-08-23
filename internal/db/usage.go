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

// usageSummaryColumns are the aggregate columns shared by every
// usage_event rollup query below, in the fixed order scanned into a
// UsageSummary (or a struct embedding one).
var usageSummaryColumns = []string{
	"COUNT(*)",
	"COALESCE(SUM(prompt_tokens), 0)",
	"COALESCE(SUM(completion_tokens), 0)",
	"COALESCE(SUM(total_tokens), 0)",
	"COALESCE(SUM(cost_usd), 0)",
}

// scanUsageSummary scans the usageSummaryColumns projection, in order,
// into s.
func scanUsageSummary(row interface{ Scan(...any) error }, s *UsageSummary) error {
	return row.Scan(&s.RequestCount, &s.PromptTokens, &s.CompletionTokens, &s.TotalTokens, &s.CostUSD)
}

// SumTeamUsage aggregates usage_event rows for a team since the given time.
func (db *Database) SumTeamUsage(ctx context.Context, teamID string, since time.Time) (UsageSummary, error) {
	query, args, err := db.GetSQLClient().Builder().
		Select(usageSummaryColumns...).
		From("usage_event").
		Where(sq.Eq{"team_id": teamID}).
		Where(sq.GtOrEq{"created_at": since}).
		ToSql()
	if err != nil {
		return UsageSummary{}, errors.Wrap(err, "build sum usage_event query")
	}

	var summary UsageSummary
	row := db.GetSQLClient().Runner().QueryRow(ctx, query, args...)
	if err := scanUsageSummary(row, &summary); err != nil {
		return UsageSummary{}, errors.Wrap(err, "error executing %s [args: %v]", query, args)
	}

	return summary, nil
}

// SumAccountUsage aggregates usage_event rows for a single account since
// the given time.
func (db *Database) SumAccountUsage(ctx context.Context, accountID string, since time.Time) (UsageSummary, error) {
	query, args, err := db.GetSQLClient().Builder().
		Select(usageSummaryColumns...).
		From("usage_event").
		Where(sq.Eq{"account_id": accountID}).
		Where(sq.GtOrEq{"created_at": since}).
		ToSql()
	if err != nil {
		return UsageSummary{}, errors.Wrap(err, "build sum usage_event query")
	}

	var summary UsageSummary
	row := db.GetSQLClient().Runner().QueryRow(ctx, query, args...)
	if err := scanUsageSummary(row, &summary); err != nil {
		return UsageSummary{}, errors.Wrap(err, "error executing %s [args: %v]", query, args)
	}

	return summary, nil
}

// SumGlobalUsage aggregates usage_event rows across every team and account
// since the given time, for platform-wide reporting.
func (db *Database) SumGlobalUsage(ctx context.Context, since time.Time) (UsageSummary, error) {
	query, args, err := db.GetSQLClient().Builder().
		Select(usageSummaryColumns...).
		From("usage_event").
		Where(sq.GtOrEq{"created_at": since}).
		ToSql()
	if err != nil {
		return UsageSummary{}, errors.Wrap(err, "build sum usage_event query")
	}

	var summary UsageSummary
	row := db.GetSQLClient().Runner().QueryRow(ctx, query, args...)
	if err := scanUsageSummary(row, &summary); err != nil {
		return UsageSummary{}, errors.Wrap(err, "error executing %s [args: %v]", query, args)
	}

	return summary, nil
}

// AccountUsageSummary is a UsageSummary attributed to one account, e.g. one
// row of a team's per-member usage breakdown.
type AccountUsageSummary struct {
	AccountID string
	UsageSummary
}

// TeamUsageByAccount aggregates a team's usage_event rows since the given
// time, grouped by account, for a per-member usage breakdown. Accounts
// with no usage in the window are omitted rather than returned as zero
// rows.
func (db *Database) TeamUsageByAccount(ctx context.Context, teamID string, since time.Time) ([]AccountUsageSummary, error) {
	query, args, err := db.GetSQLClient().Builder().
		Select(append([]string{"account_id"}, usageSummaryColumns...)...).
		From("usage_event").
		Where(sq.Eq{"team_id": teamID}).
		Where(sq.GtOrEq{"created_at": since}).
		GroupBy("account_id").
		OrderBy("SUM(cost_usd) DESC").
		ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "build team usage by account query")
	}

	rows, err := db.GetSQLClient().Runner().Query(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "error executing %s [args: %v]", query, args)
	}
	defer rows.Close()

	var out []AccountUsageSummary
	for rows.Next() {
		var row AccountUsageSummary
		if err := rows.Scan(&row.AccountID, &row.RequestCount, &row.PromptTokens, &row.CompletionTokens, &row.TotalTokens, &row.CostUSD); err != nil {
			return nil, errors.Wrap(err, "scan team usage by account row")
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "iterate team usage by account rows")
	}

	return out, nil
}

// ModelUsageSummary is a UsageSummary attributed to one provider/model
// pair, e.g. one row of a team's usage-by-model breakdown.
type ModelUsageSummary struct {
	Provider string
	Model    string
	UsageSummary
}

// UsageByModel aggregates a team's usage_event rows since the given time,
// grouped by provider and model.
func (db *Database) UsageByModel(ctx context.Context, teamID string, since time.Time) ([]ModelUsageSummary, error) {
	query, args, err := db.GetSQLClient().Builder().
		Select(append([]string{"provider", "model"}, usageSummaryColumns...)...).
		From("usage_event").
		Where(sq.Eq{"team_id": teamID}).
		Where(sq.GtOrEq{"created_at": since}).
		GroupBy("provider", "model").
		OrderBy("SUM(cost_usd) DESC").
		ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "build usage by model query")
	}

	rows, err := db.GetSQLClient().Runner().Query(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "error executing %s [args: %v]", query, args)
	}
	defer rows.Close()

	var out []ModelUsageSummary
	for rows.Next() {
		var row ModelUsageSummary
		if err := rows.Scan(&row.Provider, &row.Model, &row.RequestCount, &row.PromptTokens, &row.CompletionTokens, &row.TotalTokens, &row.CostUSD); err != nil {
			return nil, errors.Wrap(err, "scan usage by model row")
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "iterate usage by model rows")
	}

	return out, nil
}

// TeamUsageSummary is a UsageSummary attributed to one team, e.g. one row
// of the platform-wide usage-by-team breakdown.
type TeamUsageSummary struct {
	TeamID string
	UsageSummary
}

// GlobalUsageByTeam aggregates usage_event rows across the whole platform
// since the given time, grouped by team. Rows with a nil team_id (calls
// made by an account with no team) are omitted.
func (db *Database) GlobalUsageByTeam(ctx context.Context, since time.Time) ([]TeamUsageSummary, error) {
	query, args, err := db.GetSQLClient().Builder().
		Select(append([]string{"team_id"}, usageSummaryColumns...)...).
		From("usage_event").
		Where(sq.NotEq{"team_id": nil}).
		Where(sq.GtOrEq{"created_at": since}).
		GroupBy("team_id").
		OrderBy("SUM(cost_usd) DESC").
		ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "build global usage by team query")
	}

	rows, err := db.GetSQLClient().Runner().Query(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "error executing %s [args: %v]", query, args)
	}
	defer rows.Close()

	var out []TeamUsageSummary
	for rows.Next() {
		var row TeamUsageSummary
		if err := rows.Scan(&row.TeamID, &row.RequestCount, &row.PromptTokens, &row.CompletionTokens, &row.TotalTokens, &row.CostUSD); err != nil {
			return nil, errors.Wrap(err, "scan global usage by team row")
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "iterate global usage by team rows")
	}

	return out, nil
}

// DailyUsageSummary is a coarser rollup than UsageSummary, meant for
// charting a usage trend: one row per calendar day, with request count,
// total tokens, and cost, but no prompt/completion token split.
type DailyUsageSummary struct {
	Day          time.Time
	RequestCount int
	TotalTokens  int
	CostUSD      float64
}

// UsageDaily aggregates a team's usage_event rows between since and until,
// grouped by calendar day (UTC), for a daily usage trend chart. Days with
// no usage are omitted rather than returned as zero rows.
func (db *Database) UsageDaily(ctx context.Context, teamID string, since, until time.Time) ([]DailyUsageSummary, error) {
	query, args, err := db.GetSQLClient().Builder().
		Select(
			"date_trunc('day', created_at)",
			"COUNT(*)",
			"COALESCE(SUM(total_tokens), 0)",
			"COALESCE(SUM(cost_usd), 0)",
		).
		From("usage_event").
		Where(sq.Eq{"team_id": teamID}).
		Where(sq.GtOrEq{"created_at": since}).
		Where(sq.LtOrEq{"created_at": until}).
		GroupBy("date_trunc('day', created_at)").
		OrderBy("date_trunc('day', created_at) ASC").
		ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "build usage daily query")
	}

	rows, err := db.GetSQLClient().Runner().Query(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "error executing %s [args: %v]", query, args)
	}
	defer rows.Close()

	var out []DailyUsageSummary
	for rows.Next() {
		var row DailyUsageSummary
		if err := rows.Scan(&row.Day, &row.RequestCount, &row.TotalTokens, &row.CostUSD); err != nil {
			return nil, errors.Wrap(err, "scan usage daily row")
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "iterate usage daily rows")
	}

	return out, nil
}
