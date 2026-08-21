package proxy_test

import (
	"context"
	"testing"

	"github.com/hovanhoa/llmgateway/internal/db"
	"github.com/hovanhoa/llmgateway/internal/db/testdb"
	"github.com/hovanhoa/llmgateway/internal/model"
	"github.com/hovanhoa/llmgateway/internal/provider"
	"github.com/hovanhoa/llmgateway/internal/proxy"
	"github.com/hovanhoa/llmgateway/pkg/openai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeProvider is a minimal provider.Provider used to test proxy routing
// without any real HTTP calls.
type fakeProvider struct {
	name       string
	calls      int
	failTimes  int
	failKind   provider.ErrorKind
	streamChan chan provider.StreamEvent
	lastModel  string
}

func (f *fakeProvider) Name() string { return f.name }

func (f *fakeProvider) ChatCompletion(_ context.Context, req *openai.ChatCompletionRequest) (*openai.ChatCompletionResponse, error) {
	f.calls++
	f.lastModel = req.Model
	if f.calls <= f.failTimes {
		return nil, provider.NewError(f.name, f.failKind, "synthetic failure", nil)
	}
	return &openai.ChatCompletionResponse{
		ID:      "resp_1",
		Model:   req.Model,
		Choices: []openai.Choice{{Message: openai.Message{Role: openai.RoleAssistant, Content: "ok"}}},
	}, nil
}

func (f *fakeProvider) StreamChatCompletion(_ context.Context, req *openai.ChatCompletionRequest) (<-chan provider.StreamEvent, error) {
	f.lastModel = req.Model
	return f.streamChan, nil
}

func chatRequest(model string) *openai.ChatCompletionRequest {
	return &openai.ChatCompletionRequest{
		Model:    model,
		Messages: []openai.Message{{Role: openai.RoleUser, Content: "hi"}},
	}
}

func newHandler(database *db.Database, providers provider.Registry) *proxy.Handler {
	return proxy.NewHandler(proxy.Dependencies{Database: database, Providers: providers})
}

func TestChatCompletion_RoutesToProviderWithBareModelName(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)
	fake := &fakeProvider{name: "anthropic"}
	h := newHandler(database, provider.Registry{"anthropic": fake})

	principal := &proxy.Principal{ID: "account_1", Type: model.IdentityAccount}
	resp, err := h.ChatCompletion(ctx, principal, chatRequest("anthropic/claude-3-5-sonnet"))
	require.NoError(t, err)
	assert.Equal(t, "ok", resp.Choices[0].Message.Content)
	assert.Equal(t, "claude-3-5-sonnet", fake.lastModel, "the provider prefix must be stripped before dispatch")
}

func TestChatCompletion_InvalidModelFormat(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)
	h := newHandler(database, provider.Registry{})

	principal := &proxy.Principal{ID: "account_1", Type: model.IdentityAccount}
	_, err := h.ChatCompletion(ctx, principal, chatRequest("claude-3-5-sonnet"))
	require.Error(t, err)

	status, body := proxy.ErrorResponseFor(err)
	assert.Equal(t, 400, status)
	assert.Equal(t, openai.ErrorTypeInvalidRequest, body.Error.Type)
	assert.Contains(t, body.Error.Message, `"provider/model"`)
}

func TestChatCompletion_UnknownProvider(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)
	h := newHandler(database, provider.Registry{})

	principal := &proxy.Principal{ID: "account_1", Type: model.IdentityAccount}
	_, err := h.ChatCompletion(ctx, principal, chatRequest("mistral/large"))
	require.Error(t, err)

	status, body := proxy.ErrorResponseFor(err)
	assert.Equal(t, 400, status)
	assert.Contains(t, body.Error.Message, `unknown provider "mistral"`)
}

func TestChatCompletion_ValidationFailure(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)
	h := newHandler(database, provider.Registry{})

	principal := &proxy.Principal{ID: "account_1", Type: model.IdentityAccount}
	req := chatRequest("anthropic/claude-3-5-sonnet")
	req.Messages = nil

	_, err := h.ChatCompletion(ctx, principal, req)
	require.Error(t, err)
	status, _ := proxy.ErrorResponseFor(err)
	assert.Equal(t, 400, status)
}

func TestChatCompletion_NoTeamIsUnrestricted(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)
	fake := &fakeProvider{name: "anthropic"}
	h := newHandler(database, provider.Registry{"anthropic": fake})

	// OrgID empty - the principal's account has no team.
	principal := &proxy.Principal{ID: "account_1", Type: model.IdentityAccount, OrgID: ""}
	_, err := h.ChatCompletion(ctx, principal, chatRequest("anthropic/claude-3-5-sonnet"))
	require.NoError(t, err)
}

func TestChatCompletion_BlockedByTeamAllowlist(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	team, err := database.CreateTeam(ctx, &model.Team{Name: "Restricted"})
	require.NoError(t, err)
	_, err = database.UpdateTeamModelAllowlist(ctx, team.ID, []string{"openai:*"})
	require.NoError(t, err)

	fake := &fakeProvider{name: "anthropic"}
	h := newHandler(database, provider.Registry{"anthropic": fake})

	principal := &proxy.Principal{ID: "account_1", Type: model.IdentityAccount, OrgID: team.ID}
	_, err = h.ChatCompletion(ctx, principal, chatRequest("anthropic/claude-3-5-sonnet"))
	require.Error(t, err)

	status, body := proxy.ErrorResponseFor(err)
	assert.Equal(t, 403, status)
	assert.Equal(t, openai.ErrorTypePermission, body.Error.Type)
	assert.Equal(t, 0, fake.calls, "a blocked call must never reach the provider")
}

func TestChatCompletion_AllowedByTeamAllowlist(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	team, err := database.CreateTeam(ctx, &model.Team{Name: "Allowed"})
	require.NoError(t, err)
	_, err = database.UpdateTeamModelAllowlist(ctx, team.ID, []string{"anthropic:*"})
	require.NoError(t, err)

	fake := &fakeProvider{name: "anthropic"}
	h := newHandler(database, provider.Registry{"anthropic": fake})

	principal := &proxy.Principal{ID: "account_1", Type: model.IdentityAccount, OrgID: team.ID}
	_, err = h.ChatCompletion(ctx, principal, chatRequest("anthropic/claude-3-5-sonnet"))
	require.NoError(t, err)
	assert.Equal(t, 1, fake.calls)
}

func TestChatCompletion_RetriesTransientProviderErrors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)
	fake := &fakeProvider{name: "anthropic", failTimes: 2, failKind: provider.ErrorKindTransient}
	h := newHandler(database, provider.Registry{"anthropic": fake})

	principal := &proxy.Principal{ID: "account_1", Type: model.IdentityAccount}
	resp, err := h.ChatCompletion(ctx, principal, chatRequest("anthropic/claude-3-5-sonnet"))
	require.NoError(t, err)
	assert.Equal(t, "ok", resp.Choices[0].Message.Content)
	assert.Equal(t, 3, fake.calls)
}

func TestChatCompletion_DoesNotRetryAuthErrors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)
	fake := &fakeProvider{name: "anthropic", failTimes: 1, failKind: provider.ErrorKindAuth}
	h := newHandler(database, provider.Registry{"anthropic": fake})

	principal := &proxy.Principal{ID: "account_1", Type: model.IdentityAccount}
	_, err := h.ChatCompletion(ctx, principal, chatRequest("anthropic/claude-3-5-sonnet"))
	require.Error(t, err)
	assert.Equal(t, 1, fake.calls)

	status, _ := proxy.ErrorResponseFor(err)
	assert.Equal(t, 401, status)
}

func TestStreamChatCompletion_ForwardsProviderChannel(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	ch := make(chan provider.StreamEvent, 1)
	ch <- provider.StreamEvent{Chunk: &openai.ChatCompletionChunk{ID: "c1"}}
	close(ch)

	fake := &fakeProvider{name: "anthropic", streamChan: ch}
	h := newHandler(database, provider.Registry{"anthropic": fake})

	req := chatRequest("anthropic/claude-3-5-sonnet")
	req.Stream = true

	principal := &proxy.Principal{ID: "account_1", Type: model.IdentityAccount}
	events, err := h.StreamChatCompletion(ctx, principal, req)
	require.NoError(t, err)

	var got []provider.StreamEvent
	for ev := range events {
		got = append(got, ev)
	}
	require.Len(t, got, 1)
	assert.Equal(t, "c1", got[0].Chunk.ID)
	assert.Equal(t, "claude-3-5-sonnet", fake.lastModel)
}

func TestErrorResponseFor_UnknownErrorMapsToInternal(t *testing.T) {
	t.Parallel()

	status, body := proxy.ErrorResponseFor(assertError{})
	assert.Equal(t, 500, status)
	assert.Equal(t, openai.ErrorTypeInternal, body.Error.Type)
}

type assertError struct{}

func (assertError) Error() string { return "boom" }
