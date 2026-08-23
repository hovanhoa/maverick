package db

import (
	"context"
	"encoding/base64"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/hovanhoa/llmgateway/pkg/core/errors"
)

// MaxAvatarBytes bounds how large an uploaded avatar image may be, to keep
// account_avatar from growing unbounded - generous enough for a profile
// picture, nowhere near enough for someone to use it as file storage.
const MaxAvatarBytes = 2 << 20 // 2 MiB

// Avatar is one account's stored profile picture.
type Avatar struct {
	ContentType string
	Data        []byte
}

// SetAccountAvatar stores (or replaces) accountID's avatar. data is
// base64-encoded before storage - see the migration's doc comment for why
// this table stores TEXT rather than BYTEA.
func (db *Database) SetAccountAvatar(ctx context.Context, accountID string, contentType string, data []byte) error {
	query, args, err := db.GetSQLClient().Builder().
		Insert("account_avatar").
		Columns("account_id", "content_type", "data", "updated_at").
		Values(accountID, contentType, base64.StdEncoding.EncodeToString(data), time.Now().UTC()).
		Suffix("ON CONFLICT (account_id) DO UPDATE SET content_type = EXCLUDED.content_type, data = EXCLUDED.data, updated_at = EXCLUDED.updated_at").
		ToSql()
	if err != nil {
		return errors.Wrap(err, "build set account avatar query")
	}

	if _, err := db.GetSQLClient().Runner().Exec(ctx, query, args...); err != nil {
		return errors.Wrap(err, "error executing %s [args: %v]", query, args)
	}

	return nil
}

// GetAccountAvatar returns accountID's stored avatar. A nil avatar and nil
// error means the account has none set.
func (db *Database) GetAccountAvatar(ctx context.Context, accountID string) (*Avatar, error) {
	query, args, err := db.GetSQLClient().Builder().
		Select("content_type", "data").
		From("account_avatar").
		Where(sq.Eq{"account_id": accountID}).
		Limit(1).
		ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "build get account avatar query")
	}

	rows, err := db.GetSQLClient().Runner().Query(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "error executing %s [args: %v]", query, args)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, nil
	}

	var contentType, encoded string
	if err := rows.Scan(&contentType, &encoded); err != nil {
		return nil, errors.Wrap(err, "error scanning row")
	}
	rows.Close()

	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, errors.Wrap(err, "decode stored avatar")
	}

	return &Avatar{ContentType: contentType, Data: data}, nil
}

// DeleteAccountAvatar removes accountID's avatar, if any. The bool is true
// when a row was actually removed.
func (db *Database) DeleteAccountAvatar(ctx context.Context, accountID string) (bool, error) {
	query, args, err := db.GetSQLClient().Builder().
		Delete("account_avatar").
		Where(sq.Eq{"account_id": accountID}).
		ToSql()
	if err != nil {
		return false, errors.Wrap(err, "build delete account avatar query")
	}

	tag, err := db.GetSQLClient().Runner().Exec(ctx, query, args...)
	if err != nil {
		return false, errors.Wrap(err, "error executing %s [args: %v]", query, args)
	}

	return tag.RowsAffected() > 0, nil
}
