package db

import (
	"context"
	"database/sql"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/hovanhoa/llmgateway/internal/model"
	"github.com/hovanhoa/llmgateway/pkg/core/encoding"
	"github.com/hovanhoa/llmgateway/pkg/core/errors"
)

// requestLogColumns is the fixed column order used by both InsertRequestLog
// and every scan in this file, so the two stay in lockstep.
var requestLogColumns = []string{
	"id", "request_id", "account_id", "team_id", "provider", "model",
	"requested_model", "status", "error_kind", "error_message", "stream",
	"request_body", "response_body", "prompt_tokens", "completion_tokens",
	"total_tokens", "cost_usd", "latency_ms", "created_at",
}

// InsertRequestLog persists a single, immutable request_log row. If
// entry.ID is empty, one is generated. Every proxy call - success or
// failure, ever reaching a provider or not - gets exactly one row.
func (db *Database) InsertRequestLog(ctx context.Context, entry *model.RequestLog) error {
	if entry.ID == "" {
		entry.ID = encoding.NewRandomIdentifier("reqlog")
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now().UTC()
	}

	query, args, err := db.GetSQLClient().Builder().
		Insert("request_log").
		Columns(requestLogColumns...).
		Values(
			entry.ID, entry.RequestID, entry.AccountID, entry.TeamID, entry.Provider, entry.Model,
			entry.RequestedModel, string(entry.Status), entry.ErrorKind, entry.ErrorMessage, entry.Stream,
			entry.RequestBody, entry.ResponseBody, entry.PromptTokens, entry.CompletionTokens,
			entry.TotalTokens, entry.CostUsd, entry.LatencyMs, entry.CreatedAt,
		).
		ToSql()
	if err != nil {
		return errors.Wrap(err, "build insert request_log query")
	}

	if _, err := db.GetSQLClient().Runner().Exec(ctx, query, args...); err != nil {
		return errors.Wrap(err, "error executing %s [args: %v]", query, args)
	}

	return nil
}

// RequestLogFilter scopes a ListRequestLogs/CountRequestLogs query. At most
// one of TeamID/AccountID is expected to be set by any given caller -
// TeamID for a team-scoped query, AccountID for a self-scoped one; neither
// set means unrestricted (platform-wide).
type RequestLogFilter struct {
	TeamID    *string
	AccountID *string
}

func (f RequestLogFilter) apply(stmt sq.SelectBuilder) sq.SelectBuilder {
	if f.TeamID != nil {
		stmt = stmt.Where(sq.Eq{"team_id": *f.TeamID})
	}
	if f.AccountID != nil {
		stmt = stmt.Where(sq.Eq{"account_id": *f.AccountID})
	}
	return stmt
}

// ListRequestLogs returns a page of request logs matching filter, most
// recently created first, along with the total number of matching rows
// ignoring limit and offset.
func (db *Database) ListRequestLogs(ctx context.Context, filter RequestLogFilter, limit, offset int) ([]model.RequestLog, int, error) {
	total, err := db.CountRequestLogs(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	// Nothing to fetch, and Postgres would happily scan for it anyway.
	if offset >= total {
		return nil, total, nil
	}

	query, args, err := filter.apply(
		db.GetSQLClient().Builder().
			Select(requestLogColumns...).
			From("request_log"),
	).
		OrderBy("created_at DESC", "id DESC").
		Limit(uint64(limit)).
		Offset(uint64(offset)).
		ToSql()
	if err != nil {
		return nil, 0, errors.Wrap(err, "build list request_log query")
	}

	rows, err := db.GetSQLClient().Runner().Query(ctx, query, args...)
	if err != nil {
		return nil, 0, errors.Wrap(err, "error executing %s [args: %v]", query, args)
	}
	defer rows.Close()

	var out []model.RequestLog
	for rows.Next() {
		entry, err := scanRequestLog(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, errors.Wrap(err, "iterate request_log rows")
	}

	return out, total, nil
}

// CountRequestLogs returns the number of request_log rows matching filter.
func (db *Database) CountRequestLogs(ctx context.Context, filter RequestLogFilter) (int, error) {
	query, args, err := filter.apply(
		db.GetSQLClient().Builder().
			Select("COUNT(*)").
			From("request_log"),
	).ToSql()
	if err != nil {
		return 0, errors.Wrap(err, "build count request_log query")
	}

	var count int
	if err := db.GetSQLClient().Runner().QueryRow(ctx, query, args...).Scan(&count); err != nil {
		return 0, errors.Wrap(err, "error executing %s [args: %v]", query, args)
	}

	return count, nil
}

// scanRequestLog scans one row in requestLogColumns order into a
// model.RequestLog, translating the nullable columns (provider/model,
// token/cost figures, error detail, response body - all absent when a call
// never reached a provider) via sql.Null* intermediates.
func scanRequestLog(row interface{ Scan(...any) error }) (model.RequestLog, error) {
	var (
		entry                                    model.RequestLog
		status                                   string
		provider, modelName, errorKind, errorMsg sql.NullString
		responseBody                             sql.NullString
		promptTokens, completionTokens           sql.NullInt64
		totalTokens                              sql.NullInt64
		costUSD                                  sql.NullFloat64
	)

	if err := row.Scan(
		&entry.ID, &entry.RequestID, &entry.AccountID, &entry.TeamID, &provider, &modelName,
		&entry.RequestedModel, &status, &errorKind, &errorMsg, &entry.Stream,
		&entry.RequestBody, &responseBody, &promptTokens, &completionTokens,
		&totalTokens, &costUSD, &entry.LatencyMs, &entry.CreatedAt,
	); err != nil {
		return model.RequestLog{}, errors.Wrap(err, "scan request_log row")
	}

	entry.Status = model.RequestLogStatus(status)
	if provider.Valid {
		entry.Provider = &provider.String
	}
	if modelName.Valid {
		entry.Model = &modelName.String
	}
	if errorKind.Valid {
		entry.ErrorKind = &errorKind.String
	}
	if errorMsg.Valid {
		entry.ErrorMessage = &errorMsg.String
	}
	if responseBody.Valid {
		entry.ResponseBody = &responseBody.String
	}
	if promptTokens.Valid {
		v := int(promptTokens.Int64)
		entry.PromptTokens = &v
	}
	if completionTokens.Valid {
		v := int(completionTokens.Int64)
		entry.CompletionTokens = &v
	}
	if totalTokens.Valid {
		v := int(totalTokens.Int64)
		entry.TotalTokens = &v
	}
	if costUSD.Valid {
		entry.CostUsd = &costUSD.Float64
	}

	return entry, nil
}
