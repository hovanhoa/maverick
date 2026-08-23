// Package anthropic adapts Claude's Messages API to the gateway's
// canonical, OpenAI-compatible request/response types.
package anthropic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hovanhoa/llmgateway/internal/provider"
	"github.com/hovanhoa/llmgateway/pkg/openai"
)

const defaultBaseURL = "https://api.anthropic.com"

const anthropicVersion = "2023-06-01"

// defaultMaxTokens is Anthropic's max_tokens, which - unlike OpenAI's -
// is a required field. Used when the caller doesn't specify one.
const defaultMaxTokens = 1024

// Config configures an Adapter.
type Config struct {
	// APIKey authenticates requests via the x-api-key header.
	APIKey string
	// BaseURL overrides the Anthropic API base URL. Used in tests to point
	// at a mock server; defaults to https://api.anthropic.com.
	BaseURL string
}

// Adapter implements provider.Provider for Anthropic's Claude models.
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

// Name returns "anthropic".
func (a *Adapter) Name() string { return "anthropic" }

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model         string    `json:"model"`
	MaxTokens     int       `json:"max_tokens"`
	System        string    `json:"system,omitempty"`
	Messages      []message `json:"messages"`
	Temperature   *float64  `json:"temperature,omitempty"`
	TopP          *float64  `json:"top_p,omitempty"`
	StopSequences []string  `json:"stop_sequences,omitempty"`
	Stream        bool      `json:"stream,omitempty"`
}

func toAnthropicRequest(req *openai.ChatCompletionRequest, stream bool) *chatRequest {
	var system []string
	messages := make([]message, 0, len(req.Messages))
	for _, m := range req.Messages {
		if m.Role == openai.RoleSystem {
			system = append(system, m.Content)
			continue
		}
		messages = append(messages, message{Role: string(m.Role), Content: m.Content})
	}

	maxTokens := defaultMaxTokens
	if req.MaxTokens != nil {
		maxTokens = *req.MaxTokens
	}

	return &chatRequest{
		Model:         req.Model,
		MaxTokens:     maxTokens,
		System:        strings.Join(system, "\n\n"),
		Messages:      messages,
		Temperature:   req.Temperature,
		TopP:          req.TopP,
		StopSequences: req.Stop,
		Stream:        stream,
	}
}

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type chatResponse struct {
	ID         string         `json:"id"`
	Model      string         `json:"model"`
	Content    []contentBlock `json:"content"`
	StopReason string         `json:"stop_reason"`
	Usage      usage          `json:"usage"`
}

type errorEnvelope struct {
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

func mapFinishReason(stopReason string) openai.FinishReason {
	if stopReason == "max_tokens" {
		return openai.FinishReasonLength
	}
	return openai.FinishReasonStop
}

func mapHTTPError(providerName string, status int, body []byte) error {
	var env errorEnvelope
	_ = json.Unmarshal(body, &env)
	msg := env.Error.Message
	if msg == "" {
		msg = string(body)
	}

	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
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

func (a *Adapter) newRequest(ctx context.Context, body any, stream bool) (*http.Request, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, provider.NewError(a.Name(), provider.ErrorKindUnknown, "encode request", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/v1/messages", bytes.NewReader(data))
	if err != nil {
		return nil, provider.NewError(a.Name(), provider.ErrorKindUnknown, "build request", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", a.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicVersion)
	if stream {
		httpReq.Header.Set("Accept", "text/event-stream")
	}

	return httpReq, nil
}

func classifyTransportError(providerName string, ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return provider.NewError(providerName, provider.ErrorKindTimeout, "request timed out", ctx.Err())
	}
	return provider.NewError(providerName, provider.ErrorKindTransient, "request failed", err)
}

// ChatCompletion implements provider.Provider.
func (a *Adapter) ChatCompletion(ctx context.Context, req *openai.ChatCompletionRequest) (*openai.ChatCompletionResponse, error) {
	httpReq, err := a.newRequest(ctx, toAnthropicRequest(req, false), false)
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

	var cr chatResponse
	if err := json.Unmarshal(respBody, &cr); err != nil {
		return nil, provider.NewError(a.Name(), provider.ErrorKindUnknown, "decode response", err)
	}

	var text strings.Builder
	for _, block := range cr.Content {
		if block.Type == "text" {
			text.WriteString(block.Text)
		}
	}

	return &openai.ChatCompletionResponse{
		ID:      cr.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   cr.Model,
		Choices: []openai.Choice{{
			Index:        0,
			Message:      openai.Message{Role: openai.RoleAssistant, Content: text.String()},
			FinishReason: mapFinishReason(cr.StopReason),
		}},
		Usage: openai.Usage{
			PromptTokens:     cr.Usage.InputTokens,
			CompletionTokens: cr.Usage.OutputTokens,
			TotalTokens:      cr.Usage.InputTokens + cr.Usage.OutputTokens,
		},
	}, nil
}

type sseEnvelope struct {
	Type    string `json:"type"`
	Message struct {
		ID    string `json:"id"`
		Model string `json:"model"`
		// Usage.InputTokens is the full prompt token count, known upfront
		// on message_start. Usage.OutputTokens is 0 here (generation
		// hasn't happened yet) - the real output count comes later, on
		// message_delta.
		Usage usage `json:"usage"`
	} `json:"message"`
	Delta struct {
		Text       string `json:"text"`
		StopReason string `json:"stop_reason"`
	} `json:"delta"`
	// Usage is only populated on message_delta events, and only carries
	// OutputTokens (the final completion token count) - see
	// https://docs.claude.com/en/docs/build-with-claude/streaming.
	Usage usage `json:"usage"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

// StreamChatCompletion implements provider.Provider.
func (a *Adapter) StreamChatCompletion(ctx context.Context, req *openai.ChatCompletionRequest) (<-chan provider.StreamEvent, error) {
	httpReq, err := a.newRequest(ctx, toAnthropicRequest(req, true), true)
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
	go a.pumpStream(ctx, resp.Body, req.Model, events)

	return events, nil
}

func (a *Adapter) pumpStream(ctx context.Context, body io.ReadCloser, requestedModel string, events chan<- provider.StreamEvent) {
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

	id := ""
	model := requestedModel
	inputTokens := 0

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var currentEvent string
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "event:"):
			currentEvent = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			var env sseEnvelope
			if err := json.Unmarshal([]byte(payload), &env); err != nil {
				continue
			}

			switch currentEvent {
			case "message_start":
				id = env.Message.ID
				if env.Message.Model != "" {
					model = env.Message.Model
				}
				inputTokens = env.Message.Usage.InputTokens
			case "content_block_delta":
				chunk := &openai.ChatCompletionChunk{
					ID: id, Object: "chat.completion.chunk", Created: time.Now().Unix(), Model: model,
					Choices: []openai.ChunkChoice{{Index: 0, Delta: openai.Delta{Content: env.Delta.Text}}},
				}
				if !send(provider.StreamEvent{Chunk: chunk}) {
					return
				}
			case "message_delta":
				if env.Delta.StopReason == "" {
					continue
				}
				finish := mapFinishReason(env.Delta.StopReason)
				chunk := &openai.ChatCompletionChunk{
					ID: id, Object: "chat.completion.chunk", Created: time.Now().Unix(), Model: model,
					Choices: []openai.ChunkChoice{{Index: 0, Delta: openai.Delta{}, FinishReason: &finish}},
				}
				usage := &openai.Usage{
					PromptTokens:     inputTokens,
					CompletionTokens: env.Usage.OutputTokens,
					TotalTokens:      inputTokens + env.Usage.OutputTokens,
				}
				if !send(provider.StreamEvent{Chunk: chunk, Usage: usage}) {
					return
				}
			case "error":
				send(provider.StreamEvent{Err: provider.NewError(a.Name(), provider.ErrorKindTransient, env.Error.Message, nil)})
				return
			}
		}
	}

	if err := scanner.Err(); err != nil {
		send(provider.StreamEvent{Err: provider.NewError(a.Name(), provider.ErrorKindTransient, "stream read failed", err)})
	}
}

var _ provider.Provider = (*Adapter)(nil)
