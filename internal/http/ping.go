package http

import (
	"context"

	"github.com/hovanhoa/llmgateway/pkg/core/http"
)

type PingRequest struct {
}

type PingResponse struct {
	Message string
}

// ping implements the logic for the Ping endpoint
func (s *Service) ping(ctx context.Context, req *http.HandlerRequest[PingRequest]) (*http.HandlerResponse, *http.Error) {
	return http.HandlerResponseJSON(PingResponse{
		Message: "pong",
	}), nil
}
