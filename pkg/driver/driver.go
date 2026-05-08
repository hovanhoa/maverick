package driver

import (
	"context"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/hovanhoa/llmgateway/pkg/core/retries"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Name is a type identifying the database driver. Each driver
// should define a `DriverName` of this type which is unique.
type Name string

type KVMapper func(kv map[string]string) (newKV map[string]string, err error)

// KVStore is a key-value database.
type KVStore interface {
	// Get returns the value stored for the specified key. `found` will be set to `true`
	// if the key exists (and hence a valid `value` was returned). Otherwise, `found`
	// will be set to `false`.
	Get(ctx context.Context, key string) (found bool, value string, err error)

	// Set stores the given value for the given key, overwriting any
	// existing value, and sets the given expiration value as the key TTL. The key
	// will be removed once its TTL expires.
	Set(ctx context.Context, key string, value string, expiration time.Duration) error

	// Del removes the given key and its associated value from the database. It returns
	// successfully even if the key does not exist.
	Del(ctx context.Context, key string) error

	// Scan scans the database given a pattern, count and type.
	Keys(ctx context.Context, pattern string) ([]string, error)

	// GetAndSet atomically gets the given keys and passes them as input to the KVMapper.
	// The result of the KVMapper is then set in the database atomically. An error is
	// returned if another transaction tries to modify any of the same keys.
	GetAndSet(ctx context.Context, fn KVMapper, keys ...string) error

	// GetDriverName returns an identifier naming this driver
	GetDriverName() Name

	// Close the database connection.
	Close() error
}

type SQLWithTxFunc func(context.Context, SQLStore, *retries.RetryControl) error

// SQLStore is a SQL-based database.
type SQLStore interface {
	// Runner returns a query runner instance connected to the database
	Runner() SQLRunner

	// Builder returns a SQL statement builder based on the dialect of the
	// database, configured to run on the backing database.
	Builder() sq.StatementBuilderType

	// WithTransaction creates and manages the lifecycle of a
	// transaction, allowing the callback function to run multiple
	// DB queries atomically.
	WithTransaction(context.Context, SQLWithTxFunc) error

	// GetDriverName returns an identifier naming this driver
	GetDriverName() Name

	// Close the database connection.
	Close() error
}

// SQLRunner defines the subset of the *sql.DB interface we're interested in
// interacting with.
type SQLRunner interface {
	Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, query string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, query string, args ...any) pgx.Row
}

// SerializedStore stores the information needed to recreate a driver.
type SerializedStore struct {
	DriverName Name
	Config     map[string]string
}
