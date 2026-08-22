package proxy_test

import (
	"context"
	"testing"
	"time"

	"github.com/hovanhoa/llmgateway/internal/db"
	"github.com/hovanhoa/llmgateway/internal/db/testdb"
	"github.com/hovanhoa/llmgateway/internal/model"
	"github.com/hovanhoa/llmgateway/internal/policy"
	"github.com/hovanhoa/llmgateway/internal/provider"
	"github.com/hovanhoa/llmgateway/internal/proxy"
	"github.com/hovanhoa/llmgateway/internal/quota"
	"github.com/hovanhoa/llmgateway/pkg/driver/memkv"
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
	lastReq    *openai.ChatCompletionRequest
	respUsage  openai.Usage
}

func (f *fakeProvider) Name() string { return f.name }

func (f *fakeProvider) ChatCompletion(_ context.Context, req *openai.ChatCompletionRequest) (*openai.ChatCompletionResponse, error) {
	f.calls++
	f.lastModel = req.Model
	f.lastReq = req
	if f.calls <= f.failTimes {
		return nil, provider.NewError(f.name, f.failKind, "synthetic failure", nil)
	}
	return &openai.ChatCompletionResponse{
		ID:      "resp_1",
		Model:   req.Model,
		Choices: []openai.Choice{{Message: openai.Message{Role: openai.RoleAssistant, Content: "ok"}}},
		Usage:   f.respUsage,
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

func newHandlerWithQuotaPolicy(database *db.Database, providers provider.Registry, q *quota.Checker, p *policy.Chain) *proxy.Handler {
	return proxy.NewHandler(proxy.Dependencies{Database: database, Providers: providers, Quota: q, Policy: p})
}

func TestChatCompletion_RoutesToProviderWithBareModelName(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)
	fake := &fakeProvider{name: "anthropic"}
	h := newHandler(database, provider.Registry{"anthropic": fake})

	principal := &proxy.Principal{ID: "account_1", Type: model.IdentityAccount}
	resp, err := h.ChatCompletion(ctx, "req_test", principal, chatRequest("anthropic/claude-3-5-sonnet"))
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
	_, err := h.ChatCompletion(ctx, "req_test", principal, chatRequest("claude-3-5-sonnet"))
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
	_, err := h.ChatCompletion(ctx, "req_test", principal, chatRequest("mistral/large"))
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

	_, err := h.ChatCompletion(ctx, "req_test", principal, req)
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
	_, err := h.ChatCompletion(ctx, "req_test", principal, chatRequest("anthropic/claude-3-5-sonnet"))
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
	_, err = h.ChatCompletion(ctx, "req_test", principal, chatRequest("anthropic/claude-3-5-sonnet"))
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
	_, err = h.ChatCompletion(ctx, "req_test", principal, chatRequest("anthropic/claude-3-5-sonnet"))
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
	resp, err := h.ChatCompletion(ctx, "req_test", principal, chatRequest("anthropic/claude-3-5-sonnet"))
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
	_, err := h.ChatCompletion(ctx, "req_test", principal, chatRequest("anthropic/claude-3-5-sonnet"))
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
	events, err := h.StreamChatCompletion(ctx, "req_test", principal, req)
	require.NoError(t, err)

	var got []provider.StreamEvent
	for ev := range events {
		got = append(got, ev)
	}
	require.Len(t, got, 1)
	assert.Equal(t, "c1", got[0].Chunk.ID)
	assert.Equal(t, "claude-3-5-sonnet", fake.lastModel)
}

func intPtr(n int) *int { return &n }

func TestChatCompletion_QuotaExceededBlocksCallAndNeverReachesProvider(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	team, err := database.CreateTeam(ctx, &model.Team{Name: "Tiny Budget", MonthlyTokenBudget: intPtr(1)})
	require.NoError(t, err)

	fake := &fakeProvider{name: "anthropic"}
	checker := quota.NewChecker(memkv.New())
	h := newHandlerWithQuotaPolicy(database, provider.Registry{"anthropic": fake}, checker, nil)

	principal := &proxy.Principal{ID: "account_1", Type: model.IdentityAccount, OrgID: team.ID}
	_, err = h.ChatCompletion(ctx, "req_test", principal, chatRequest("anthropic/claude-3-5-sonnet"))
	require.Error(t, err)

	status, body := proxy.ErrorResponseFor(err)
	assert.Equal(t, 429, status)
	assert.Equal(t, openai.ErrorTypeRateLimit, body.Error.Type)
	assert.Equal(t, 0, fake.calls, "a quota-exceeded call must never reach the provider")
}

func TestChatCompletion_QuotaReconciledToActualUsageAfterSuccess(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	team, err := database.CreateTeam(ctx, &model.Team{Name: "Roomy Budget", MonthlyTokenBudget: intPtr(100_000)})
	require.NoError(t, err)

	fake := &fakeProvider{name: "anthropic", respUsage: openai.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15}}
	checker := quota.NewChecker(memkv.New())
	h := newHandlerWithQuotaPolicy(database, provider.Registry{"anthropic": fake}, checker, nil)

	principal := &proxy.Principal{ID: "account_1", Type: model.IdentityAccount, OrgID: team.ID}
	_, err = h.ChatCompletion(ctx, "req_test", principal, chatRequest("anthropic/claude-3-5-sonnet"))
	require.NoError(t, err)

	usage, err := checker.Usage(ctx, team.ID)
	require.NoError(t, err)
	assert.Equal(t, 15, usage, "the reservation's rough estimate must be reconciled down to the provider's actual usage")
}

func TestChatCompletion_PolicyDenyBlocksCallAndReleasesQuotaReservation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	team, err := database.CreateTeam(ctx, &model.Team{Name: "Policed", MonthlyTokenBudget: intPtr(100_000)})
	require.NoError(t, err)

	fake := &fakeProvider{name: "anthropic"}
	checker := quota.NewChecker(memkv.New())
	chain := policy.NewChain(policy.BlockedPatterns{Patterns: []string{"forbidden"}})
	h := newHandlerWithQuotaPolicy(database, provider.Registry{"anthropic": fake}, checker, chain)

	principal := &proxy.Principal{ID: "account_1", Type: model.IdentityAccount, OrgID: team.ID}
	req := chatRequest("anthropic/claude-3-5-sonnet")
	req.Messages[0].Content = "this is a forbidden request"

	_, err = h.ChatCompletion(ctx, "req_test", principal, req)
	require.Error(t, err)

	status, body := proxy.ErrorResponseFor(err)
	assert.Equal(t, 403, status)
	assert.Equal(t, openai.ErrorTypePermission, body.Error.Type)
	assert.Equal(t, 0, fake.calls, "a policy-denied call must never reach the provider")

	usage, err := checker.Usage(ctx, team.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, usage, "the reservation for a denied call must be released")
}

func TestChatCompletion_PolicyRedactsSensitiveDataBeforeDispatch(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	fake := &fakeProvider{name: "anthropic"}
	chain := policy.NewChain(policy.SensitiveDataRedaction{})
	h := newHandlerWithQuotaPolicy(database, provider.Registry{"anthropic": fake}, nil, chain)

	principal := &proxy.Principal{ID: "account_1", Type: model.IdentityAccount}
	req := chatRequest("anthropic/claude-3-5-sonnet")
	req.Messages[0].Content = "my key is sk-abcdefghijklmnopqrstuvwxyz"

	_, err := h.ChatCompletion(ctx, "req_test", principal, req)
	require.NoError(t, err)

	require.NotNil(t, fake.lastReq)
	assert.Equal(t, "my key is [REDACTED]", fake.lastReq.Messages[0].Content)
}

func TestChatCompletion_RecordsUsageEventOnSuccess(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := testdb.NewTestDatabase(ctx, t)

	team, err := database.CreateTeam(ctx, &model.Team{Name: "Metered"})
	require.NoError(t, err)

	account, err := database.CreateAccount(ctx, &model.Account{Email: "metered@example.com", Username: "metered"})
	require.NoError(t, err)

	fake := &fakeProvider{name: "anthropic", respUsage: openai.Usage{PromptTokens: 7, CompletionTokens: 3, TotalTokens: 10}}
	h := newHandler(database, provider.Registry{"anthropic": fake})

	principal := &proxy.Principal{ID: account.ID, Type: model.IdentityAccount, OrgID: team.ID}
	_, err = h.ChatCompletion(ctx, "req_test", principal, chatRequest("anthropic/claude-3-5-sonnet"))
	require.NoError(t, err)

	summary, err := database.SumTeamUsage(ctx, team.ID, time.Now().Add(-time.Hour))
	require.NoError(t, err)
	assert.Equal(t, 1, summary.RequestCount)
	assert.Equal(t, 10, summary.TotalTokens)
}

func TestErrorResponseFor_UnknownErrorMapsToInternal(t *testing.T) {
	t.Parallel()

	status, body := proxy.ErrorResponseFor(assertError{})
	assert.Equal(t, 500, status)
	assert.Equal(t, openai.ErrorTypeInternal, body.Error.Type)
}

type assertError struct{}

func (assertError) Error() string { return "boom" }
