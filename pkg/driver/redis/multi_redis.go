package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/hovanhoa/llmgateway/pkg/core/errors"
	"github.com/hovanhoa/llmgateway/pkg/driver"
)

const (
	MultiDriverName driver.Name = "multi_redis"
)

type MultiConfig struct {
	EnableRead   bool
	EnableWrite  bool
	ReadPriority int
	Config       Config
}

// MultiDatabase is a Redis implementation that manages reading and writing
// from multiple databases.
type MultiDatabase struct {
	// readers are the list of database to read from. MultiDatabase
	// will initiate a read from all database simultaneously and
	// return an error only if all reads failed.
	readers []driver.KVStore
	// writers are a list of database to write to. MultiDatabase
	// will attempt to write to all database simultaneously and
	// return an error only if all writers failed. No guarantee
	// of cross-database consistency is made.
	writers []driver.KVStore
	// configs are the list of configs used to create this database.
	// these are kept for serialization purposes
	configs []MultiConfig
}

// New returns a new key-value database connected to the given
// Redis instance.
func NewMulti(ctx context.Context, configs []MultiConfig) (*MultiDatabase, error) {
	// Sort configs by read priority so they can be inserted in the right order
	// and so that reads are performed by priority
	sort.Slice(configs, func(i, j int) bool {
		// This function returns true if configs[i] should come before configs[j]
		return configs[i].ReadPriority > configs[j].ReadPriority
	})

	mdb := MultiDatabase{configs: configs}

	for _, c := range configs {
		db, err := New(ctx, c.Config)
		if err != nil {
			return nil, errors.Wrap(err, "error connecting to %s:%s", c.Config.Host, c.Config.Port)
		}
		if c.EnableRead {
			mdb.readers = append(mdb.readers, db)
		}
		if c.EnableWrite {
			mdb.writers = append(mdb.writers, db)
		}
	}

	return &mdb, nil
}

func (db *MultiDatabase) Get(ctx context.Context, key string) (found bool, value string, err error) {
	for _, reader := range db.readers {
		found, value, err = reader.Get(ctx, key)
		if found && err == nil {
			return
		}
	}
	return false, "", err
}

func (db *MultiDatabase) Set(ctx context.Context, key string, value string, expiration time.Duration) (err error) {
	for _, writer := range db.writers {
		err = writer.Set(ctx, key, value, expiration)
	}
	return
}

func (db *MultiDatabase) Del(ctx context.Context, key string) (err error) {
	for _, writer := range db.writers {
		err = writer.Del(ctx, key)
	}
	return
}

func (db *MultiDatabase) Keys(ctx context.Context, pattern string) ([]string, error) {
	// Intentionally not implemented since this should be used in any non-migration codepaths.
	panic("not implemented")
}

func (db *MultiDatabase) GetAndSet(ctx context.Context, fn driver.KVMapper, keys ...string) (err error) {
	// Perform GetAndSet on all readers and writers
	for _, subDB := range append(db.readers, db.writers...) {
		err = subDB.GetAndSet(ctx, fn, keys...)
	}
	return
}

func (db MultiDatabase) GetDriverName() driver.Name {
	return MultiDriverName
}

func (db MultiDatabase) Dehydrate() *driver.SerializedStore {
	return &driver.SerializedStore{
		DriverName: MultiDriverName,
		Config:     db.serializeConfig(),
	}
}

func (db *MultiDatabase) Close() (err error) {
	for _, subDB := range append(db.readers, db.writers...) {
		if closeErr := subDB.Close(); closeErr != nil {
			err = closeErr
		}
	}
	return
}

func (db MultiDatabase) serializeConfig() map[string]string {
	s := make(map[string]string)

	for i, config := range db.configs {
		encoded, err := json.Marshal(config)
		if err != nil {
			panic(err)
		}

		s[fmt.Sprintf("%d", i)] = string(encoded)
	}

	return s
}

func DeserializeMultiConfig(config map[string]string) (configs []MultiConfig) {
	for _, value := range config {
		var m MultiConfig
		if err := json.Unmarshal([]byte(value), &m); err != nil {
			panic(err)
		}

		configs = append(configs, m)
	}

	return
}
