package api

import (
	"github.com/hovanhoa/llmgateway/internal/api/generated"
	"github.com/hovanhoa/llmgateway/internal/db"
	gql "github.com/hovanhoa/llmgateway/pkg/core/graphql"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require
// here.
type Dependencies struct {
	Database *db.Database
}

type Resolver struct {
	deps Dependencies
}

func NewService(deps Dependencies) *gql.Service {
	resolver := &Resolver{deps}
	c := generated.Config{
		Resolvers: resolver,
	}

	return gql.NewService(
		generated.NewExecutableSchema(c),
	)
}
