package db

import (
	"context"
	"encoding/json"

	sq "github.com/Masterminds/squirrel"
	"github.com/hovanhoa/llmgateway/internal/model"
	"github.com/hovanhoa/llmgateway/pkg/core/encoding"
	"github.com/hovanhoa/llmgateway/pkg/core/errors"
	"golang.org/x/crypto/bcrypt"
)

// accountPasswordLength is the number of random characters generated for a
// new account password - long enough to be secure while still short enough
// to comfortably copy-paste, matching the API key prefix convention.
const accountPasswordLength = 16

// HashPassword hashes a plaintext password for storage. Only the hash is
// ever persisted; the plaintext is shown to the caller once, at
// creation/reset time, and never stored - mirrors HashAPIKey.
func HashPassword(plaintext string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcrypt.DefaultCost)
	if err != nil {
		return "", errors.Wrap(err, "hash password")
	}
	return string(hash), nil
}

// SetAccountPassword overwrites accountID's stored password hash.
func (db *Database) SetAccountPassword(ctx context.Context, accountID string, passwordHash string) error {
	query, args, err := db.GetSQLClient().Builder().
		Update("account").
		Set("password_hash", passwordHash).
		Where(sq.Eq{"id": accountID}).
		ToSql()
	if err != nil {
		return errors.Wrap(err, "build set account password query")
	}

	if _, err := db.GetSQLClient().Runner().Exec(ctx, query, args...); err != nil {
		return errors.Wrap(err, "error executing %s [args: %v]", query, args)
	}

	return nil
}

// SetRandomAccountPassword generates a new random password for accountID,
// persists its hash, and returns the plaintext. Mirrors CreateAPIKey: the
// plaintext is never persisted and cannot be retrieved again after this
// call returns.
func (db *Database) SetRandomAccountPassword(ctx context.Context, accountID string) (string, error) {
	plaintext := encoding.NewRandomString(accountPasswordLength)

	hash, err := HashPassword(plaintext)
	if err != nil {
		return "", err
	}

	if err := db.SetAccountPassword(ctx, accountID, hash); err != nil {
		return "", err
	}

	return plaintext, nil
}

// VerifyAccountPassword returns the account matching username if password
// matches its stored hash. A nil account and nil error means the username
// doesn't exist, has no password set yet, or the password didn't match -
// callers must not distinguish these to a caller, to avoid leaking which
// one it was.
func (db *Database) VerifyAccountPassword(ctx context.Context, username string, password string) (*model.Account, error) {
	query, args, err := db.GetSQLClient().Builder().
		Select("account", "password_hash").
		From("account").
		Where(sq.Expr("account->>'username' = ?", username)).
		Limit(1).
		ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "build verify account password query")
	}

	rows, err := db.GetSQLClient().Runner().Query(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "error executing %s [args: %v]", query, args)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, nil
	}

	var accountJSON []byte
	var passwordHash *string
	if err := rows.Scan(&accountJSON, &passwordHash); err != nil {
		return nil, errors.Wrap(err, "error scanning row")
	}
	rows.Close()

	if passwordHash == nil || bcrypt.CompareHashAndPassword([]byte(*passwordHash), []byte(password)) != nil {
		return nil, nil
	}

	var account model.Account
	if err := json.Unmarshal(accountJSON, &account); err != nil {
		return nil, errors.Wrap(err, "json.Unmarshal")
	}

	return &account, nil
}
