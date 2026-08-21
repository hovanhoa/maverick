package http

import (
	"github.com/hovanhoa/llmgateway/internal/api"
	"github.com/hovanhoa/llmgateway/internal/authz"
	"github.com/hovanhoa/llmgateway/internal/db"
	"github.com/hovanhoa/llmgateway/internal/model"
	"github.com/hovanhoa/llmgateway/pkg/core/http"

	"github.com/benbjohnson/clock"
)

// Dependencies of the API service.
type Dependencies struct {
	DB *db.Database

	Clock clock.Clock
}

// Service implements the core API routes
type Service struct {
	*http.Service
	deps Dependencies
}

// NewService returns a new configured instance of the API service
func NewService(deps Dependencies) *Service {
	// Define the HTTP service with CORS enabled
	s := &Service{
		Service: http.NewService(
			http.WithCORS(),
			http.WithLogDropper(func(c *http.Context) bool {
				// Drop logs for flag requests since they are mostly noise.
				if c.FullPath() == "/flags" || c.FullPath() == "/flag/:id" {
					return true
				}

				// Drop logs for CORS requests also
				if c.Request.Method == "OPTIONS" {
					return true
				}

				return false
			}),
		),
		deps: deps,
	}

	s.setupService()
	return s
}

func (s *Service) getGraphQLRouter() http.IRouterGroup {
	// Set up the router
	graphqlRouter := s.Service.Router().Group("/graphql")

	// Authenticate the API key on every GraphQL request, then require that
	// authentication succeeded. Role-specific checks (RequireRole) live in
	// the resolvers themselves, since a single /graphql route can't be
	// gated per-operation by HTTP middleware.
	//
	// The playground itself is just a static HTML/JS page used to compose
	// queries - it isn't the GraphQL endpoint, so it's exempt. The actual
	// query it sends still goes through /graphql/query and is authenticated
	// like any other request.
	authorizer := authz.New(authz.Dependencies{Database: s.deps.DB})
	requireAuth := http.RequireAuth[model.Identity, model.Role]()
	graphqlRouter.Use(http.AuthMiddleware[model.Identity, model.Role](authorizer))
	graphqlRouter.Use(func(c *http.Context) {
		if c.Request.URL.Path == "/graphql/playground" {
			c.Next()
			return
		}
		requireAuth(c)
	})

	// Set up the loaders for the GraphQL layer.
	graphqlRouter.Use(func(c *http.Context) {
		c.Request = c.Request.WithContext(api.ContextWithLoaders(c.Request.Context(), api.NewLoaders(s.deps.DB)))
		c.Next()
	})

	return graphqlRouter
}

func (s *Service) setupService() {
	// Set up the routes
	s.Service.Router().GET("/ping", http.HandleAPIResponse(s.ping))

	// Set up GraphQL API
	api.
		NewService(api.Dependencies{
			Database: s.deps.DB,
		}).
		MountHTTP(s.getGraphQLRouter())
}
