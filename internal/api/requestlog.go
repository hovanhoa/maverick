package api

import (
	"context"

	"github.com/hovanhoa/llmgateway/internal/db"
	"github.com/hovanhoa/llmgateway/internal/model"
	"github.com/hovanhoa/llmgateway/pkg/core/errors"
)

func toRequestLogConnection(logs []model.RequestLog, total int, offset, limit int) *model.RequestLogConnection {
	return &model.RequestLogConnection{
		Items:       logs,
		TotalCount:  total,
		HasNextPage: offset+len(logs) < total && len(logs) == limit,
	}
}

// teamRequestLogs returns a team's request log, most recent first.
// Restricted to an OWNER/ADMIN of that team: unlike teamUsage's aggregate
// numbers, this exposes every member's full raw prompt/response content,
// so it needs the stricter tier teamUsageByAccount already uses, not
// teamUsage's open one.
func (r *Resolver) teamRequestLogs(ctx context.Context, teamID string, limit, offset *int) (*model.RequestLogConnection, error) {
	if err := requireTeamRole(ctx, teamID, model.RoleOwner, model.RoleAdmin); err != nil {
		return nil, err
	}

	resolvedLimit, resolvedOffset, err := resolvePage(limit, offset)
	if err != nil {
		return nil, err
	}

	logs, total, err := r.deps.Database.ListRequestLogs(ctx, db.RequestLogFilter{TeamID: &teamID}, resolvedLimit, resolvedOffset)
	if err != nil {
		return nil, err
	}

	return toRequestLogConnection(logs, total, resolvedOffset, resolvedLimit), nil
}

// myRequestLogs returns the currently authenticated account's own request
// log, most recent first. Every authenticated principal may see their own
// call history, so this needs no further authorization.
func (r *Resolver) myRequestLogs(ctx context.Context, limit, offset *int) (*model.RequestLogConnection, error) {
	principal := currentPrincipal(ctx)
	if principal == nil {
		return nil, errors.New("forbidden: no authenticated principal")
	}

	resolvedLimit, resolvedOffset, err := resolvePage(limit, offset)
	if err != nil {
		return nil, err
	}

	logs, total, err := r.deps.Database.ListRequestLogs(ctx, db.RequestLogFilter{AccountID: &principal.ID}, resolvedLimit, resolvedOffset)
	if err != nil {
		return nil, err
	}

	return toRequestLogConnection(logs, total, resolvedOffset, resolvedLimit), nil
}

// globalRequestLogs returns the platform-wide request log across every
// team and account. OWNER/ADMIN only, same as globalUsage.
func (r *Resolver) globalRequestLogs(ctx context.Context, limit, offset *int) (*model.RequestLogConnection, error) {
	if err := requireRole(ctx, model.RoleOwner, model.RoleAdmin); err != nil {
		return nil, err
	}

	resolvedLimit, resolvedOffset, err := resolvePage(limit, offset)
	if err != nil {
		return nil, err
	}

	logs, total, err := r.deps.Database.ListRequestLogs(ctx, db.RequestLogFilter{}, resolvedLimit, resolvedOffset)
	if err != nil {
		return nil, err
	}

	return toRequestLogConnection(logs, total, resolvedOffset, resolvedLimit), nil
}
