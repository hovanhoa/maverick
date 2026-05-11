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

// CreateAccount creates and persists a new account record.
func (db *Database) CreateAccount(ctx context.Context, account *model.Account) (*model.Account, error) {
	if account.ID == "" {
		account.ID = encoding.NewRandomIdentifier("account")
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
		Columns("id", "data", "created_at", "updated_at").
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

// UpdateAccount updates an account's email and/or username. At least one of email or username must be non-nil.
func (db *Database) UpdateAccount(ctx context.Context, id string, email *string, username *string) (*model.Account, error) {
	if email == nil && username == nil {
		return nil, errors.New("at least one of email or username must be provided")
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

	existing.UpdatedAt = time.Now().UTC()

	payload, err := json.Marshal(existing)
	if err != nil {
		return nil, errors.Wrap(err, "marshal account payload")
	}

	query, args, err := db.GetSQLClient().Builder().
		Update("account").
		Set("data", payload).
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
			Select("data").
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
