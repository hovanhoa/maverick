// Package proxy implements the business logic behind the gateway's
// OpenAI-compatible LLM proxy endpoint: request validation, provider
// routing (by a "provider/model" prefix on the request's model field),
// the Phase 2 per-team model allowlist, Phase 4's per-team and per-account
// quota and policy checks, and dispatch to the resolved provider.Provider,
// with a retry wrapper on non-streaming calls and usage metering on
// completion.
//
// This package is deliberately independent of the HTTP framework -
// internal/http wires it into an actual route.
package proxy

import (
	"context"
	"encoding/json"
	"errors"
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
// disables quota enforcement entirely (equivalent to every team and account
// being unlimited), and a nil Policy skips content policy checks.
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
			`model must be formatted as "provider/model", e.g. "anthropic/claude-sonnet-5"`, nil)
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
	provider           provider.Provider
	providerName       string
	modelName          string
	upstream           *openai.ChatCompletionRequest
	team               *model.Team    // nil when the caller's account has no team
	account            *model.Account // nil if the principal isn't backed by a real account (shouldn't happen outside tests)
	estimate           int
	quotaWindow        string // the window Reserve made its team reservation against; passed back to Reconcile unchanged
	accountQuotaWindow string // same, for the account reservation
}

// prepare validates the request, resolves its provider/model, checks the
// caller's team allowlist and the team's and account's quotas, and runs the
// policy chain - in that order, cheapest checks first, so an expensive
// provider call is the last thing that can happen. It returns everything
// ChatCompletion/StreamChatCompletion need to dispatch and later reconcile
// quota/usage.
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

	account, err := h.deps.Database.GetAccountByID(ctx, principal.ID)
	if err != nil {
		return nil, err
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

	var accountQuotaWindow string
	if h.deps.Quota != nil && account != nil {
		var err error
		accountQuotaWindow, err = h.deps.Quota.Reserve(ctx, account.ID, account.MonthlyTokenBudget, estimate)
		if err != nil {
			h.releaseQuota(ctx, team, quotaWindow, nil, "", estimate)
			if err == quota.ErrExceeded {
				accountQuotaDeniedTotal.WithLabelValues(account.ID).Inc()
				return nil, provider.NewError(providerName, provider.ErrorKindQuota,
					"account monthly token budget exceeded", nil)
			}
			return nil, err
		}
	}

	// A team's policy overrides run ahead of the platform baseline chain
	// below, on the original unredacted content - if this ran after the
	// baseline, a team's stricter "deny on sensitive data" would never
	// fire, since the baseline's SensitiveDataRedaction would have already
	// scrubbed the match out of the text by then.
	if team != nil {
		teamOverrides := policy.TeamOverrides{
			BlockedPatterns:     team.Policy.BlockedPatterns,
			DenyOnSensitiveData: team.Policy.DenyOnSensitiveData,
		}
		if teamOverrides.HasOverrides() {
			working, decision := policy.TeamChain(teamOverrides).Evaluate(&upstream)
			h.logPolicyDecision(ctx, requestID, decision)
			if decision.Action == policy.ActionDeny {
				h.releaseQuota(ctx, team, quotaWindow, account, accountQuotaWindow, estimate)
				return nil, provider.NewError(providerName, provider.ErrorKindPolicy, decision.Message, nil)
			}
			upstream = *working
		}
	}

	if h.deps.Policy != nil {
		redacted, decision := h.deps.Policy.Evaluate(&upstream)
		h.logPolicyDecision(ctx, requestID, decision)
		if decision.Action == policy.ActionDeny {
			h.releaseQuota(ctx, team, quotaWindow, account, accountQuotaWindow, estimate)
			return nil, provider.NewError(providerName, provider.ErrorKindPolicy, decision.Message, nil)
		}
		upstream = *redacted
	}

	return &preparedCall{
		provider:           p,
		providerName:       providerName,
		modelName:          modelName,
		upstream:           &upstream,
		team:               team,
		account:            account,
		estimate:           estimate,
		quotaWindow:        quotaWindow,
		accountQuotaWindow: accountQuotaWindow,
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
// format), in which case provider/model are simply omitted. It also
// persists the durable request_log audit row for this call - req is
// always available; resp is non-nil only for a successful non-streaming
// call (a streamed call's response is never reconstructed, see
// StreamChatCompletion's doc comment).
func (h *Handler) logCompletion(ctx context.Context, requestID string, principal *Principal, call *preparedCall, start time.Time, req *openai.ChatCompletionRequest, resp *openai.ChatCompletionResponse, err error) {
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
	} else {
		log.FromContext(ctx).Info("chat_completion", append(fields, log.String("status", "success"))...)
	}

	h.recordRequestLog(ctx, requestID, principal, call, start, req, resp, err)
}

// recordRequestLog builds and persists the request_log audit row for one
// proxy call attempt. Persistence failures are logged and swallowed, never
// surfaced to the caller - the same non-blocking posture recordUsage
// already takes for usage_event.
func (h *Handler) recordRequestLog(ctx context.Context, requestID string, principal *Principal, call *preparedCall, start time.Time, req *openai.ChatCompletionRequest, resp *openai.ChatCompletionResponse, err error) {
	requestBody, marshalErr := json.Marshal(req)
	if marshalErr != nil {
		log.FromContext(ctx).Error("failed to marshal request for request_log", log.Error(marshalErr), log.String("request_id", requestID))
		return
	}

	entry := &model.RequestLog{
		RequestID:      requestID,
		AccountID:      principal.ID,
		RequestedModel: req.Model,
		Status:         model.RequestLogStatusSuccess,
		Stream:         req.Stream,
		RequestBody:    string(requestBody),
		LatencyMs:      int(time.Since(start).Milliseconds()),
	}
	if principal.OrgID != "" {
		teamID := principal.OrgID
		entry.TeamID = &teamID
	}
	if call != nil {
		entry.Provider = &call.providerName
		entry.Model = &call.modelName
	}

	if err != nil {
		entry.Status = model.RequestLogStatusError
		msg := err.Error()
		entry.ErrorMessage = &msg
		var perr *provider.Error
		if errors.As(err, &perr) {
			kind := string(perr.Kind)
			entry.ErrorKind = &kind
		}
	} else if resp != nil {
		if body, marshalErr := json.Marshal(resp); marshalErr == nil {
			respBody := string(body)
			entry.ResponseBody = &respBody
		}
		promptTokens, completionTokens, totalTokens := resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens
		entry.PromptTokens = &promptTokens
		entry.CompletionTokens = &completionTokens
		entry.TotalTokens = &totalTokens
		if call != nil {
			cost := usage.CalculateCost(call.providerName, call.modelName, promptTokens, completionTokens)
			entry.CostUsd = &cost
		}
	}

	if err := h.deps.Database.InsertRequestLog(ctx, entry); err != nil {
		log.FromContext(ctx).Error("failed to record request log", log.Error(err), log.String("request_id", requestID))
	}
}

// releaseQuota undoes a reservation for a call that never reached (or
// never completed at) the provider. team and account are independent - a
// nil team (or window) simply skips the team release, and likewise for
// account, so a caller can release just the reservation(s) it actually made.
func (h *Handler) releaseQuota(ctx context.Context, team *model.Team, teamWindow string, account *model.Account, accountWindow string, estimate int) {
	if h.deps.Quota == nil {
		return
	}
	if team != nil {
		_ = h.deps.Quota.Reconcile(ctx, teamWindow, team.MonthlyTokenBudget, estimate, 0)
	}
	if account != nil {
		_ = h.deps.Quota.Reconcile(ctx, accountWindow, account.MonthlyTokenBudget, estimate, 0)
	}
}

// recordUsage reconciles the quota reservations to actual usage and persists
// a durable usage_event row. usage is the real, provider-reported token
// count for the completed call - for a non-streaming call this always
// comes from the response body; for a streaming call, only from adapters
// that can reliably report it over the stream (see StreamChatCompletion).
func (h *Handler) recordUsage(ctx context.Context, requestID string, principal *Principal, call *preparedCall, usg openai.Usage) {
	if h.deps.Quota != nil {
		if call.team != nil {
			_ = h.deps.Quota.Reconcile(ctx, call.quotaWindow, call.team.MonthlyTokenBudget, call.estimate, usg.TotalTokens)
		}
		if call.account != nil {
			_ = h.deps.Quota.Reconcile(ctx, call.accountQuotaWindow, call.account.MonthlyTokenBudget, call.estimate, usg.TotalTokens)
		}
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
		PromptTokens:     usg.PromptTokens,
		CompletionTokens: usg.CompletionTokens,
		TotalTokens:      usg.TotalTokens,
		CostUSD:          usage.CalculateCost(call.providerName, call.modelName, usg.PromptTokens, usg.CompletionTokens),
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
		h.logCompletion(ctx, requestID, principal, nil, start, req, nil, err)
		return nil, err
	}

	resp, err := provider.WithRetry(ctx, maxRetries, func() (*openai.ChatCompletionResponse, error) {
		return call.provider.ChatCompletion(ctx, call.upstream)
	})
	if err != nil {
		h.releaseQuota(ctx, call.team, call.quotaWindow, call.account, call.accountQuotaWindow, call.estimate)
		h.logCompletion(ctx, requestID, principal, call, start, req, nil, err)
		return nil, err
	}

	h.recordUsage(ctx, requestID, principal, call, resp.Usage)
	h.logCompletion(ctx, requestID, principal, call, start, req, resp, nil)
	return resp, nil
}

// StreamChatCompletion handles a streaming chat completion call. Streaming
// calls are not retried - a client already receiving chunks can't safely
// have the request silently restarted underneath it.
//
// Known limitation: not every adapter reliably surfaces a final token-usage
// figure over the stream (Anthropic does, via its message_delta event -
// OpenAI and Gemini streaming usage reporting isn't wired up yet). When the
// adapter doesn't report usage, a successful stream simply keeps its
// upfront estimate reserved (no usage_event is recorded) rather than
// reconciling to a real number. A failed stream still fully releases its
// reservation regardless of provider.
func (h *Handler) StreamChatCompletion(ctx context.Context, requestID string, principal *Principal, req *openai.ChatCompletionRequest) (<-chan provider.StreamEvent, error) {
	start := time.Now()

	call, err := h.prepare(ctx, requestID, principal, req)
	if err != nil {
		h.logCompletion(ctx, requestID, principal, nil, start, req, nil, err)
		return nil, err
	}

	upstream, err := call.provider.StreamChatCompletion(ctx, call.upstream)
	if err != nil {
		h.releaseQuota(ctx, call.team, call.quotaWindow, call.account, call.accountQuotaWindow, call.estimate)
		h.logCompletion(ctx, requestID, principal, call, start, req, nil, err)
		return nil, err
	}

	events := make(chan provider.StreamEvent)
	go func() {
		defer close(events)

		var streamErr error
		var finalUsage *openai.Usage
		for ev := range upstream {
			if ev.Err != nil {
				streamErr = ev.Err
				h.releaseQuota(ctx, call.team, call.quotaWindow, call.account, call.accountQuotaWindow, call.estimate)
			}
			if ev.Usage != nil {
				finalUsage = ev.Usage
			}
			events <- ev
		}

		status := "success"
		if streamErr != nil {
			status = "error"
		} else if finalUsage != nil {
			h.recordUsage(ctx, requestID, principal, call, *finalUsage)
		}
		streamDurationSeconds.WithLabelValues(call.providerName, status).Observe(time.Since(start).Seconds())
		// resp is always nil here: a streamed call's response is never
		// reconstructed into a single body (see this function's doc comment).
		h.logCompletion(ctx, requestID, principal, call, start, req, nil, streamErr)
	}()

	return events, nil
}
