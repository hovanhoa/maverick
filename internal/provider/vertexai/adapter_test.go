package vertexai_test

import (
	"context"
	"testing"

	"github.com/hovanhoa/llmgateway/internal/provider"
	"github.com/hovanhoa/llmgateway/internal/provider/vertexai"
	"github.com/hovanhoa/llmgateway/pkg/openai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdapter_NotImplemented(t *testing.T) {
	t.Parallel()

	adapter := vertexai.New(vertexai.Config{})
	assert.Equal(t, "vertexai", adapter.Name())

	req := &openai.ChatCompletionRequest{Model: "gemini-1.5-pro", Messages: []openai.Message{{Role: openai.RoleUser, Content: "hi"}}}

	_, err := adapter.ChatCompletion(context.Background(), req)
	require.Error(t, err)
	var perr *provider.Error
	require.ErrorAs(t, err, &perr)
	assert.Equal(t, "vertexai", perr.Provider)

	_, err = adapter.StreamChatCompletion(context.Background(), req)
	require.Error(t, err)
}
