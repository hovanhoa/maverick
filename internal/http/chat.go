package http

import (
	"encoding/json"
	"net/http"

	"github.com/hovanhoa/llmgateway/internal/model"
	"github.com/hovanhoa/llmgateway/internal/proxy"
	"github.com/hovanhoa/llmgateway/pkg/core/auth"
	corehttp "github.com/hovanhoa/llmgateway/pkg/core/http"
	"github.com/hovanhoa/llmgateway/pkg/openai"
)

// chatCompletions implements the OpenAI-compatible POST /v1/chat/completions
// endpoint. RequireAuth (wired ahead of this route) guarantees a principal
// is present in the request context.
func (s *Service) chatCompletions(c *corehttp.Context) {
	var req openai.ChatCompletionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, openai.NewErrorResponse(openai.ErrorTypeInvalidRequest, "invalid request body: "+err.Error()))
		return
	}

	principal := auth.GetPrincipal[model.Identity, model.Role](c.Request.Context())
	if principal == nil {
		c.JSON(http.StatusUnauthorized, openai.NewErrorResponse(openai.ErrorTypeAuthentication, "authentication required"))
		return
	}

	if req.Stream {
		s.streamChatCompletions(c, principal, &req)
		return
	}

	resp, err := s.proxyHandler.ChatCompletion(c.Request.Context(), principal, &req)
	if err != nil {
		status, body := proxy.ErrorResponseFor(err)
		c.JSON(status, body)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (s *Service) streamChatCompletions(c *corehttp.Context, principal *auth.Principal[model.Identity, model.Role], req *openai.ChatCompletionRequest) {
	events, err := s.proxyHandler.StreamChatCompletion(c.Request.Context(), principal, req)
	if err != nil {
		status, body := proxy.ErrorResponseFor(err)
		c.JSON(status, body)
		return
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.WriteHeader(http.StatusOK)

	flusher, canFlush := c.Writer.(http.Flusher)
	writeEvent := func(v any) {
		data, err := json.Marshal(v)
		if err != nil {
			return
		}
		_, _ = c.Writer.Write([]byte("data: " + string(data) + "\n\n"))
		if canFlush {
			flusher.Flush()
		}
	}

	for ev := range events {
		if ev.Err != nil {
			_, body := proxy.ErrorResponseFor(ev.Err)
			writeEvent(body)
			return
		}
		writeEvent(ev.Chunk)
	}

	_, _ = c.Writer.Write([]byte("data: [DONE]\n\n"))
	if canFlush {
		flusher.Flush()
	}
}
