// Command seed creates the first OWNER account and API key so there is an
// initial admin able to create everyone else. It is a one-off bootstrap
// script: it does nothing (beyond logging the existing owner) if an OWNER
// account already exists.
package main

import (
	"context"
	"flag"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/hovanhoa/llmgateway/internal/db"
	"github.com/hovanhoa/llmgateway/internal/db/migrations"
	"github.com/hovanhoa/llmgateway/internal/model"
	"github.com/hovanhoa/llmgateway/pkg/core/log"
	"github.com/hovanhoa/llmgateway/pkg/core/secrets"
	"github.com/hovanhoa/llmgateway/pkg/driver/postgres"
	"github.com/hovanhoa/llmgateway/pkg/driver/redis"
)

func main() {
	email := flag.String("email", "owner@example.com", "email for the seeded OWNER account")
	username := flag.String("username", "owner", "username for the seeded OWNER account")
	flag.Parse()

	ctx := context.Background()
	logger := log.New()

	kvStore, err := redis.New(ctx, redis.Config{
		Host: secrets.Require("REDIS_HOST"),
		Port: secrets.Require("REDIS_PORT"),
		Pass: secrets.Get("REDIS_PASS"),
		DB:   redis.DBDefault,
	})
	if err != nil {
		logger.Fatal("unable to connect to redis", log.Error(err))
	}

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

	database := db.New(sqlStore, kvStore)
	defer func() { _ = database.Close() }()

	if err := migrations.NewMigrator(sqlStore, kvStore).Up(ctx); err != nil {
		logger.Fatal("error up'ing database", log.Error(err))
	}

	ownerCount, err := database.CountAccounts(ctx, func(stmt sq.SelectBuilder) sq.SelectBuilder {
		return stmt.Where(sq.Expr("account->>'role' = ?", string(model.RoleOwner)))
	})
	if err != nil {
		logger.Fatal("error checking for existing owner accounts", log.Error(err))
	}
	if ownerCount > 0 {
		logger.Info("an OWNER account already exists, skipping seed")
		return
	}

	owner, err := database.CreateAccount(ctx, &model.Account{
		Email:    *email,
		Username: *username,
		Role:     model.RoleOwner,
	})
	if err != nil {
		logger.Fatal("error creating owner account", log.Error(err))
	}

	secret, err := database.CreateAPIKey(ctx, owner.ID)
	if err != nil {
		logger.Fatal("error creating owner api key", log.Error(err))
	}

	fmt.Println("Seeded OWNER account:")
	fmt.Printf("  Account ID: %s\n", owner.ID)
	fmt.Printf("  Email:      %s\n", owner.Email)
	fmt.Printf("  API Key:    %s\n", secret.Key)
	fmt.Println()
	fmt.Println("Store this API key now - it will not be shown again.")
}
