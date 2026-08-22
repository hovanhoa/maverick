package main

import (
	"time"

	"github.com/hovanhoa/llmgateway/internal/provider"
	"github.com/hovanhoa/llmgateway/internal/provider/anthropic"
	"github.com/hovanhoa/llmgateway/internal/provider/bedrock"
	"github.com/hovanhoa/llmgateway/internal/provider/gemini"
	openaiprovider "github.com/hovanhoa/llmgateway/internal/provider/openai"
	"github.com/hovanhoa/llmgateway/internal/provider/vertexai"
	corehttp "github.com/hovanhoa/llmgateway/pkg/core/http"
	"github.com/hovanhoa/llmgateway/pkg/core/secrets"
)

// providerHTTPClient returns an HTTP client for the named provider,
// instrumented with Prometheus metrics and a Sentry/OTel span for every
// outbound call (see corehttp.NewInstrumentedClient) - this is what makes
// provider throughput/latency/error metrics and tracing (Phase 5) exist at
// all. Body logging is disabled: a provider call's body is the user's raw
// prompt and the model's raw completion, which must not be written to
// logs verbatim.
func providerHTTPClient(name string) *corehttp.Client {
	client := corehttp.NewInstrumentedClient(name, corehttp.WithoutBodyLogging())
	client.Timeout = 120 * time.Second
	return client
}

// newProviderRegistry builds the set of providers the /v1/chat/completions
// proxy can route to. The API-key-based providers (anthropic, openai,
// gemini) are only registered when their key is configured, so a
// deployment only needs to set the keys for the providers it actually
// uses. bedrock/vertexai are still-scaffold adapters (see their package
// docs) and are always registered so a request for them gets a clear
// "not implemented" error rather than "unknown provider".
func newProviderRegistry() provider.Registry {
	registry := provider.Registry{
		"bedrock":  bedrock.New(bedrock.Config{}),
		"vertexai": vertexai.New(vertexai.Config{}),
	}

	if key := secrets.Get("ANTHROPIC_API_KEY"); key != "" {
		registry["anthropic"] = anthropic.New(providerHTTPClient("anthropic"), anthropic.Config{APIKey: key})
	}
	if key := secrets.Get("OPENAI_API_KEY"); key != "" {
		registry["openai"] = openaiprovider.New(providerHTTPClient("openai"), openaiprovider.Config{APIKey: key})
	}
	if key := secrets.Get("GEMINI_API_KEY"); key != "" {
		registry["gemini"] = gemini.New(providerHTTPClient("gemini"), gemini.Config{APIKey: key})
	}

	return registry
}
