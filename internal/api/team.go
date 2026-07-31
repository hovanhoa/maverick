package api

import (
	"context"

	"github.com/hovanhoa/llmgateway/internal/model"
)

func (r *Resolver) createTeam(ctx context.Context, name string) (*model.Team, error) {
	team := &model.Team{Name: name}
	return r.deps.Database.CreateTeam(ctx, team)
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
	return r.deps.Database.GetTeamByID(ctx, id)
}

func (r *Resolver) updateTeam(ctx context.Context, id string, name string) (*model.Team, error) {
	return r.deps.Database.UpdateTeam(ctx, id, name)
}

func (r *Resolver) deleteTeam(ctx context.Context, id string) (bool, error) {
	return r.deps.Database.DeleteTeam(ctx, id)
}
