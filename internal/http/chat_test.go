package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/benbjohnson/clock"
	"github.com/hovanhoa/llmgateway/internal/db/testdb"
	"github.com/hovanhoa/llmgateway/internal/model"
	"github.com/hovanhoa/llmgateway/internal/provider"
	corehttp "github.com/hovanhoa/llmgateway/pkg/core/http/testhttp"
	"github.com/hovanhoa/llmgateway/pkg/openai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubProvider is a minimal provider.Provider for exercising the HTTP
// route without any real network calls.
type stubProvider struct {
	name string
}

func (s *stubProvider) Name() string { return s.name }

func (s *stubProvider) ChatCompletion(_ context.Context, req *openai.ChatCompletionRequest) (*openai.ChatCompletionResponse, error) {
	return &openai.ChatCompletionResponse{
		ID:      "resp_1",
		Object:  "chat.completion",
		Model:   req.Model,
		Choices: []openai.Choice{{Message: openai.Message{Role: openai.RoleAssistant, Content: "stub reply"}, FinishReason: openai.FinishReasonStop}},
		Usage:   openai.Usage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2},
	}, nil
}

func (s *stubProvider) StreamChatCompletion(_ context.Context, req *openai.ChatCompletionRequest) (<-chan provider.StreamEvent, error) {
	ch := make(chan provider.StreamEvent, 2)
	ch <- provider.StreamEvent{Chunk: &openai.ChatCompletionChunk{
		ID: "resp_1", Object: "chat.completion.chunk", Model: req.Model,
		Choices: []openai.ChunkChoice{{Delta: openai.Delta{Role: openai.RoleAssistant, Content: "Hel"}}},
	}}
	finish := openai.FinishReasonStop
	ch <- provider.StreamEvent{Chunk: &openai.ChatCompletionChunk{
		ID: "resp_1", Object: "chat.completion.chunk", Model: req.Model,
		Choices: []openai.ChunkChoice{{Delta: openai.Delta{Content: "lo"}, FinishReason: &finish}},
	}}
	close(ch)
	return ch, nil
}

var _ provider.Provider = (*stubProvider)(nil)

const chatCompletionsBody = `{"model":"stub/model-x","messages":[{"role":"user","content":"hi"}]}`

func TestChatCompletions_RequiresAuth(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)
	service := NewService(Dependencies{DB: database, Providers: provider.Registry{"stub": &stubProvider{name: "stub"}}, Clock: clock.New()})
	tester := corehttp.NewHTTPTester(t, service.Service)

	req := corehttp.NewRequestBuilder(http.MethodPost, "/v1/chat/completions").
		WithHeader("Content-Type", "application/json").
		WithBodyString(chatCompletionsBody).
		Build()

	tester.Run(req).AssertStatusCode(http.StatusUnauthorized)
}

func TestChatCompletions_NonStreamingSuccess(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	account, err := database.CreateAccount(ctx, &model.Account{Email: "proxy@example.com", Username: "proxyuser"})
	require.NoError(t, err)
	secret, err := database.CreateAPIKey(ctx, account.ID)
	require.NoError(t, err)

	service := NewService(Dependencies{DB: database, Providers: provider.Registry{"stub": &stubProvider{name: "stub"}}, Clock: clock.New()})
	tester := corehttp.NewHTTPTester(t, service.Service)

	req := corehttp.NewRequestBuilder(http.MethodPost, "/v1/chat/completions").
		WithHeader("Authorization", "Bearer "+secret.Key).
		WithHeader("Content-Type", "application/json").
		WithBodyString(chatCompletionsBody).
		Build()

	resp := tester.Run(req).AssertStatusCode(http.StatusOK)

	var body openai.ChatCompletionResponse
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	assert.Equal(t, "model-x", body.Model, "the provider prefix must be stripped before dispatch")
	assert.Equal(t, "stub reply", body.Choices[0].Message.Content)
}

func TestChatCompletions_InvalidModelFormatReturnsOpenAIError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	account, err := database.CreateAccount(ctx, &model.Account{Email: "proxy2@example.com", Username: "proxyuser2"})
	require.NoError(t, err)
	secret, err := database.CreateAPIKey(ctx, account.ID)
	require.NoError(t, err)

	service := NewService(Dependencies{DB: database, Providers: provider.Registry{}, Clock: clock.New()})
	tester := corehttp.NewHTTPTester(t, service.Service)

	req := corehttp.NewRequestBuilder(http.MethodPost, "/v1/chat/completions").
		WithHeader("Authorization", "Bearer "+secret.Key).
		WithHeader("Content-Type", "application/json").
		WithBodyString(`{"model":"no-provider-prefix","messages":[{"role":"user","content":"hi"}]}`).
		Build()

	resp := tester.Run(req).AssertStatusCode(http.StatusBadRequest)

	var body openai.ErrorResponse
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	assert.Equal(t, openai.ErrorTypeInvalidRequest, body.Error.Type)
}

func TestChatCompletions_BlockedByTeamAllowlist(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	team, err := database.CreateTeam(ctx, &model.Team{Name: "Locked"})
	require.NoError(t, err)
	_, err = database.UpdateTeamModelAllowlist(ctx, team.ID, []string{"other:*"})
	require.NoError(t, err)

	account, err := database.CreateAccount(ctx, &model.Account{Email: "proxy3@example.com", Username: "proxyuser3", TeamID: &team.ID})
	require.NoError(t, err)
	secret, err := database.CreateAPIKey(ctx, account.ID)
	require.NoError(t, err)

	service := NewService(Dependencies{DB: database, Providers: provider.Registry{"stub": &stubProvider{name: "stub"}}, Clock: clock.New()})
	tester := corehttp.NewHTTPTester(t, service.Service)

	req := corehttp.NewRequestBuilder(http.MethodPost, "/v1/chat/completions").
		WithHeader("Authorization", "Bearer "+secret.Key).
		WithHeader("Content-Type", "application/json").
		WithBodyString(chatCompletionsBody).
		Build()

	resp := tester.Run(req).AssertStatusCode(http.StatusForbidden)

	var body openai.ErrorResponse
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	assert.Equal(t, openai.ErrorTypePermission, body.Error.Type)
}

func TestChatCompletions_StreamingSuccessSendsSSE(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	account, err := database.CreateAccount(ctx, &model.Account{Email: "proxy4@example.com", Username: "proxyuser4"})
	require.NoError(t, err)
	secret, err := database.CreateAPIKey(ctx, account.ID)
	require.NoError(t, err)

	service := NewService(Dependencies{DB: database, Providers: provider.Registry{"stub": &stubProvider{name: "stub"}}, Clock: clock.New()})
	tester := corehttp.NewHTTPTester(t, service.Service)

	req := corehttp.NewRequestBuilder(http.MethodPost, "/v1/chat/completions").
		WithHeader("Authorization", "Bearer "+secret.Key).
		WithHeader("Content-Type", "application/json").
		WithBodyString(`{"model":"stub/model-x","stream":true,"messages":[{"role":"user","content":"hi"}]}`).
		Build()

	resp := tester.Run(req).AssertStatusCode(http.StatusOK)
	resp.AssertHeader("Content-Type", "text/event-stream")

	body := resp.Body.String()
	frames := strings.Split(strings.TrimSpace(body), "\n\n")
	require.Len(t, frames, 3, "2 chunks + [DONE]")
	assert.Equal(t, "data: [DONE]", frames[2])

	var chunk openai.ChatCompletionChunk
	require.NoError(t, json.Unmarshal([]byte(strings.TrimPrefix(frames[0], "data: ")), &chunk))
	assert.Equal(t, "Hel", chunk.Choices[0].Delta.Content)
}
