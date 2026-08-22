package redis

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/hovanhoa/llmgateway/pkg/core/env"
	"github.com/hovanhoa/llmgateway/pkg/core/errors"
	"github.com/hovanhoa/llmgateway/pkg/core/retries"
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
		// Unset otherwise (no cap at all), which lets a connection go
		// stale behind a load balancer/proxy that silently drops
		// long-lived idle TCP connections. DialTimeout/ReadTimeout/
		// WriteTimeout/PoolSize are left at the client's own defaults
		// (5s/3s/3s/10*GOMAXPROCS), which are already reasonable.
		MaxConnAge: 1 * time.Hour,
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

// getAndSetBackoff bounds how long and how many times GetAndSet retries
// after losing the optimistic lock (another client modified one of the
// watched keys between the read and the write). The interval is
// deliberately short - this is an in-process retry loop around a
// sub-millisecond round trip, not a network-call backoff - with jitter so
// concurrent retriers don't collide in lockstep.
var getAndSetBackoff = retries.NewBackoff().
	WithMaxRetries(20).
	WithInterval(time.Millisecond).
	WithMinInterval(time.Millisecond).
	WithMaxInterval(50 * time.Millisecond).
	WithMaxJitter(2 * time.Millisecond)

func (db *Database) GetAndSet(ctx context.Context, fn driver.KVMapper, keys ...string) error {
	backoff := getAndSetBackoff
	for attempt := 0; ; attempt++ {
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

			// Commands are only actually guarded by WATCH once they're
			// queued via TxPipelined and sent as a single MULTI/EXEC -
			// calling t.Set directly would execute immediately, outside
			// any transaction, and never detect the conflict.
			_, err = t.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				for k, v := range newKV {
					pipe.Set(ctx, k, v, 60*24*time.Hour)
				}
				return nil
			})
			return err
		}, keys...)

		if err == redis.TxFailedErr {
			if attempt < backoff.MaxRetries {
				if sleepErr := backoff.SleepFunc(backoff.SleepDuration(attempt)); sleepErr != nil {
					return errors.Wrap(sleepErr, "client.GetAndSet(%v): backoff sleep", keys)
				}
				continue
			}
			return errors.New("client.GetAndSet(%v): exceeded %d retries after repeated optimistic-lock conflicts", keys, backoff.MaxRetries)
		}
		if err != nil {
			return errors.Wrap(err, "client.GetAndSet(%v)", keys)
		}
		return nil
	}
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
