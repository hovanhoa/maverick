package bedrock_test

import (
	"context"
	"testing"

	"github.com/hovanhoa/llmgateway/internal/provider"
	"github.com/hovanhoa/llmgateway/internal/provider/bedrock"
	"github.com/hovanhoa/llmgateway/pkg/openai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdapter_NotImplemented(t *testing.T) {
	t.Parallel()

	adapter := bedrock.New(bedrock.Config{})
	assert.Equal(t, "bedrock", adapter.Name())

	req := &openai.ChatCompletionRequest{Model: "anthropic.claude-3", Messages: []openai.Message{{Role: openai.RoleUser, Content: "hi"}}}

	_, err := adapter.ChatCompletion(context.Background(), req)
	require.Error(t, err)
	var perr *provider.Error
	require.ErrorAs(t, err, &perr)
	assert.Equal(t, "bedrock", perr.Provider)

	_, err = adapter.StreamChatCompletion(context.Background(), req)
	require.Error(t, err)
}
