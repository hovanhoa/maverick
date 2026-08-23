// Package usage extracts and normalizes provider token/cost accounting for
// the LLM proxy. Pricing here is illustrative/approximate - not a
// live-synced price list - and is only ever used to produce a rough cost
// estimate for reporting; it never blocks or alters a request.
package usage

// modelPricing is USD cost per 1M tokens, input (prompt) and output
// (completion) priced separately since most providers charge them
// differently.
type modelPricing struct {
	InputPer1M  float64
	OutputPer1M float64
}

// pricing is keyed by provider name, then by the bare model name (no
// "provider/" prefix). Unrecognized provider/model pairs simply cost $0
// rather than erroring - see CalculateCost.
//
// Snapshot as of 2026-08-22, from each provider's public pricing page.
// Provider model lineups and prices change often (an end-to-end proxy test
// this same day found claude-3-5-haiku-20241022 - what this table had
// before - already retired); this table needs a manual refresh whenever a
// provider ships new models, since nothing here is live-synced (see the
// package doc and the Phase 6 roadmap item on this).
var pricing = map[string]map[string]modelPricing{
	"anthropic": {
		"claude-opus-5":              {InputPer1M: 5.00, OutputPer1M: 25.00},
		"claude-opus-4-8":            {InputPer1M: 5.00, OutputPer1M: 25.00},
		"claude-opus-4-7":            {InputPer1M: 5.00, OutputPer1M: 25.00},
		"claude-opus-4-6":            {InputPer1M: 5.00, OutputPer1M: 25.00},
		"claude-opus-4-5-20251101":   {InputPer1M: 5.00, OutputPer1M: 25.00},
		"claude-sonnet-5":            {InputPer1M: 2.00, OutputPer1M: 10.00},
		"claude-sonnet-4-6":          {InputPer1M: 3.00, OutputPer1M: 15.00},
		"claude-sonnet-4-5-20250929": {InputPer1M: 3.00, OutputPer1M: 15.00},
		"claude-haiku-4-5-20251001":  {InputPer1M: 1.00, OutputPer1M: 5.00},
	},
	"openai": {
		"gpt-5":       {InputPer1M: 1.25, OutputPer1M: 10.00},
		"gpt-5-mini":  {InputPer1M: 0.25, OutputPer1M: 2.00},
		"gpt-4o":      {InputPer1M: 2.50, OutputPer1M: 10.00},
		"gpt-4o-mini": {InputPer1M: 0.15, OutputPer1M: 0.60},
	},
	"gemini": {
		"gemini-2.5-pro":        {InputPer1M: 1.25, OutputPer1M: 10.00},
		"gemini-2.5-flash":      {InputPer1M: 0.30, OutputPer1M: 2.50},
		"gemini-2.5-flash-lite": {InputPer1M: 0.10, OutputPer1M: 0.40},
	},
}

// CalculateCost returns the estimated USD cost of a completed call. Cost
// estimation must never block a response, so an unrecognized provider or
// model - a new model the pricing table hasn't caught up with yet, for
// example - simply costs $0 rather than erroring.
func CalculateCost(providerName, modelName string, promptTokens, completionTokens int) float64 {
	models, ok := pricing[providerName]
	if !ok {
		return 0
	}
	price, ok := models[modelName]
	if !ok {
		return 0
	}

	return (float64(promptTokens)/1_000_000)*price.InputPer1M + (float64(completionTokens)/1_000_000)*price.OutputPer1M
}
