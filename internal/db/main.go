package db

import (
	"context"
	_ "embed"
	"time"

	"github.com/hovanhoa/llmgateway/pkg/core/retries"
	"github.com/hovanhoa/llmgateway/pkg/driver"
)

const (
	// StandardExpiry is a constant that defines the recommended TTL
	// for ephemeral data.
	StandardExpiry time.Duration = 60 * 24 * time.Hour

	// NeverExpiry is a constant that indicates that no TTL should be
	// applied.
	NeverExpiry time.Duration = 0
)

// Database combines a SQL and key-value store, and implements
// all sub-database types.
type Database struct {
	sqlClient driver.SQLStore
	kvClient  driver.KVStore
}

// New returns a new Database instance backed with the given SQL and
// key-value stores.
func New(sqlClient driver.SQLStore, kvClient driver.KVStore) *Database {
	return &Database{
		sqlClient: sqlClient,
		kvClient:  kvClient,
	}
}

// GetSQLClient returns the underlying SQL store client.
func (db *Database) GetSQLClient() driver.SQLStore {
	return db.sqlClient
}

// GetSQLClient returns the underlying KV store client.
func (db *Database) GetKVClient() driver.KVStore {
	return db.kvClient
}

// Close attempts to close the database connection.
// This abstraction should be preferred to closing sqlClient and kvClient manually.
func (db *Database) Close() error {
	if db.sqlClient != nil {
		err := db.sqlClient.Close()
		if err != nil {
			return err
		}
	}
	if db.kvClient != nil {
		err := db.kvClient.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// WithTransaction executes a callback function with an instance of
// Database in which each method executes in a single transaction.
func (db *Database) WithTransaction(ctx context.Context, fn func(ctx context.Context, db *Database, r *retries.RetryControl) error) error {
	return db.GetSQLClient().WithTransaction(ctx, func(ctx context.Context, s driver.SQLStore, r *retries.RetryControl) error {
		return fn(ctx, New(s, db.kvClient), r)
	})
}
