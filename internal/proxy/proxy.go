// Package proxy implements the business logic behind the gateway's
// OpenAI-compatible LLM proxy endpoint: request validation, provider
// routing (by a "provider/model" prefix on the request's model field),
// the Phase 2 per-team model allowlist, Phase 4's per-team quota and
// policy checks, and dispatch to the resolved provider.Provider, with a
// retry wrapper on non-streaming calls and usage metering on completion.
//
// This package is deliberately independent of the HTTP framework -
// internal/http wires it into an actual route.
package proxy

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hovanhoa/llmgateway/internal/db"
	"github.com/hovanhoa/llmgateway/internal/model"
	"github.com/hovanhoa/llmgateway/internal/policy"
	"github.com/hovanhoa/llmgateway/internal/provider"
	"github.com/hovanhoa/llmgateway/internal/quota"
	"github.com/hovanhoa/llmgateway/internal/usage"
	"github.com/hovanhoa/llmgateway/pkg/core/auth"
	"github.com/hovanhoa/llmgateway/pkg/core/log"
	"github.com/hovanhoa/llmgateway/pkg/openai"
)

// maxRetries bounds how many additional attempts a non-streaming call gets
// when the upstream provider returns a transient error.
const maxRetries = 2

// Dependencies of the Handler. Quota and Policy are optional: a nil Quota
// disables quota enforcement entirely (equivalent to every team being
// unlimited), and a nil Policy skips content policy checks.
type Dependencies struct {
	Database  *db.Database
	Providers provider.Registry
	Quota     *quota.Checker
	Policy    *policy.Chain
}

// Handler implements the proxy's request handling, independent of any HTTP
// framework.
type Handler struct {
	deps Dependencies
}

// NewHandler returns a new Handler.
func NewHandler(deps Dependencies) *Handler {
	return &Handler{deps: deps}
}

// Principal is the concrete Principal instantiation used throughout this
// project - see internal/model/identity.go.
type Principal = auth.Principal[model.Identity, model.Role]

// resolve splits a "provider/model" request model field and looks up the
// named provider in the registry.
func (h *Handler) resolve(modelField string) (provider.Provider, string, string, error) {
	providerName, modelName, ok := strings.Cut(modelField, "/")
	if !ok || providerName == "" || modelName == "" {
		return nil, "", "", provider.NewError("", provider.ErrorKindInvalidRequest,
			`model must be formatted as "provider/model", e.g. "anthropic/claude-3-5-sonnet-20241022"`, nil)
	}

	p, ok := h.deps.Providers.Get(providerName)
	if !ok {
		return nil, "", "", provider.NewError(providerName, provider.ErrorKindInvalidRequest,
			fmt.Sprintf("unknown provider %q", providerName), nil)
	}

	return p, providerName, modelName, nil
}

// preparedCall is everything prepare resolved about a request, threaded
// through to the caller so it can dispatch, meter, and reconcile quota
// without re-deriving any of it.
type preparedCall struct {
	provider     provider.Provider
	providerName string
	modelName    string
	upstream     *openai.ChatCompletionRequest
	team         *model.Team // nil when the caller's account has no team
	estimate     int
	quotaWindow  string // the window Reserve made its reservation against; passed back to Reconcile unchanged
}

// prepare validates the request, resolves its provider/model, checks the
// caller's team allowlist and quota, and runs the policy chain - in that
// order, cheapest checks first, so an expensive provider call is the last
// thing that can happen. It returns everything ChatCompletion/
// StreamChatCompletion need to dispatch and later reconcile quota/usage.
func (h *Handler) prepare(ctx context.Context, requestID string, principal *Principal, req *openai.ChatCompletionRequest) (*preparedCall, error) {
	if err := req.Validate(); err != nil {
		return nil, provider.NewError("", provider.ErrorKindInvalidRequest, err.Error(), nil)
	}

	p, providerName, modelName, err := h.resolve(req.Model)
	if err != nil {
		return nil, err
	}

	var team *model.Team
	if principal.OrgID != "" {
		team, err = h.deps.Database.GetTeamByID(ctx, principal.OrgID)
		if err != nil {
			return nil, err
		}
	}

	if team != nil && !team.IsModelAllowed(providerName, modelName) {
		return nil, provider.NewError(providerName, provider.ErrorKindPolicy,
			fmt.Sprintf("model %q is not on this team's allowlist", providerName+"/"+modelName), nil)
	}

	upstream := *req
	upstream.Model = modelName

	estimate := quota.EstimateTokens(&upstream)
	var quotaWindow string
	if h.deps.Quota != nil && team != nil {
		var err error
		quotaWindow, err = h.deps.Quota.Reserve(ctx, team.ID, team.MonthlyTokenBudget, estimate)
		if err != nil {
			if err == quota.ErrExceeded {
				quotaDeniedTotal.WithLabelValues(team.ID).Inc()
				return nil, provider.NewError(providerName, provider.ErrorKindQuota,
					"team monthly token budget exceeded", nil)
			}
			return nil, err
		}
	}

	if h.deps.Policy != nil {
		redacted, decision := h.deps.Policy.Evaluate(&upstream)
		h.logPolicyDecision(ctx, requestID, decision)
		if decision.Action == policy.ActionDeny {
			h.releaseQuota(ctx, team, quotaWindow, estimate)
			return nil, provider.NewError(providerName, provider.ErrorKindPolicy, decision.Message, nil)
		}
		upstream = *redacted
	}

	return &preparedCall{
		provider:     p,
		providerName: providerName,
		modelName:    modelName,
		upstream:     &upstream,
		team:         team,
		estimate:     estimate,
		quotaWindow:  quotaWindow,
	}, nil
}

func (h *Handler) logPolicyDecision(ctx context.Context, requestID string, decision policy.Decision) {
	if decision.Action == policy.ActionAllow {
		return
	}
	// Only the reason code is logged, never raw request content.
	log.FromContext(ctx).Info("policy decision",
		log.String("request_id", requestID),
		log.String("action", string(decision.Action)),
		log.String("reason_code", decision.ReasonCode),
	)
}

// logCompletion emits the one structured log line Phase 5 calls for per
// proxy call: request id, account, team, provider, model, latency, and
// status. call may be nil if prepare() itself failed (e.g. invalid model
// format), in which case provider/model are simply omitted.
func (h *Handler) logCompletion(ctx context.Context, requestID string, principal *Principal, call *preparedCall, start time.Time, err error) {
	fields := []log.Field{
		log.String("request_id", requestID),
		log.String("account_id", principal.ID),
		log.Duration("latency", time.Since(start)),
	}
	if principal.OrgID != "" {
		fields = append(fields, log.String("team_id", principal.OrgID))
	}
	if call != nil {
		fields = append(fields, log.String("provider", call.providerName), log.String("model", call.modelName))
	}

	if err != nil {
		fields = append(fields, log.String("status", "error"), log.Error(err))
		log.FromContext(ctx).Error("chat_completion", fields...)
		return
	}
	log.FromContext(ctx).Info("chat_completion", append(fields, log.String("status", "success"))...)
}

// releaseQuota undoes a reservation for a call that never reached (or
// never completed at) the provider.
func (h *Handler) releaseQuota(ctx context.Context, team *model.Team, window string, estimate int) {
	if h.deps.Quota == nil || team == nil {
		return
	}
	_ = h.deps.Quota.Reconcile(ctx, window, team.MonthlyTokenBudget, estimate, 0)
}

// recordUsage reconciles the quota reservation to actual usage and persists
// a durable usage_event row.
func (h *Handler) recordUsage(ctx context.Context, requestID string, principal *Principal, call *preparedCall, resp *openai.ChatCompletionResponse) {
	if h.deps.Quota != nil && call.team != nil {
		_ = h.deps.Quota.Reconcile(ctx, call.quotaWindow, call.team.MonthlyTokenBudget, call.estimate, resp.Usage.TotalTokens)
	}

	var teamID *string
	if call.team != nil {
		teamID = &call.team.ID
	}

	event := &model.UsageEvent{
		RequestID:        requestID,
		AccountID:        principal.ID,
		TeamID:           teamID,
		Provider:         call.providerName,
		Model:            call.modelName,
		PromptTokens:     resp.Usage.PromptTokens,
		CompletionTokens: resp.Usage.CompletionTokens,
		TotalTokens:      resp.Usage.TotalTokens,
		CostUSD:          usage.CalculateCost(call.providerName, call.modelName, resp.Usage.PromptTokens, resp.Usage.CompletionTokens),
	}
	if err := h.deps.Database.RecordUsageEvent(ctx, event); err != nil {
		log.FromContext(ctx).Error("failed to record usage event", log.Error(err), log.String("request_id", requestID))
	}
}

// ChatCompletion handles a single non-streaming chat completion call.
func (h *Handler) ChatCompletion(ctx context.Context, requestID string, principal *Principal, req *openai.ChatCompletionRequest) (*openai.ChatCompletionResponse, error) {
	start := time.Now()

	call, err := h.prepare(ctx, requestID, principal, req)
	if err != nil {
		h.logCompletion(ctx, requestID, principal, nil, start, err)
		return nil, err
	}

	resp, err := provider.WithRetry(ctx, maxRetries, func() (*openai.ChatCompletionResponse, error) {
		return call.provider.ChatCompletion(ctx, call.upstream)
	})
	if err != nil {
		h.releaseQuota(ctx, call.team, call.quotaWindow, call.estimate)
		h.logCompletion(ctx, requestID, principal, call, start, err)
		return nil, err
	}

	h.recordUsage(ctx, requestID, principal, call, resp)
	h.logCompletion(ctx, requestID, principal, call, start, nil)
	return resp, nil
}

// StreamChatCompletion handles a streaming chat completion call. Streaming
// calls are not retried - a client already receiving chunks can't safely
// have the request silently restarted underneath it.
//
// Known limitation: none of the three live adapters reliably surface a
// final token-usage figure over the stream, so a successful stream simply
// keeps its upfront estimate reserved (no usage_event is recorded for
// streaming calls yet) rather than reconciling to a real number. A failed
// stream still fully releases its reservation.
func (h *Handler) StreamChatCompletion(ctx context.Context, requestID string, principal *Principal, req *openai.ChatCompletionRequest) (<-chan provider.StreamEvent, error) {
	start := time.Now()

	call, err := h.prepare(ctx, requestID, principal, req)
	if err != nil {
		h.logCompletion(ctx, requestID, principal, nil, start, err)
		return nil, err
	}

	upstream, err := call.provider.StreamChatCompletion(ctx, call.upstream)
	if err != nil {
		h.releaseQuota(ctx, call.team, call.quotaWindow, call.estimate)
		h.logCompletion(ctx, requestID, principal, call, start, err)
		return nil, err
	}

	events := make(chan provider.StreamEvent)
	go func() {
		defer close(events)

		var streamErr error
		for ev := range upstream {
			if ev.Err != nil {
				streamErr = ev.Err
				h.releaseQuota(ctx, call.team, call.quotaWindow, call.estimate)
			}
			events <- ev
		}

		status := "success"
		if streamErr != nil {
			status = "error"
		}
		streamDurationSeconds.WithLabelValues(call.providerName, status).Observe(time.Since(start).Seconds())
		h.logCompletion(ctx, requestID, principal, call, start, streamErr)
	}()

	return events, nil
}
