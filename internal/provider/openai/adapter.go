// Package openai adapts OpenAI's Chat Completions API to the gateway's
// canonical request/response types. Since those types are themselves
// modeled directly on OpenAI's wire format, this adapter is close to a
// pass-through: request/response shapes match field-for-field, aside from
// error envelope mapping.
package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/hovanhoa/llmgateway/internal/provider"
	"github.com/hovanhoa/llmgateway/pkg/openai"
)

const defaultBaseURL = "https://api.openai.com"

// Config configures an Adapter.
type Config struct {
	// APIKey authenticates requests via the Authorization: Bearer header.
	APIKey string
	// BaseURL overrides the OpenAI API base URL. Used in tests to point at
	// a mock server; defaults to https://api.openai.com.
	BaseURL string
}

// Adapter implements provider.Provider for OpenAI's Chat Completions API.
type Adapter struct {
	client  *http.Client
	apiKey  string
	baseURL string
}

// New returns a new Adapter using the given HTTP client and config.
func New(client *http.Client, cfg Config) *Adapter {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Adapter{client: client, apiKey: cfg.APIKey, baseURL: baseURL}
}

// Name returns "openai".
func (a *Adapter) Name() string { return "openai" }

type errorEnvelope struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

func mapHTTPError(providerName string, status int, body []byte) error {
	var env errorEnvelope
	_ = json.Unmarshal(body, &env)
	msg := env.Error.Message
	if msg == "" {
		msg = string(body)
	}

	switch {
	case status == http.StatusUnauthorized:
		return provider.NewError(providerName, provider.ErrorKindAuth, msg, nil)
	case status == http.StatusTooManyRequests:
		return provider.NewError(providerName, provider.ErrorKindQuota, msg, nil)
	case status == http.StatusBadRequest || status == http.StatusNotFound || status == http.StatusUnprocessableEntity:
		return provider.NewError(providerName, provider.ErrorKindInvalidRequest, msg, nil)
	case status >= 500:
		return provider.NewError(providerName, provider.ErrorKindTransient, msg, nil)
	default:
		return provider.NewError(providerName, provider.ErrorKindUnknown, msg, nil)
	}
}

func classifyTransportError(providerName string, ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return provider.NewError(providerName, provider.ErrorKindTimeout, "request timed out", ctx.Err())
	}
	return provider.NewError(providerName, provider.ErrorKindTransient, "request failed", err)
}

func (a *Adapter) newRequest(ctx context.Context, req *openai.ChatCompletionRequest) (*http.Request, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, provider.NewError(a.Name(), provider.ErrorKindUnknown, "encode request", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/v1/chat/completions", bytes.NewReader(data))
	if err != nil {
		return nil, provider.NewError(a.Name(), provider.ErrorKindUnknown, "build request", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+a.apiKey)
	if req.Stream {
		httpReq.Header.Set("Accept", "text/event-stream")
	}

	return httpReq, nil
}

// ChatCompletion implements provider.Provider.
func (a *Adapter) ChatCompletion(ctx context.Context, req *openai.ChatCompletionRequest) (*openai.ChatCompletionResponse, error) {
	nonStreaming := *req
	nonStreaming.Stream = false

	httpReq, err := a.newRequest(ctx, &nonStreaming)
	if err != nil {
		return nil, err
	}

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, classifyTransportError(a.Name(), ctx, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, provider.NewError(a.Name(), provider.ErrorKindTransient, "read response", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, mapHTTPError(a.Name(), resp.StatusCode, respBody)
	}

	var out openai.ChatCompletionResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, provider.NewError(a.Name(), provider.ErrorKindUnknown, "decode response", err)
	}

	return &out, nil
}

// StreamChatCompletion implements provider.Provider.
func (a *Adapter) StreamChatCompletion(ctx context.Context, req *openai.ChatCompletionRequest) (<-chan provider.StreamEvent, error) {
	streaming := *req
	streaming.Stream = true

	httpReq, err := a.newRequest(ctx, &streaming)
	if err != nil {
		return nil, err
	}

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, classifyTransportError(a.Name(), ctx, err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, mapHTTPError(a.Name(), resp.StatusCode, respBody)
	}

	events := make(chan provider.StreamEvent)
	go a.pumpStream(ctx, resp.Body, events)

	return events, nil
}

func (a *Adapter) pumpStream(ctx context.Context, body io.ReadCloser, events chan<- provider.StreamEvent) {
	defer close(events)
	defer body.Close()

	send := func(ev provider.StreamEvent) bool {
		select {
		case events <- ev:
			return true
		case <-ctx.Done():
			return false
		}
	}

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			return
		}
		if payload == "" {
			continue
		}

		var chunk openai.ChatCompletionChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}
		if !send(provider.StreamEvent{Chunk: &chunk}) {
			return
		}
	}

	if err := scanner.Err(); err != nil {
		send(provider.StreamEvent{Err: provider.NewError(a.Name(), provider.ErrorKindTransient, "stream read failed", err)})
	}
}

var _ provider.Provider = (*Adapter)(nil)
