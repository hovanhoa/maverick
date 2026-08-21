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

// CreateAccount creates and persists a new account record. Role defaults to
// MEMBER when not set.
func (db *Database) CreateAccount(ctx context.Context, account *model.Account) (*model.Account, error) {
	if account.ID == "" {
		account.ID = encoding.NewRandomIdentifier("account")
	}

	if account.Role == "" {
		account.Role = model.RoleMember
	} else if !account.Role.IsValid() {
		return nil, errors.New("invalid role %q", account.Role)
	}

	if account.TeamID != nil && *account.TeamID != "" {
		team, err := db.GetTeamByID(ctx, *account.TeamID)
		if err != nil {
			return nil, err
		}
		if team == nil {
			return nil, errors.New("team not found")
		}
	}

	now := time.Now().UTC()
	account.CreatedAt = now
	account.UpdatedAt = now

	payload, err := json.Marshal(account)
	if err != nil {
		return nil, errors.Wrap(err, "marshal account payload")
	}

	query, args, err := db.GetSQLClient().Builder().
		Insert("account").
		Columns("id", "account", "created_at", "updated_at").
		Values(account.ID, payload, account.CreatedAt, account.UpdatedAt).
		ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "build create account query")
	}

	if _, err := db.GetSQLClient().Runner().Exec(ctx, query, args...); err != nil {
		return nil, errors.Wrap(err, "error executing %s [args: %v]", query, args)
	}

	return account, nil
}

// UpdateAccount updates an account's email, username, team assignment, and/or role.
// At least one of email, username, teamId, clearTeamId, or role must be provided for a meaningful update.
func (db *Database) UpdateAccount(ctx context.Context, id string, email *string, username *string, teamID *string, clearTeamID *bool, role *model.Role) (*model.Account, error) {
	clear := clearTeamID != nil && *clearTeamID
	hasTeamID := teamID != nil && *teamID != ""
	if email == nil && username == nil && !hasTeamID && !clear && role == nil {
		return nil, errors.New("at least one of email, username, teamId, clearTeamId, or role must be provided")
	}
	if clear && hasTeamID {
		return nil, errors.New("cannot set teamId and clearTeamId in the same request")
	}
	if role != nil && !role.IsValid() {
		return nil, errors.New("invalid role %q", *role)
	}

	existing, err := db.GetAccountByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, errors.New("account not found")
	}

	if email != nil {
		existing.Email = *email
	}
	if username != nil {
		existing.Username = *username
	}
	if role != nil {
		existing.Role = *role
	}
	if clear {
		existing.TeamID = nil
	} else if hasTeamID {
		team, err := db.GetTeamByID(ctx, *teamID)
		if err != nil {
			return nil, err
		}
		if team == nil {
			return nil, errors.New("team not found")
		}
		tid := *teamID
		existing.TeamID = &tid
	}

	existing.UpdatedAt = time.Now().UTC()

	payload, err := json.Marshal(existing)
	if err != nil {
		return nil, errors.Wrap(err, "marshal account payload")
	}

	query, args, err := db.GetSQLClient().Builder().
		Update("account").
		Set("account", payload).
		Set("updated_at", existing.UpdatedAt).
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "build update account query")
	}

	tag, err := db.GetSQLClient().Runner().Exec(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "error executing %s [args: %v]", query, args)
	}
	if tag.RowsAffected() == 0 {
		return nil, errors.New("account not found")
	}

	return existing, nil
}

// DeleteAccount removes an account by id. The bool is true when a row was deleted.
func (db *Database) DeleteAccount(ctx context.Context, id string) (bool, error) {
	query, args, err := db.GetSQLClient().Builder().
		Delete("account").
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return false, errors.Wrap(err, "build delete account query")
	}

	tag, err := db.GetSQLClient().Runner().Exec(ctx, query, args...)
	if err != nil {
		return false, errors.Wrap(err, "error executing %s [args: %v]", query, args)
	}

	return tag.RowsAffected() > 0, nil
}

// accountTeamFilter restricts a query to accounts assigned to teamID. A nil or
// empty teamID matches every account. The list and count halves of a paginated
// query must share this so totalCount agrees with items.
func accountTeamFilter(teamID *string) func(stmt sq.SelectBuilder) sq.SelectBuilder {
	return func(stmt sq.SelectBuilder) sq.SelectBuilder {
		if teamID != nil && *teamID != "" {
			stmt = stmt.Where(sq.Expr("account->>'teamId' = ?", *teamID))
		}
		return stmt
	}
}

// ListAccounts returns a page of accounts, most recently created first, along
// with the total number of matching accounts ignoring limit and offset. When
// teamID is non-nil only accounts assigned to that team are considered.
func (db *Database) ListAccounts(ctx context.Context, teamID *string, limit int, offset int) ([]model.Account, int, error) {
	filter := accountTeamFilter(teamID)

	total, err := db.CountAccounts(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	// Nothing to fetch, and Postgres would happily scan for it anyway.
	if offset >= total {
		return nil, total, nil
	}

	accounts, err := db.GetAccounts(ctx, func(stmt sq.SelectBuilder) sq.SelectBuilder {
		return filter(stmt).
			OrderBy("created_at DESC", "id DESC").
			Limit(uint64(limit)).
			Offset(uint64(offset))
	})
	if err != nil {
		return nil, 0, err
	}

	return accounts, total, nil
}

// CountAccounts returns the number of accounts matching the given predicate.
func (db *Database) CountAccounts(ctx context.Context, predicate func(stmt sq.SelectBuilder) sq.SelectBuilder) (int, error) {
	query, args, _ := predicate(
		db.sqlClient.Builder().
			Select("COUNT(*)").
			From("account"),
	).ToSql()

	var count int
	if err := db.GetSQLClient().Runner().QueryRow(ctx, query, args...).Scan(&count); err != nil {
		return 0, errors.Wrap(err, "error executing %s [args: %v]", query, args)
	}

	return count, nil
}

// GetAccountByID returns an account by id. A nil account and nil error means not found.
func (db *Database) GetAccountByID(ctx context.Context, id string) (*model.Account, error) {
	return db.GetAccount(ctx, func(stmt sq.SelectBuilder) sq.SelectBuilder {
		return stmt.Where(sq.Eq{"id": id})
	})
}

// GetAccountsByIDs returns accounts whose id is in the supplied set.
func (db *Database) GetAccountsByIDs(ctx context.Context, ids []string) ([]model.Account, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	return db.GetAccounts(ctx, func(stmt sq.SelectBuilder) sq.SelectBuilder {
		return stmt.Where(sq.Eq{"id": ids})
	})
}

func (db *Database) GetAccount(ctx context.Context, predicate func(stmt sq.SelectBuilder) sq.SelectBuilder) (*model.Account, error) {
	accounts, err := db.GetAccounts(ctx, func(stmt sq.SelectBuilder) sq.SelectBuilder {
		return predicate(stmt).Limit(1)
	})
	if err != nil {
		return nil, err
	}
	if len(accounts) == 0 {
		return nil, nil
	}

	return &accounts[0], nil
}

func (db *Database) GetAccounts(ctx context.Context, predicate func(stmt sq.SelectBuilder) sq.SelectBuilder) ([]model.Account, error) {
	query, args, _ := predicate(
		db.sqlClient.Builder().
			Select("account").
			From("account"),
	).ToSql()

	rows, err := db.GetSQLClient().Runner().Query(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "error executing %s [args: %v]", query, args)
	}

	defer rows.Close()

	var accounts []model.Account
	for rows.Next() {
		var accountJSON []byte
		if err := rows.Scan(&accountJSON); err != nil {
			return nil, errors.Wrap(err, "error scanning row")
		}

		var account model.Account
		if err := json.Unmarshal(accountJSON, &account); err != nil {
			return nil, errors.Wrap(err, "json.Unmarshal")
		}

		accounts = append(accounts, account)
	}

	return accounts, nil
}
