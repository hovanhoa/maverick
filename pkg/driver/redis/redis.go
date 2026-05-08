package redis

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/hovanhoa/llmgateway/pkg/core/env"
	"github.com/hovanhoa/llmgateway/pkg/core/errors"
	"github.com/hovanhoa/llmgateway/pkg/driver"

	"github.com/go-redis/redis/v8"
)

type DB int

const (
	DBDefault DB = iota

	DriverName driver.Name = "redis"
)

type (
	PubSub = redis.PubSub
)

type Config struct {
	Host string
	Port string
	Pass string
	DB   DB
}

// Database implements a key-value store backed by Redis.
type Database struct {
	client *redis.Client
	config Config
}

// New returns a new key-value database connected to the given
// Redis instance.
func New(ctx context.Context, config Config) (*Database, error) {
	var tlsConfig *tls.Config
	if env.GetEnvironment() != env.Dev {
		tlsConfig = &tls.Config{}
	}

	var user string
	if env.GetEnvironment() == env.Dev {
		user = "redis"
	}

	client := redis.NewClient(&redis.Options{
		Network:   "tcp",
		Addr:      fmt.Sprintf("%s:%s", config.Host, config.Port),
		Username:  user,
		Password:  config.Pass,
		DB:        int(config.DB),
		TLSConfig: tlsConfig,
	})
	client.AddHook(NewTracingHook(config))

	db := &Database{client, config}
	if _, err := client.Ping(ctx).Result(); err != nil {
		return nil, errors.Wrap(err, "client.Ping")
	}

	return db, nil
}

func (db *Database) Get(ctx context.Context, key string) (found bool, value string, err error) {
	res, redisErr := db.client.Get(ctx, key).Result()
	if redisErr == redis.Nil {
		found = false
		return
	}
	if redisErr != nil {
		err = errors.Wrap(redisErr, "client.Get(%q)", key)
		return
	}
	return true, res, nil
}

func (db *Database) Set(ctx context.Context, key string, value string, expiration time.Duration) error {
	_, err := db.client.Set(ctx, key, value, expiration).Result()
	if err != nil {
		return errors.Wrap(err, "client.Set(%q, %q, %q)", key, value, expiration)
	}

	return nil
}

func (db *Database) Del(ctx context.Context, key string) error {
	_, err := db.client.Del(ctx, key).Result()
	if err != nil {
		return errors.Wrap(err, "client.Del(%q)", key)
	}

	return nil
}

func (db *Database) Keys(ctx context.Context, pattern string) ([]string, error) {
	allKeys, err := db.client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, errors.Wrap(err, "client.Keys(%s)", pattern)
	}

	return allKeys, nil
}

func (db *Database) GetAndSet(ctx context.Context, fn driver.KVMapper, keys ...string) error {
	err := db.client.Watch(ctx, func(t *redis.Tx) error {
		kv := make(map[string]string)

		for _, k := range keys {
			v, redisErr := t.Get(ctx, k).Result()
			if redisErr == redis.Nil {
				continue
			}
			if redisErr != nil {
				return errors.Wrap(redisErr, "t.Get(%q)", k)
			}
			kv[k] = v
		}

		newKV, err := fn(kv)
		if err != nil {
			return err
		}

		for k, v := range newKV {
			_, err := t.Set(ctx, k, v, 60*24*time.Hour).Result()
			if err != nil {
				return errors.Wrap(err, "t.Set(%q, %q)", k, v)
			}
		}

		return nil
	}, keys...)

	if err != nil {
		return errors.Wrap(err, "client.GetAndSet(%v)", keys)
	}
	return nil
}

func (db *Database) Publish(ctx context.Context, channel string, message string) error {
	return db.client.Publish(ctx, channel, message).Err()
}

func (db *Database) Subscribe(ctx context.Context, channels ...string) *redis.PubSub {
	return db.client.Subscribe(ctx, channels...)
}

func (db Database) GetDriverName() driver.Name {
	return DriverName
}

func (db *Database) Close() error {
	return db.client.Close()
}
