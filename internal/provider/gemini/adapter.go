// Package gemini adapts Google's Generative Language API
// (generateContent/streamGenerateContent) to the gateway's canonical,
// OpenAI-compatible request/response types.
package gemini

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hovanhoa/llmgateway/internal/provider"
	"github.com/hovanhoa/llmgateway/pkg/openai"
)

const defaultBaseURL = "https://generativelanguage.googleapis.com"

// Config configures an Adapter.
type Config struct {
	// APIKey authenticates requests via the ?key= query parameter.
	APIKey string
	// BaseURL overrides the Gemini API base URL. Used in tests to point at
	// a mock server; defaults to https://generativelanguage.googleapis.com.
	BaseURL string
}

// Adapter implements provider.Provider for Google's Gemini models.
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

// Name returns "gemini".
func (a *Adapter) Name() string { return "gemini" }

type part struct {
	Text string `json:"text"`
}

type content struct {
	Role  string `json:"role,omitempty"`
	Parts []part `json:"parts"`
}

type generationConfig struct {
	Temperature     *float64 `json:"temperature,omitempty"`
	TopP            *float64 `json:"topP,omitempty"`
	MaxOutputTokens *int     `json:"maxOutputTokens,omitempty"`
	StopSequences   []string `json:"stopSequences,omitempty"`
}

type generateRequest struct {
	Contents          []content         `json:"contents"`
	SystemInstruction *content          `json:"systemInstruction,omitempty"`
	GenerationConfig  *generationConfig `json:"generationConfig,omitempty"`
}

// geminiRole maps a canonical role to Gemini's "user"/"model" vocabulary.
func geminiRole(role openai.Role) string {
	if role == openai.RoleAssistant {
		return "model"
	}
	return "user"
}

func toGeminiRequest(req *openai.ChatCompletionRequest) *generateRequest {
	var system *content
	contents := make([]content, 0, len(req.Messages))
	for _, m := range req.Messages {
		if m.Role == openai.RoleSystem {
			system = &content{Parts: []part{{Text: m.Content}}}
			continue
		}
		contents = append(contents, content{Role: geminiRole(m.Role), Parts: []part{{Text: m.Content}}})
	}

	var cfg *generationConfig
	if req.Temperature != nil || req.TopP != nil || req.MaxTokens != nil || len(req.Stop) > 0 {
		cfg = &generationConfig{
			Temperature:     req.Temperature,
			TopP:            req.TopP,
			MaxOutputTokens: req.MaxTokens,
			StopSequences:   req.Stop,
		}
	}

	return &generateRequest{Contents: contents, SystemInstruction: system, GenerationConfig: cfg}
}

type candidate struct {
	Content struct {
		Parts []part `json:"parts"`
		Role  string `json:"role"`
	} `json:"content"`
	FinishReason string `json:"finishReason"`
}

type usageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

type generateResponse struct {
	Candidates    []candidate   `json:"candidates"`
	UsageMetadata usageMetadata `json:"usageMetadata"`
}

type errorEnvelope struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

func mapFinishReason(reason string) openai.FinishReason {
	if reason == "MAX_TOKENS" {
		return openai.FinishReasonLength
	}
	return openai.FinishReasonStop
}

func candidateText(c candidate) string {
	var text strings.Builder
	for _, p := range c.Content.Parts {
		text.WriteString(p.Text)
	}
	return text.String()
}

func mapHTTPError(providerName string, status int, body []byte) error {
	var env errorEnvelope
	_ = json.Unmarshal(body, &env)
	msg := env.Error.Message
	if msg == "" {
		msg = string(body)
	}

	switch {
	case status == http.StatusUnauthorized || env.Error.Status == "UNAUTHENTICATED" || env.Error.Status == "PERMISSION_DENIED":
		return provider.NewError(providerName, provider.ErrorKindAuth, msg, nil)
	case status == http.StatusTooManyRequests || env.Error.Status == "RESOURCE_EXHAUSTED":
		return provider.NewError(providerName, provider.ErrorKindQuota, msg, nil)
	case status == http.StatusBadRequest || env.Error.Status == "INVALID_ARGUMENT":
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

func (a *Adapter) endpoint(model, method string, extraQuery string) string {
	return fmt.Sprintf("%s/v1beta/models/%s:%s?key=%s%s", a.baseURL, url.PathEscape(model), method, url.QueryEscape(a.apiKey), extraQuery)
}

// ChatCompletion implements provider.Provider.
func (a *Adapter) ChatCompletion(ctx context.Context, req *openai.ChatCompletionRequest) (*openai.ChatCompletionResponse, error) {
	data, err := json.Marshal(toGeminiRequest(req))
	if err != nil {
		return nil, provider.NewError(a.Name(), provider.ErrorKindUnknown, "encode request", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.endpoint(req.Model, "generateContent", ""), bytes.NewReader(data))
	if err != nil {
		return nil, provider.NewError(a.Name(), provider.ErrorKindUnknown, "build request", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

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

	var gr generateResponse
	if err := json.Unmarshal(respBody, &gr); err != nil {
		return nil, provider.NewError(a.Name(), provider.ErrorKindUnknown, "decode response", err)
	}
	if len(gr.Candidates) == 0 {
		return nil, provider.NewError(a.Name(), provider.ErrorKindUnknown, "response had no candidates", nil)
	}

	c := gr.Candidates[0]
	return &openai.ChatCompletionResponse{
		ID:      "",
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   req.Model,
		Choices: []openai.Choice{{
			Index:        0,
			Message:      openai.Message{Role: openai.RoleAssistant, Content: candidateText(c)},
			FinishReason: mapFinishReason(c.FinishReason),
		}},
		Usage: openai.Usage{
			PromptTokens:     gr.UsageMetadata.PromptTokenCount,
			CompletionTokens: gr.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      gr.UsageMetadata.TotalTokenCount,
		},
	}, nil
}

// StreamChatCompletion implements provider.Provider.
func (a *Adapter) StreamChatCompletion(ctx context.Context, req *openai.ChatCompletionRequest) (<-chan provider.StreamEvent, error) {
	data, err := json.Marshal(toGeminiRequest(req))
	if err != nil {
		return nil, provider.NewError(a.Name(), provider.ErrorKindUnknown, "encode request", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.endpoint(req.Model, "streamGenerateContent", "&alt=sse"), bytes.NewReader(data))
	if err != nil {
		return nil, provider.NewError(a.Name(), provider.ErrorKindUnknown, "build request", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

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

func (a *Adapter) pumpStream(ctx context.Context, body io.ReadCloser, model string, events chan<- provider.StreamEvent) {
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
		if payload == "" {
			continue
		}

		var gr generateResponse
		if err := json.Unmarshal([]byte(payload), &gr); err != nil {
			continue
		}
		if len(gr.Candidates) == 0 {
			continue
		}

		c := gr.Candidates[0]
		chunk := &openai.ChatCompletionChunk{
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Model:   model,
			Choices: []openai.ChunkChoice{{Index: 0, Delta: openai.Delta{Content: candidateText(c)}}},
		}
		if c.FinishReason != "" {
			finish := mapFinishReason(c.FinishReason)
			chunk.Choices[0].FinishReason = &finish
		}
		if !send(provider.StreamEvent{Chunk: chunk}) {
			return
		}
	}

	if err := scanner.Err(); err != nil {
		send(provider.StreamEvent{Err: provider.NewError(a.Name(), provider.ErrorKindTransient, "stream read failed", err)})
	}
}

var _ provider.Provider = (*Adapter)(nil)
