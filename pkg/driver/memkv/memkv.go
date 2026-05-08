package memkv

import (
	"context"
	"regexp"
	"sync"
	"time"

	"github.com/hovanhoa/llmgateway/pkg/core/errors"
	"github.com/hovanhoa/llmgateway/pkg/driver"
)

const (
	DriverName driver.Name = "memkv"
)

// Database implements an in-memory key-value database.
type Database struct {
	db map[string]string
	mu sync.RWMutex
}

// New returns a new thread-safe in-memory key-value store.
func New() *Database {
	return &Database{
		db: make(map[string]string),
	}
}

func NewWithData(db map[string]string) *Database {
	return &Database{
		db: db,
	}
}

func (db *Database) GetData() map[string]string {
	return db.db
}

// Get the value stored for the given key
func (db *Database) Get(ctx context.Context, key string) (found bool, value string, err error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	value, found = db.db[key]
	return
}

// Set the given value for the given key.
func (db *Database) Set(ctx context.Context, key string, value string, expiration time.Duration) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	db.db[key] = value
	return nil
}

// Del deletes the stored value for the given key.
func (db *Database) Del(ctx context.Context, key string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	delete(db.db, key)
	return nil
}

// Scan scans the database
// i.e. for keys ==> match string then return all the keys
func (db *Database) Keys(ctx context.Context, pattern string) ([]string, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	var allKeys []string
	for key, value := range db.db {
		isMatch, err := regexp.MatchString(pattern, key)
		if err != nil {
			return nil, errors.Wrap(err, "regexp.MatchString(%q, %q)", pattern, key)
		}
		if isMatch {
			allKeys = append(allKeys, value)
		}
	}
	return allKeys, nil
}

// GetAndSet gets the requested keys and set the returned values. This is not
// actually atomic.
func (db *Database) GetAndSet(ctx context.Context, fn driver.KVMapper, keys ...string) error {
	kv := make(map[string]string)

	for _, k := range keys {
		found, v, _ := db.Get(ctx, k)
		if !found {
			continue
		}
		kv[k] = v
	}

	newKV, err := fn(kv)
	if err != nil {
		return err
	}

	for _, k := range keys {
		if v, ok := newKV[k]; ok {
			_ = db.Set(ctx, k, v, 0)
		}
	}

	return nil
}

func (db *Database) GetDriverName() driver.Name {
	return DriverName
}

// Close is a noop.
func (db *Database) Close() error {
	return nil
}
