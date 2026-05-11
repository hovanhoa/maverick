package api

import (
	"context"

	"github.com/hovanhoa/llmgateway/internal/model"
)

func (r *Resolver) createTeam(ctx context.Context, name string) (*model.Team, error) {
	team := &model.Team{Name: name}
	return r.deps.Database.CreateTeam(ctx, team)
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
