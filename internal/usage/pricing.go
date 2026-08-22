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
var pricing = map[string]map[string]modelPricing{
	"anthropic": {
		"claude-3-5-sonnet-20241022": {InputPer1M: 3.00, OutputPer1M: 15.00},
		"claude-3-5-haiku-20241022":  {InputPer1M: 0.80, OutputPer1M: 4.00},
		"claude-3-opus-20240229":     {InputPer1M: 15.00, OutputPer1M: 75.00},
	},
	"openai": {
		"gpt-4o":      {InputPer1M: 2.50, OutputPer1M: 10.00},
		"gpt-4o-mini": {InputPer1M: 0.15, OutputPer1M: 0.60},
	},
	"gemini": {
		"gemini-1.5-pro":   {InputPer1M: 1.25, OutputPer1M: 5.00},
		"gemini-1.5-flash": {InputPer1M: 0.075, OutputPer1M: 0.30},
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
