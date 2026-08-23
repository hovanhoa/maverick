package usage_test

import (
	"testing"

	"github.com/hovanhoa/llmgateway/internal/usage"
	"github.com/stretchr/testify/assert"
)

func TestCalculateCost_KnownModel(t *testing.T) {
	t.Parallel()

	cost := usage.CalculateCost("openai", "gpt-4o", 1_000_000, 1_000_000)
	assert.InDelta(t, 12.50, cost, 0.0001)
}

func TestCalculateCost_ZeroTokens(t *testing.T) {
	t.Parallel()

	cost := usage.CalculateCost("anthropic", "claude-sonnet-5", 0, 0)
	assert.Zero(t, cost)
}

func TestCalculateCost_UnknownProviderIsZero(t *testing.T) {
	t.Parallel()

	cost := usage.CalculateCost("mistral", "large", 1000, 1000)
	assert.Zero(t, cost)
}

func TestCalculateCost_UnknownModelIsZero(t *testing.T) {
	t.Parallel()

	cost := usage.CalculateCost("openai", "gpt-99", 1000, 1000)
	assert.Zero(t, cost)
}

func TestCalculateCost_InputAndOutputPricedSeparately(t *testing.T) {
	t.Parallel()

	inputOnly := usage.CalculateCost("gemini", "gemini-2.5-flash", 1_000_000, 0)
	outputOnly := usage.CalculateCost("gemini", "gemini-2.5-flash", 0, 1_000_000)
	assert.InDelta(t, 0.30, inputOnly, 0.0001)
	assert.InDelta(t, 2.50, outputOnly, 0.0001)
	assert.NotEqual(t, inputOnly, outputOnly)
}
