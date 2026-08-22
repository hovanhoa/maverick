package db

import (
	"context"
	"encoding/json"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/hovanhoa/llmgateway/internal/model"
	"github.com/hovanhoa/llmgateway/pkg/core/encoding"
	"github.com/hovanhoa/llmgateway/pkg/core/errors"
)

// CreateTeam creates and persists a new team record.
func (db *Database) CreateTeam(ctx context.Context, team *model.Team) (*model.Team, error) {
	if team.ID == "" {
		team.ID = encoding.NewRandomIdentifier("team")
	}

	now := time.Now().UTC()
	team.CreatedAt = now
	team.UpdatedAt = now

	payload, err := json.Marshal(team)
	if err != nil {
		return nil, errors.Wrap(err, "marshal team payload")
	}

	query, args, err := db.GetSQLClient().Builder().
		Insert("team").
		Columns("id", "team", "created_at", "updated_at").
		Values(team.ID, payload, team.CreatedAt, team.UpdatedAt).
		ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "build create team query")
	}

	if _, err := db.GetSQLClient().Runner().Exec(ctx, query, args...); err != nil {
		return nil, errors.Wrap(err, "error executing %s [args: %v]", query, args)
	}

	return team, nil
}

// UpdateTeam updates a team's name.
func (db *Database) UpdateTeam(ctx context.Context, id string, name string) (*model.Team, error) {
	existing, err := db.GetTeamByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, errors.New("team not found")
	}

	existing.Name = name
	existing.UpdatedAt = time.Now().UTC()

	payload, err := json.Marshal(existing)
	if err != nil {
		return nil, errors.Wrap(err, "marshal team payload")
	}

	query, args, err := db.GetSQLClient().Builder().
		Update("team").
		Set("team", payload).
		Set("updated_at", existing.UpdatedAt).
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "build update team query")
	}

	tag, err := db.GetSQLClient().Runner().Exec(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "error executing %s [args: %v]", query, args)
	}
	if tag.RowsAffected() == 0 {
		return nil, errors.New("team not found")
	}

	return existing, nil
}

// UpdateTeamModelAllowlist replaces a team's model allowlist wholesale.
func (db *Database) UpdateTeamModelAllowlist(ctx context.Context, id string, allowlist []string) (*model.Team, error) {
	existing, err := db.GetTeamByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, errors.New("team not found")
	}

	existing.ModelAllowlist = allowlist
	existing.UpdatedAt = time.Now().UTC()

	payload, err := json.Marshal(existing)
	if err != nil {
		return nil, errors.Wrap(err, "marshal team payload")
	}

	query, args, err := db.GetSQLClient().Builder().
		Update("team").
		Set("team", payload).
		Set("updated_at", existing.UpdatedAt).
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "build update team query")
	}

	tag, err := db.GetSQLClient().Runner().Exec(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "error executing %s [args: %v]", query, args)
	}
	if tag.RowsAffected() == 0 {
		return nil, errors.New("team not found")
	}

	return existing, nil
}

// UpdateTeamQuota sets or clears a team's monthly token budget. At least
// one of monthlyTokenBudget or clearMonthlyTokenBudget must be provided.
func (db *Database) UpdateTeamQuota(ctx context.Context, id string, monthlyTokenBudget *int, clearMonthlyTokenBudget *bool) (*model.Team, error) {
	clear := clearMonthlyTokenBudget != nil && *clearMonthlyTokenBudget
	if monthlyTokenBudget == nil && !clear {
		return nil, errors.New("at least one of monthlyTokenBudget or clearMonthlyTokenBudget must be provided")
	}
	if clear && monthlyTokenBudget != nil {
		return nil, errors.New("cannot set monthlyTokenBudget and clearMonthlyTokenBudget in the same request")
	}

	existing, err := db.GetTeamByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, errors.New("team not found")
	}

	if clear {
		existing.MonthlyTokenBudget = nil
	} else {
		existing.MonthlyTokenBudget = monthlyTokenBudget
	}
	existing.UpdatedAt = time.Now().UTC()

	payload, err := json.Marshal(existing)
	if err != nil {
		return nil, errors.Wrap(err, "marshal team payload")
	}

	query, args, err := db.GetSQLClient().Builder().
		Update("team").
		Set("team", payload).
		Set("updated_at", existing.UpdatedAt).
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "build update team query")
	}

	tag, err := db.GetSQLClient().Runner().Exec(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "error executing %s [args: %v]", query, args)
	}
	if tag.RowsAffected() == 0 {
		return nil, errors.New("team not found")
	}

	return existing, nil
}

// DeleteTeam removes a team by id. The bool is true when a row was deleted.
func (db *Database) DeleteTeam(ctx context.Context, id string) (bool, error) {
	query, args, err := db.GetSQLClient().Builder().
		Delete("team").
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return false, errors.Wrap(err, "build delete team query")
	}

	tag, err := db.GetSQLClient().Runner().Exec(ctx, query, args...)
	if err != nil {
		return false, errors.Wrap(err, "error executing %s [args: %v]", query, args)
	}

	return tag.RowsAffected() > 0, nil
}

// ListTeams returns a page of teams, most recently created first, along with the
// total number of teams ignoring limit and offset.
func (db *Database) ListTeams(ctx context.Context, limit int, offset int) ([]model.Team, int, error) {
	total, err := db.CountTeams(ctx, noFilter)
	if err != nil {
		return nil, 0, err
	}

	// Nothing to fetch, and Postgres would happily scan for it anyway.
	if offset >= total {
		return nil, total, nil
	}

	teams, err := db.GetTeams(ctx, func(stmt sq.SelectBuilder) sq.SelectBuilder {
		return stmt.
			OrderBy("created_at DESC", "id DESC").
			Limit(uint64(limit)).
			Offset(uint64(offset))
	})
	if err != nil {
		return nil, 0, err
	}

	return teams, total, nil
}

// CountTeams returns the number of teams matching the given predicate.
func (db *Database) CountTeams(ctx context.Context, predicate func(stmt sq.SelectBuilder) sq.SelectBuilder) (int, error) {
	query, args, _ := predicate(
		db.sqlClient.Builder().
			Select("COUNT(*)").
			From("team"),
	).ToSql()

	var count int
	if err := db.GetSQLClient().Runner().QueryRow(ctx, query, args...).Scan(&count); err != nil {
		return 0, errors.Wrap(err, "error executing %s [args: %v]", query, args)
	}

	return count, nil
}

// GetTeamByID returns a team by id. A nil team and nil error means not found.
func (db *Database) GetTeamByID(ctx context.Context, id string) (*model.Team, error) {
	return db.GetTeam(ctx, func(stmt sq.SelectBuilder) sq.SelectBuilder {
		return stmt.Where(sq.Eq{"id": id})
	})
}

// GetTeamsByIDs returns teams whose id is in the supplied set.
func (db *Database) GetTeamsByIDs(ctx context.Context, ids []string) ([]model.Team, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	return db.GetTeams(ctx, func(stmt sq.SelectBuilder) sq.SelectBuilder {
		return stmt.Where(sq.Eq{"id": ids})
	})
}

func (db *Database) GetTeam(ctx context.Context, predicate func(stmt sq.SelectBuilder) sq.SelectBuilder) (*model.Team, error) {
	teams, err := db.GetTeams(ctx, func(stmt sq.SelectBuilder) sq.SelectBuilder {
		return predicate(stmt).Limit(1)
	})
	if err != nil {
		return nil, err
	}
	if len(teams) == 0 {
		return nil, nil
	}

	return &teams[0], nil
}

func (db *Database) GetTeams(ctx context.Context, predicate func(stmt sq.SelectBuilder) sq.SelectBuilder) ([]model.Team, error) {
	query, args, _ := predicate(
		db.sqlClient.Builder().
			Select("team").
			From("team"),
	).ToSql()

	rows, err := db.GetSQLClient().Runner().Query(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "error executing %s [args: %v]", query, args)
	}

	defer rows.Close()

	var teams []model.Team
	for rows.Next() {
		var teamJSON []byte
		if err := rows.Scan(&teamJSON); err != nil {
			return nil, errors.Wrap(err, "error scanning row")
		}

		var team model.Team
		if err := json.Unmarshal(teamJSON, &team); err != nil {
			return nil, errors.Wrap(err, "json.Unmarshal")
		}

		teams = append(teams, team)
	}

	return teams, nil
}
