package graphql

import (
	"context"
	"encoding/json"

	gengql "github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/executor"
	"github.com/Khan/genqlient/graphql"
	"github.com/hovanhoa/llmgateway/pkg/core/env"
	"github.com/hovanhoa/llmgateway/pkg/core/http"
)

// Client is an interface that queries a GraphQL API.
type Client = graphql.Client

// localClientImpl implements a GraphQL client that directly
// executes a request against a given GraphQL API without a
// network round trip.
type localClientImpl struct {
	executor *executor.Executor
}

// NewLocalClientImpl creates a new localClientImpl with the
// given GraphQL API to query against.
func NewLocalClientImpl(executor *executor.Executor) Client {
	return &localClientImpl{
		executor: executor,
	}
}

// NewRemoteClient connects to the given GraphQL endpoint over the network
// and can be used by genqlient-generated queries to access APIs hosted at this endpoint.
func NewRemoteClient(service *env.Service, endpoint string) Client {
	return graphql.NewClient(service.InternalURL()+endpoint, http.NewServiceClient(service))
}

// MakeRequest implements the logic to directly execute a request against
// the configured GraphQL API.
func (c *localClientImpl) MakeRequest(ctx context.Context, req *graphql.Request, resp *graphql.Response) error {
	// Create a GraphQL server context with tracing. This is actually not necessary
	// for our use case, but the gqlgen library isn't super well written so it panics
	// if the tracing keys are not set up on the context, even though we don't need it.
	ctx = gengql.StartOperationTrace(ctx)

	// Set up the request and perform prevalidation.
	rc, opErr := c.executor.CreateOperationContext(ctx, &gengql.RawParams{
		OperationName: req.OpName,
		Query:         req.Query,
		Variables:     convertVariables(req.Variables),
	})
	if opErr != nil {
		return handleResponse(
			c.executor.DispatchError(gengql.WithOperationContext(ctx, rc), opErr),
			resp,
		)
	}

	// Execute the actual request.
	responses, ctx := c.executor.DispatchOperation(ctx, rc)
	return handleResponse(responses(ctx), resp)
}

// convertVariables converts the GraphQL variables passed in by the GraphQL client
// into a form the GraphQL server understands. Unfortunately, this requires a round
// trip through a JSON marshaller, again, due to the poor interfaces of both libraries.
func convertVariables(clientVariables interface{}) map[string]interface{} {
	if clientVariables == nil {
		return nil
	}

	data, err := json.Marshal(clientVariables)
	if err != nil {
		panic(err)
	}

	convertedVariables := make(map[string]interface{})
	if err := json.Unmarshal(data, &convertedVariables); err != nil {
		panic(err)
	}

	return convertedVariables
}

// handleResponse returns the GraphQL errors, if any, or otherwise the typed JSON
// response from the server. This could again be more efficient if we didn't already
// receive a JSON-marshalled blob from the query executor.
func handleResponse(graphResponse *gengql.Response, clientResponse *graphql.Response) error {
	if len(graphResponse.Errors) > 0 {
		return graphResponse.Errors
	}

	if err := json.Unmarshal(graphResponse.Data, &clientResponse.Data); err != nil {
		panic(err)
	}

	return nil
}
