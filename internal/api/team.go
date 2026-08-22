package api

import (
	"context"

	"github.com/hovanhoa/llmgateway/internal/model"
	"github.com/hovanhoa/llmgateway/pkg/core/errors"
)

func (r *Resolver) createTeam(ctx context.Context, name string) (*model.Team, error) {
	if err := requireRole(ctx, model.RoleOwner, model.RoleAdmin); err != nil {
		return nil, err
	}

	team := &model.Team{Name: name}
	team, err := r.deps.Database.CreateTeam(ctx, team)
	if err != nil {
		return nil, err
	}

	// The team creator is auto-assigned OWNER for the team it created. This
	// only applies to callers backed by a real account; RBAC guarantees
	// that's always true outside of tests.
	if principal := currentPrincipal(ctx); principal != nil {
		caller, err := r.deps.Database.GetAccountByID(ctx, principal.ID)
		if err != nil {
			return nil, err
		}
		if caller != nil {
			ownerRole := model.RoleOwner
			if _, err := r.deps.Database.UpdateAccount(ctx, principal.ID, nil, nil, &team.ID, nil, &ownerRole); err != nil {
				return nil, err
			}
		}
	}

	return team, nil
}

func (r *Resolver) listTeams(ctx context.Context, limit *int, offset *int) (*model.TeamConnection, error) {
	resolvedLimit, resolvedOffset, err := resolvePage(limit, offset)
	if err != nil {
		return nil, err
	}

	teams, total, err := r.deps.Database.ListTeams(ctx, resolvedLimit, resolvedOffset)
	if err != nil {
		return nil, err
	}

	return &model.TeamConnection{
		Items:       teams,
		TotalCount:  total,
		HasNextPage: hasNextPage(resolvedOffset, len(teams), total),
	}, nil
}

func (r *Resolver) getTeam(ctx context.Context, id string) (*model.Team, error) {
	if err := requireTeamMember(ctx, id); err != nil {
		return nil, err
	}
	return r.deps.Database.GetTeamByID(ctx, id)
}

func (r *Resolver) updateTeam(ctx context.Context, id string, name string) (*model.Team, error) {
	if err := requireTeamRole(ctx, id, model.RoleOwner, model.RoleAdmin); err != nil {
		return nil, err
	}
	return r.deps.Database.UpdateTeam(ctx, id, name)
}

func (r *Resolver) deleteTeam(ctx context.Context, id string) (bool, error) {
	if err := requireTeamRole(ctx, id, model.RoleOwner, model.RoleAdmin); err != nil {
		return false, err
	}
	return r.deps.Database.DeleteTeam(ctx, id)
}

func (r *Resolver) updateTeamModelAllowlist(ctx context.Context, teamID string, allowlist []string) (*model.Team, error) {
	if err := requireTeamRole(ctx, teamID, model.RoleOwner, model.RoleAdmin); err != nil {
		return nil, err
	}
	return r.deps.Database.UpdateTeamModelAllowlist(ctx, teamID, allowlist)
}

func (r *Resolver) isModelAllowed(ctx context.Context, teamID string, provider string, modelName string) (bool, error) {
	if err := requireTeamMember(ctx, teamID); err != nil {
		return false, err
	}
	team, err := r.deps.Database.GetTeamByID(ctx, teamID)
	if err != nil {
		return false, err
	}
	if team == nil {
		return false, errors.New("team not found")
	}
	return team.IsModelAllowed(provider, modelName), nil
}

func (r *Resolver) updateTeamQuota(ctx context.Context, teamID string, monthlyTokenBudget *int, clearMonthlyTokenBudget *bool) (*model.Team, error) {
	if err := requireTeamRole(ctx, teamID, model.RoleOwner, model.RoleAdmin); err != nil {
		return nil, err
	}
	return r.deps.Database.UpdateTeamQuota(ctx, teamID, monthlyTokenBudget, clearMonthlyTokenBudget)
}
