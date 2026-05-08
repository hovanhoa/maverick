package http

import (
	"github.com/hovanhoa/llmgateway/internal/api"
	"github.com/hovanhoa/llmgateway/internal/db"
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
