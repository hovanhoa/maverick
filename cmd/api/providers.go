package main

import (
	"net/http"
	"time"

	"github.com/hovanhoa/llmgateway/internal/provider"
	"github.com/hovanhoa/llmgateway/internal/provider/anthropic"
	"github.com/hovanhoa/llmgateway/internal/provider/bedrock"
	"github.com/hovanhoa/llmgateway/internal/provider/gemini"
	openaiprovider "github.com/hovanhoa/llmgateway/internal/provider/openai"
	"github.com/hovanhoa/llmgateway/internal/provider/vertexai"
	"github.com/hovanhoa/llmgateway/pkg/core/secrets"
)

// newProviderRegistry builds the set of providers the /v1/chat/completions
// proxy can route to. The API-key-based providers (anthropic, openai,
// gemini) are only registered when their key is configured, so a
// deployment only needs to set the keys for the providers it actually
// uses. bedrock/vertexai are still-scaffold adapters (see their package
// docs) and are always registered so a request for them gets a clear
// "not implemented" error rather than "unknown provider".
func newProviderRegistry() provider.Registry {
	httpClient := &http.Client{Timeout: 120 * time.Second}

	registry := provider.Registry{
		"bedrock":  bedrock.New(bedrock.Config{}),
		"vertexai": vertexai.New(vertexai.Config{}),
	}

	if key := secrets.Get("ANTHROPIC_API_KEY"); key != "" {
		registry["anthropic"] = anthropic.New(httpClient, anthropic.Config{APIKey: key})
	}
	if key := secrets.Get("OPENAI_API_KEY"); key != "" {
		registry["openai"] = openaiprovider.New(httpClient, openaiprovider.Config{APIKey: key})
	}
	if key := secrets.Get("GEMINI_API_KEY"); key != "" {
		registry["gemini"] = gemini.New(httpClient, gemini.Config{APIKey: key})
	}

	return registry
}
