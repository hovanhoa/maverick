package db

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/hovanhoa/llmgateway/internal/model"
	"github.com/hovanhoa/llmgateway/pkg/core/encoding"
	"github.com/hovanhoa/llmgateway/pkg/core/errors"
)

// apiKeySecretLength is the number of random bytes used to generate the
// plaintext portion of an API key.
const apiKeySecretLength = 24

// apiKeyPrefixLength is the number of characters of the plaintext secret
// stored unhashed, so a key can be recognized by its owner without ever
// storing or displaying the full secret.
const apiKeyPrefixLength = 8

// HashAPIKey returns the SHA-256 hash of a plaintext API key, hex-encoded.
// Only the hash is ever persisted; the plaintext is returned to the caller
// once, at creation time, and never stored.
func HashAPIKey(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

// CreateAPIKey generates a new API key for the given account, persists its
// hash, and returns the metadata plus the plaintext secret. The plaintext is
// never persisted and cannot be retrieved again after this call returns.
func (db *Database) CreateAPIKey(ctx context.Context, accountID string) (*model.APIKeySecret, error) {
	account, err := db.GetAccountByID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, errors.New("account not found")
	}

	plaintext := encoding.NewRandomIdentifierWithLength("llmgw", apiKeySecretLength)

	apiKey := model.APIKey{
		ID:        encoding.NewRandomIdentifier("apikey"),
		AccountID: accountID,
		Prefix:    plaintext[:apiKeyPrefixLength],
		CreatedAt: time.Now().UTC(),
	}

	query, args, err := db.GetSQLClient().Builder().
		Insert("api_key").
		Columns("id", "account_id", "key_hash", "prefix", "created_at").
		Values(apiKey.ID, apiKey.AccountID, HashAPIKey(plaintext), apiKey.Prefix, apiKey.CreatedAt).
		ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "build create api_key query")
	}

	if _, err := db.GetSQLClient().Runner().Exec(ctx, query, args...); err != nil {
		return nil, errors.Wrap(err, "error executing %s [args: %v]", query, args)
	}

	return &model.APIKeySecret{
		APIKey: apiKey,
		Key:    plaintext,
	}, nil
}

// GetAccountByAPIKeyHash returns the account associated with a non-revoked
// API key matching the given hash. A nil account and nil error means no
// matching, non-revoked key was found.
func (db *Database) GetAccountByAPIKeyHash(ctx context.Context, hash string) (*model.Account, error) {
	query, args, err := db.GetSQLClient().Builder().
		Select("account_id").
		From("api_key").
		Where(sq.Eq{"key_hash": hash}).
		Where(sq.Eq{"revoked_at": nil}).
		Limit(1).
		ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "build get api_key by hash query")
	}

	rows, err := db.GetSQLClient().Runner().Query(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "error executing %s [args: %v]", query, args)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, nil
	}

	var accountID string
	if err := rows.Scan(&accountID); err != nil {
		return nil, errors.Wrap(err, "error scanning row")
	}
	rows.Close()

	return db.GetAccountByID(ctx, accountID)
}

// ListAPIKeysByAccount returns metadata for every API key issued to an
// account, most recently created first. Plaintext secrets are never
// returned since they are not persisted.
func (db *Database) ListAPIKeysByAccount(ctx context.Context, accountID string) ([]model.APIKey, error) {
	query, args, err := db.GetSQLClient().Builder().
		Select("id", "account_id", "prefix", "created_at", "revoked_at").
		From("api_key").
		Where(sq.Eq{"account_id": accountID}).
		OrderBy("created_at DESC", "id DESC").
		ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "build list api_key query")
	}

	rows, err := db.GetSQLClient().Runner().Query(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "error executing %s [args: %v]", query, args)
	}
	defer rows.Close()

	var apiKeys []model.APIKey
	for rows.Next() {
		var apiKey model.APIKey
		if err := rows.Scan(&apiKey.ID, &apiKey.AccountID, &apiKey.Prefix, &apiKey.CreatedAt, &apiKey.RevokedAt); err != nil {
			return nil, errors.Wrap(err, "error scanning row")
		}
		apiKeys = append(apiKeys, apiKey)
	}

	return apiKeys, nil
}

// GetAPIKeyByID returns an API key's metadata by id. A nil key and nil error
// means not found.
func (db *Database) GetAPIKeyByID(ctx context.Context, id string) (*model.APIKey, error) {
	query, args, err := db.GetSQLClient().Builder().
		Select("id", "account_id", "prefix", "created_at", "revoked_at").
		From("api_key").
		Where(sq.Eq{"id": id}).
		Limit(1).
		ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "build get api_key query")
	}

	rows, err := db.GetSQLClient().Runner().Query(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "error executing %s [args: %v]", query, args)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, nil
	}

	var apiKey model.APIKey
	if err := rows.Scan(&apiKey.ID, &apiKey.AccountID, &apiKey.Prefix, &apiKey.CreatedAt, &apiKey.RevokedAt); err != nil {
		return nil, errors.Wrap(err, "error scanning row")
	}

	return &apiKey, nil
}

// RevokeAPIKey marks an API key as revoked. The bool is true when a
// previously non-revoked key was found and revoked.
func (db *Database) RevokeAPIKey(ctx context.Context, id string) (bool, error) {
	query, args, err := db.GetSQLClient().Builder().
		Update("api_key").
		Set("revoked_at", time.Now().UTC()).
		Where(sq.Eq{"id": id}).
		Where(sq.Eq{"revoked_at": nil}).
		ToSql()
	if err != nil {
		return false, errors.Wrap(err, "build revoke api_key query")
	}

	tag, err := db.GetSQLClient().Runner().Exec(ctx, query, args...)
	if err != nil {
		return false, errors.Wrap(err, "error executing %s [args: %v]", query, args)
	}

	return tag.RowsAffected() > 0, nil
}
