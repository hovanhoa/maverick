package apm

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	sentryotel "github.com/getsentry/sentry-go/otel"
	"github.com/hovanhoa/llmgateway/pkg/core/env"
	"github.com/hovanhoa/llmgateway/pkg/core/errors"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

var (
	StartSpan        = sentry.StartSpan
	StartTransaction = sentry.StartTransaction
	SetHubOnContext  = sentry.SetHubOnContext

	SpanFromContext        = sentry.SpanFromContext
	TransactionFromContext = sentry.TransactionFromContext

	SpanStatusOK            = sentry.SpanStatusOK
	SpanStatusInternalError = sentry.SpanStatusInternalError

	TraceHeader   = sentry.SentryTraceHeader
	BaggageHeader = sentry.SentryBaggageHeader

	WithOpName          = sentry.WithOpName
	WithDescription     = sentry.WithDescription
	ContinueFromHeaders = sentry.ContinueFromHeaders
)

type (
	Context   = sentry.Context
	EventHint = sentry.EventHint
	Hub       = sentry.Hub
	Scope     = sentry.Scope
	Span      = sentry.Span
	User      = sentry.User
)

func GetHubFromContext(ctx context.Context) *sentry.Hub {
	if hub := sentry.GetHubFromContext(ctx); hub != nil {
		return hub
	}

	return sentry.CurrentHub().Clone()
}

func ContextWithRequest(ctx context.Context, r *http.Request) context.Context {
	return context.WithValue(ctx, sentry.RequestContextKey, r)
}

func Flush() {
	sentry.Flush(2 * time.Second)
}

func init() {
	sentryDSN := os.Getenv("SENTRY_DSN")
	if sentryDSN == "" {
		return
	}

	if err := sentry.Init(sentry.ClientOptions{
		AttachStacktrace: true,
		Dsn:              sentryDSN,
		EnableTracing:    env.GetEnvironment() == env.Production,
		Environment:      env.GetEnvironment().Name,
		Release:          env.GetBuildCommitHash(),
		// Some errors are okay to ignore
		IgnoreErrors: []string{
			`Workflow execution already finished`,
			`Workflow execution is already running`,
			`context canceled`,
			`unexpected EOF`,
			errors.ErrContinuePolling.Error(),
			errors.ErrQuitPolling.Error(),
		},
		// TODO: Add a filter on transactions so that more important ones get reported
		// instead of just dropping proportionally.
		TracesSampler: sentry.TracesSampler(func(ctx sentry.SamplingContext) float64 {
			// Don't report health checks, flags, or CORS preflight requests
			if strings.HasPrefix(ctx.Span.Name, "GET /health") || strings.HasPrefix(ctx.Span.Name, "GET /flag") || strings.HasPrefix(ctx.Span.Name, "OPTIONS") {
				return 0.0
			}

			// Drop spans that are for GraphQL subscriptions since they are typically long-running
			if ctx.Span.Op == "graphql.subscription" {
				return 0.0
			}

			// Sample based on service
			switch env.CurrentServiceName() {
			case "temporal-worker":
				// Temporal worker is a high volume service, so we sample at 0.1%
				return 0.001

			default:
				// Otherwise sample at a default 10%
				return 0.1
			}
		}),
	}); err != nil {
		panic(errors.Wrap(err, "Sentry initialization failed"))
	}

	otel.SetTextMapPropagator(sentryotel.NewSentryPropagator())
	otel.SetTracerProvider(
		sdktrace.NewTracerProvider(
			sdktrace.WithSpanProcessor(
				sentryotel.NewSentrySpanProcessor(),
			),
		),
	)
}
