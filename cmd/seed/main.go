// Command seed creates the first OWNER account and API key so there is an
// initial admin able to create everyone else. It is a one-off bootstrap
// script: it does nothing (beyond logging the existing owner) if an OWNER
// account already exists.
//
// Pass -demo to additionally seed a demo team, an OWNER and MEMBER account
// on it, and a handful of sample usage_event rows - enough to see real
// numbers on the web console's Accounts/Teams/Usage pages locally. This is
// for local development only; never run -demo against production.
package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/hovanhoa/llmgateway/internal/db"
	"github.com/hovanhoa/llmgateway/internal/db/migrations"
	"github.com/hovanhoa/llmgateway/internal/model"
	"github.com/hovanhoa/llmgateway/internal/usage"
	"github.com/hovanhoa/llmgateway/pkg/core/log"
	"github.com/hovanhoa/llmgateway/pkg/core/secrets"
	"github.com/hovanhoa/llmgateway/pkg/driver/postgres"
	"github.com/hovanhoa/llmgateway/pkg/driver/redis"
)

func main() {
	email := flag.String("email", "owner@example.com", "email for the seeded OWNER account")
	username := flag.String("username", "owner", "username for the seeded OWNER account")
	demo := flag.Bool("demo", false, "also seed a demo team, a couple of member accounts, and sample usage events - for exercising the web console locally, never for production")
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
		logger.Info("an OWNER account already exists, skipping owner seed")
	} else {
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

		password, err := database.SetRandomAccountPassword(ctx, owner.ID)
		if err != nil {
			logger.Fatal("error setting owner password", log.Error(err))
		}

		fmt.Println("Seeded OWNER account:")
		fmt.Printf("  Account ID: %s\n", owner.ID)
		fmt.Printf("  Email:      %s\n", owner.Email)
		fmt.Printf("  Username:   %s\n", owner.Username)
		fmt.Printf("  Password:   %s\n", password)
		fmt.Printf("  API Key:    %s\n", secret.Key)
		fmt.Println()
		fmt.Println("Store these now - neither will be shown again. Sign in to the web console with the")
		fmt.Println("username/password at /login, or use the API key directly against the GraphQL/proxy API.")
	}

	if *demo {
		if err := seedDemoData(ctx, database, logger); err != nil {
			logger.Fatal("error seeding demo data", log.Error(err))
		}
	}
}

// demoTeamName identifies the seeded demo team, so a rerun of -demo can
// detect it already exists and skip rather than piling up duplicate teams.
const demoTeamName = "Demo Team"

// seedDemoData creates a demo team, an OWNER and a MEMBER account on it,
// and a handful of usage_event rows spread over the last few days across
// different providers/models - enough sample data to exercise the web
// console's Accounts/Teams/Usage pages locally. It is a no-op if the demo
// team already exists, so rerunning "make seed ARGS=-demo" is safe.
func seedDemoData(ctx context.Context, database *db.Database, logger *log.Logger) error {
	existing, err := database.CountTeams(ctx, func(stmt sq.SelectBuilder) sq.SelectBuilder {
		return stmt.Where(sq.Expr("team->>'name' = ?", demoTeamName))
	})
	if err != nil {
		return err
	}
	if existing > 0 {
		logger.Info("a demo team already exists, skipping demo seed")
		return nil
	}

	budget := 1_000_000
	team, err := database.CreateTeam(ctx, &model.Team{Name: demoTeamName, MonthlyTokenBudget: &budget})
	if err != nil {
		return err
	}

	owner, err := database.CreateAccount(ctx, &model.Account{
		Email: "demo-owner@example.com", Username: "demoowner", TeamID: &team.ID, Role: model.RoleOwner,
	})
	if err != nil {
		return err
	}
	member, err := database.CreateAccount(ctx, &model.Account{
		Email: "demo-member@example.com", Username: "demomember", TeamID: &team.ID, Role: model.RoleMember,
	})
	if err != nil {
		return err
	}

	calls := []struct {
		account          *model.Account
		provider, model  string
		promptTokens     int
		completionTokens int
		daysAgo          int
	}{
		{owner, "anthropic", "claude-sonnet-5", 5000, 2000, 0},
		{owner, "anthropic", "claude-haiku-4-5-20251001", 3000, 1000, 1},
		{owner, "openai", "gpt-5-mini", 1500, 500, 3},
		{member, "openai", "gpt-4o", 8000, 3000, 0},
		{member, "openai", "gpt-4o-mini", 4000, 1500, 2},
		{member, "gemini", "gemini-2.5-flash", 6000, 2000, 4},
	}
	for i, c := range calls {
		err := database.RecordUsageEvent(ctx, &model.UsageEvent{
			RequestID:        fmt.Sprintf("demo_req_%d", i),
			AccountID:        c.account.ID,
			TeamID:           &team.ID,
			Provider:         c.provider,
			Model:            c.model,
			PromptTokens:     c.promptTokens,
			CompletionTokens: c.completionTokens,
			TotalTokens:      c.promptTokens + c.completionTokens,
			CostUSD:          usage.CalculateCost(c.provider, c.model, c.promptTokens, c.completionTokens),
			CreatedAt:        time.Now().Add(-time.Duration(c.daysAgo) * 24 * time.Hour),
		})
		if err != nil {
			return err
		}
	}

	secret, err := database.CreateAPIKey(ctx, owner.ID)
	if err != nil {
		return err
	}

	ownerPassword, err := database.SetRandomAccountPassword(ctx, owner.ID)
	if err != nil {
		return err
	}
	memberPassword, err := database.SetRandomAccountPassword(ctx, member.ID)
	if err != nil {
		return err
	}

	fmt.Println("Seeded demo data:")
	fmt.Printf("  Team:            %s (%s)\n", team.Name, team.ID)
	fmt.Printf("  Owner account:   %s / %s (password: %s)\n", owner.Email, owner.Username, ownerPassword)
	fmt.Printf("  Member account:  %s / %s (password: %s)\n", member.Email, member.Username, memberPassword)
	fmt.Printf("  Owner API Key:   %s\n", secret.Key)
	fmt.Println()
	fmt.Println("Store these now - none will be shown again. Sign in to the web console with either account's")
	fmt.Println("username/password at /login, or the owner's API key directly, to see sample data on the")
	fmt.Println("Accounts, Teams, and Usage pages.")
	return nil
}
