package graphql

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/errcode"
	"github.com/99designs/gqlgen/graphql/executor"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/gorilla/websocket"
	"github.com/hovanhoa/llmgateway/pkg/core/apm"
	"github.com/hovanhoa/llmgateway/pkg/core/env"
	"github.com/hovanhoa/llmgateway/pkg/core/errors"
	"github.com/hovanhoa/llmgateway/pkg/core/graphql/model"
	"github.com/hovanhoa/llmgateway/pkg/core/http"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

// Service wraps a GraphQL schema and associated implementation and
// exposes the functionality needed to interact with the API in production.
type Service struct {
	schema graphql.ExecutableSchema
}

// NewService creates a new GraphQL service with the given schema.
func NewService(schema graphql.ExecutableSchema) *Service {
	return &Service{schema}
}

// MountHTTP allows the caller to provide an HTTP router, upon which
// this GraphQL API will be exposed.
func (s *Service) MountHTTP(router http.IRouterGroup) {
	server := handler.New(s.schema)
	server.AddTransport(transport.Websocket{
		KeepAlivePingInterval: 10 * time.Second,
		Upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")

				// Disallow empty values
				if origin == "" {
					return false
				}

				// Parse the origin URL
				originURL, err := url.Parse(origin)
				if err != nil {
					return false
				}

				// Check if the domain is allowed
				return strings.HasSuffix(originURL.Hostname(), env.GetDomain().Domain)
			},
		},
	})
	server.AddTransport(transport.Options{})
	server.AddTransport(transport.GET{})
	server.AddTransport(transport.POST{})
	server.AddTransport(MultipartForm{
		// 256MB upload limit
		MaxUploadSize: 256 << 20,
		// If file size exceeds 128MB, use disk instead of memory
		MaxMemory: 128 << 20,
	})

	server.SetQueryCache(lru.New[*ast.QueryDocument](1000))

	server.Use(extension.Introspection{})
	server.Use(extension.AutomaticPersistedQuery{
		Cache: lru.New[string](100),
	})

	// Recover from and report panics
	if env.GetEnvironment() != env.Dev {
		server.SetRecoverFunc(func(ctx context.Context, err interface{}) (userMessage error) {
			hub := apm.GetHubFromContext(ctx)
			hub.RecoverWithContext(ctx, err)

			return errors.New("An internal server error occurred: %v", err)
		})
	}

	server.SetErrorPresenter(func(ctx context.Context, err error) *gqlerror.Error {
		// Return certains errors as is
		// TODO: handle internal server errors better by reporting to Sentry and attaching the trace ID to the frontend for debugging
		if isApplicationError(err) || isValidationError(err) || isInternalServerError(err) {
			return err.(*gqlerror.Error)
		}

		// Use the default error presenter to get the graphql error
		gqlErr := graphql.DefaultErrorPresenter(ctx, err)

		// If the error is a GraphQL validation error, re-format it into our custom validation error
		if gqlErr != nil && gqlErr.Extensions != nil {
			if code, exists := gqlErr.Extensions["code"]; exists {
				if codeString, ok := code.(string); ok {
					if codeString == errcode.ValidationFailed {
						// GraphQL returns a path as a list of PathElements, which can either be a PathIndex or a PathName,
						// which are either an int or a string. We need to convert this to a list of strings.
						var path []string
						for _, element := range gqlErr.Path {
							path = append(path, fmt.Sprintf("%v", element))
						}

						gqlErr = NewValidationError(ctx, gqlErr.Message, model.ValidationErrorCodeRequired, path...)
					}
				}
			}
		}

		// For now, do the default thing and return the error
		return gqlErr
	})

	// Instrument each GraphQL operation
	server.AroundOperations(func(ctx context.Context, next graphql.OperationHandler) graphql.ResponseHandler {
		op := graphql.GetOperationContext(ctx)
		hub := apm.GetHubFromContext(ctx)
		ctx = apm.SetHubOnContext(ctx, hub)

		hub.ConfigureScope(func(scope *apm.Scope) {
			scope.SetContext("graphql", apm.Context{
				"operation_type": op.Operation.Operation,
				"operation_name": op.OperationName,
				"variables":      op.Variables,
			})
		})

		operationName := op.OperationName
		if operationName == "" {
			operationName = op.Operation.Name
		}

		var transactionName string
		if op.Operation.Operation != "" && operationName != "" {
			transactionName = string(op.Operation.Operation) + " " + operationName
		} else if op.Operation.Operation != "" {
			transactionName = string(op.Operation.Operation)
		} else {
			transactionName = "GraphQL Operation"
		}

		span := apm.StartSpan(
			ctx,
			"",
			apm.WithOpName(fmt.Sprintf("graphql.%s", op.Operation.Operation)),
			apm.WithDescription(transactionName),
			// These shouldn't be necessary, but include them anyway for now
			apm.ContinueFromHeaders(
				op.Headers.Get(apm.TraceHeader),
				op.Headers.Get(apm.BaggageHeader),
			),
		)

		span.SetData("graphql.operation.type", string(op.Operation.Operation))
		span.SetData("graphql.operation.name", operationName)

		handler := next(span.Context())

		return func(ctx context.Context) *graphql.Response {
			defer span.Finish()
			response := handler(ctx)

			span.Status = apm.SpanStatusOK
			if response != nil && len(response.Errors) > 0 {
				// TODO: report a better status here. for example, we don't want
				// validation errors to be marked as server errors
				span.Status = apm.SpanStatusInternalError

				var err error
				for _, e := range response.Errors {
					if e != nil && e.Err != nil {
						err = e.Err
						break
					}
				}

				if err != nil {
					if hub := apm.GetHubFromContext(ctx); hub != nil {
						if client, scope := hub.Client(), hub.Scope(); client != nil {
							client.CaptureException(err, &apm.EventHint{Context: ctx, OriginalException: err}, scope)
						}
					}

					http.SetRequestLogError(ctx, err)
				}
			}

			return response
		}
	})

	router.Any("/playground", http.FromHTTPHandler(playground.Handler("GraphQL playground", path.Join(router.BasePath(), "/query"))))
	router.Any("/query", setQueryContextFromHTTP(), http.FromHTTPHandler(server))
}

// LocalClient returns a GraphQL client that can be used by genqlient-
// generated queries to access this GraphQL service without traversing a network.
func (s *Service) LocalClient() Client {
	return NewLocalClientImpl(executor.New(s.schema))
}
