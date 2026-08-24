package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"

	"github.com/benbjohnson/clock"
	"github.com/hovanhoa/llmgateway/internal/db"
	"github.com/hovanhoa/llmgateway/internal/db/migrations"
	api "github.com/hovanhoa/llmgateway/internal/http"
	"github.com/hovanhoa/llmgateway/internal/model"
	"github.com/hovanhoa/llmgateway/internal/policy"
	"github.com/hovanhoa/llmgateway/internal/quota"
	"github.com/hovanhoa/llmgateway/pkg/core/apm"
	"github.com/hovanhoa/llmgateway/pkg/core/auth"
	"github.com/hovanhoa/llmgateway/pkg/core/log"
	"github.com/hovanhoa/llmgateway/pkg/core/secrets"
	"github.com/hovanhoa/llmgateway/pkg/driver/postgres"
	"github.com/hovanhoa/llmgateway/pkg/driver/redis"
)

func main() {
	defer apm.Flush()

	ctx := context.Background()
	logger := log.New()

	// Connect to Redis as the key-value store
	kvStore, err := redis.New(ctx, redis.Config{
		Host: secrets.Require("REDIS_HOST"),
		Port: secrets.Require("REDIS_PORT"),
		Pass: secrets.Get("REDIS_PASS"),
		DB:   redis.DBDefault,
	})
	if err != nil {
		logger.Fatal("unable to connect to redis", log.Error(err))
	}

	// Connect to postgres as the SQL store
	sqlStore, err := postgres.New(ctx, postgres.Config{
		Host: secrets.Require("DB_HOST"),
		Port: secrets.Require("DB_PORT"),
		User: secrets.Require("DB_USER"),
		Name: secrets.Require("DB_NAME"),
		Pass: secrets.Require("DB_PASS"),
	})
	if err != nil {
		logger.Fatal("unable to connect to postgres", log.Error(err))
	}

	// Create the database with the Redis and Postgres stores.
	database := db.New(sqlStore, kvStore)
	defer func() { _ = database.Close() }()

	// Perform the initial migration
	migrator := migrations.NewMigrator(sqlStore, kvStore)
	if err := migrator.Up(ctx); err != nil {
		logger.Fatal("error up'ing database", log.Error(err))
	}

	// The session JWT signing key is generated fresh per process rather than
	// loaded from configuration: a session token only needs to survive for
	// the life of the process that issued it (a restart just means signing
	// back in, same as an already-revoked API key would require). Running
	// multiple replicas, each with their own key, would mean a session is
	// only valid against the replica that issued it - swap this for a
	// persisted key pair (see pkg/core/auth.FetchJWTKeys) before scaling
	// this out horizontally.
	jwtPrivateKey, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		logger.Fatal("unable to generate session JWT signing key", log.Error(err))
	}
	tokens := auth.NewTokenService[model.Identity](auth.Dependencies{
		DB:            kvStore,
		JWTPrivateKey: jwtPrivateKey,
		JWTPublicKey:  &jwtPrivateKey.PublicKey,
	})

	// Create the API service with all of its dependencies
	service := api.NewService(api.Dependencies{
		DB:        database,
		Providers: newProviderRegistry(),
		Quota:     quota.NewChecker(kvStore),
		Policy:    policy.DefaultChain(),
		Tokens:    tokens,
		Clock:     clock.New(),
	})

	// Start the service
	if err := service.Start(); err != nil {
		logger.Fatal("unable to start service", log.Error(err))
	}

	logger.Info("service started")

	// Handle graceful termination
	if err := service.GracefulStopOnTermination(); err != nil {
		logger.Fatal("graceful termination returned error", log.Error(err))
	}
}
