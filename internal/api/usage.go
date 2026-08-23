package api

import (
	"context"
	"time"

	"github.com/hovanhoa/llmgateway/internal/db"
	"github.com/hovanhoa/llmgateway/internal/model"
	"github.com/hovanhoa/llmgateway/pkg/core/errors"
)

// startOfMonthUTC returns midnight UTC on the 1st of t's month.
func startOfMonthUTC(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// sinceOrStartOfMonth returns since if given, otherwise midnight UTC on the
// 1st of the current calendar month - the default reporting window used by
// every usage query below.
func sinceOrStartOfMonth(since *time.Time) time.Time {
	if since != nil {
		return *since
	}
	return startOfMonthUTC(time.Now())
}

func toUsageSummary(s db.UsageSummary) *model.UsageSummary {
	return &model.UsageSummary{
		RequestCount:     s.RequestCount,
		PromptTokens:     s.PromptTokens,
		CompletionTokens: s.CompletionTokens,
		TotalTokens:      s.TotalTokens,
		CostUsd:          s.CostUSD,
	}
}

func (r *Resolver) teamUsage(ctx context.Context, teamID string, since *time.Time) (*model.UsageSummary, error) {
	if err := requireTeamMember(ctx, teamID); err != nil {
		return nil, err
	}

	summary, err := r.deps.Database.SumTeamUsage(ctx, teamID, sinceOrStartOfMonth(since))
	if err != nil {
		return nil, err
	}

	return toUsageSummary(summary), nil
}

// myUsage returns the currently authenticated account's own usage. Every
// authenticated principal may see their own usage, so this needs no
// further authorization beyond the auth middleware already guaranteeing a
// principal is present.
func (r *Resolver) myUsage(ctx context.Context, since *time.Time) (*model.UsageSummary, error) {
	principal := currentPrincipal(ctx)
	if principal == nil {
		return nil, errors.New("forbidden: no authenticated principal")
	}

	summary, err := r.deps.Database.SumAccountUsage(ctx, principal.ID, sinceOrStartOfMonth(since))
	if err != nil {
		return nil, err
	}

	return toUsageSummary(summary), nil
}

// accountUsage returns a single account's usage. Callable by the account
// itself, or by an OWNER/ADMIN of the account's team - the same
// self-or-team-admin gate internal/api/apikey.go uses for API key
// management on someone else's account.
func (r *Resolver) accountUsage(ctx context.Context, accountID string, since *time.Time) (*model.UsageSummary, error) {
	teamID, err := r.accountsTeamID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if err := requireSelfOrTeamRole(ctx, accountID, teamID, model.RoleOwner, model.RoleAdmin); err != nil {
		return nil, err
	}

	summary, err := r.deps.Database.SumAccountUsage(ctx, accountID, sinceOrStartOfMonth(since))
	if err != nil {
		return nil, err
	}

	return toUsageSummary(summary), nil
}

// teamUsageByAccount returns a team's usage broken down per member.
// Restricted to an OWNER/ADMIN of that team since, unlike the team-wide
// aggregate, it exposes individual members' usage and spend to each other.
func (r *Resolver) teamUsageByAccount(ctx context.Context, teamID string, since *time.Time) ([]model.AccountUsage, error) {
	if err := requireTeamRole(ctx, teamID, model.RoleOwner, model.RoleAdmin); err != nil {
		return nil, err
	}

	rows, err := r.deps.Database.TeamUsageByAccount(ctx, teamID, sinceOrStartOfMonth(since))
	if err != nil {
		return nil, err
	}

	out := make([]model.AccountUsage, 0, len(rows))
	for _, row := range rows {
		out = append(out, model.AccountUsage{
			AccountID:        row.AccountID,
			RequestCount:     row.RequestCount,
			PromptTokens:     row.PromptTokens,
			CompletionTokens: row.CompletionTokens,
			TotalTokens:      row.TotalTokens,
			CostUsd:          row.CostUSD,
		})
	}
	return out, nil
}

// teamUsageByModel returns a team's usage broken down by provider/model.
// Open to any member of the team, like teamUsage, since it doesn't expose
// any individual member's usage.
func (r *Resolver) teamUsageByModel(ctx context.Context, teamID string, since *time.Time) ([]model.ModelUsage, error) {
	if err := requireTeamMember(ctx, teamID); err != nil {
		return nil, err
	}

	rows, err := r.deps.Database.UsageByModel(ctx, teamID, sinceOrStartOfMonth(since))
	if err != nil {
		return nil, err
	}

	out := make([]model.ModelUsage, 0, len(rows))
	for _, row := range rows {
		out = append(out, model.ModelUsage{
			Provider:         row.Provider,
			Model:            row.Model,
			RequestCount:     row.RequestCount,
			PromptTokens:     row.PromptTokens,
			CompletionTokens: row.CompletionTokens,
			TotalTokens:      row.TotalTokens,
			CostUsd:          row.CostUSD,
		})
	}
	return out, nil
}

// teamUsageDaily returns a team's daily usage trend between since (default:
// start of the current calendar month) and until (default: now). Open to
// any member of the team, like teamUsage.
func (r *Resolver) teamUsageDaily(ctx context.Context, teamID string, since, until *time.Time) ([]model.DailyUsage, error) {
	if err := requireTeamMember(ctx, teamID); err != nil {
		return nil, err
	}

	to := time.Now().UTC()
	if until != nil {
		to = *until
	}

	rows, err := r.deps.Database.UsageDaily(ctx, teamID, sinceOrStartOfMonth(since), to)
	if err != nil {
		return nil, err
	}

	out := make([]model.DailyUsage, 0, len(rows))
	for _, row := range rows {
		out = append(out, model.DailyUsage{
			Date:         row.Day,
			RequestCount: row.RequestCount,
			TotalTokens:  row.TotalTokens,
			CostUsd:      row.CostUSD,
		})
	}
	return out, nil
}

// globalUsage returns platform-wide usage across every team and account.
// OWNER/ADMIN only - requireRole checks role alone, with no team scoping,
// which is exactly the "platform admin" semantics this query needs.
func (r *Resolver) globalUsage(ctx context.Context, since *time.Time) (*model.UsageSummary, error) {
	if err := requireRole(ctx, model.RoleOwner, model.RoleAdmin); err != nil {
		return nil, err
	}

	summary, err := r.deps.Database.SumGlobalUsage(ctx, sinceOrStartOfMonth(since))
	if err != nil {
		return nil, err
	}

	return toUsageSummary(summary), nil
}

// globalUsageByTeam returns platform-wide usage broken down by team.
// OWNER/ADMIN only, for the same reason as globalUsage.
func (r *Resolver) globalUsageByTeam(ctx context.Context, since *time.Time) ([]model.TeamUsage, error) {
	if err := requireRole(ctx, model.RoleOwner, model.RoleAdmin); err != nil {
		return nil, err
	}

	rows, err := r.deps.Database.GlobalUsageByTeam(ctx, sinceOrStartOfMonth(since))
	if err != nil {
		return nil, err
	}

	// Resolve display names server-side: this is the only view a caller
	// gets of teams other than their own (listTeams always scopes to the
	// caller's own team), so the client has no other query to resolve
	// teamId against a name.
	ids := make([]string, len(rows))
	for i, row := range rows {
		ids[i] = row.TeamID
	}
	teams, err := r.deps.Database.GetTeamsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	names := make(map[string]string, len(teams))
	for _, t := range teams {
		names[t.ID] = t.Name
	}

	out := make([]model.TeamUsage, 0, len(rows))
	for _, row := range rows {
		out = append(out, model.TeamUsage{
			TeamID:           row.TeamID,
			Name:             names[row.TeamID],
			RequestCount:     row.RequestCount,
			PromptTokens:     row.PromptTokens,
			CompletionTokens: row.CompletionTokens,
			TotalTokens:      row.TotalTokens,
			CostUsd:          row.CostUSD,
		})
	}
	return out, nil
}
