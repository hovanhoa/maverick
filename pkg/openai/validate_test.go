package openai_test

import (
	"testing"

	"github.com/hovanhoa/llmgateway/pkg/openai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validRequest() openai.ChatCompletionRequest {
	return openai.ChatCompletionRequest{
		Model:    "gpt-4o",
		Messages: []openai.Message{{Role: openai.RoleUser, Content: "hi"}},
	}
}

func TestValidate_Valid(t *testing.T) {
	t.Parallel()
	req := validRequest()
	require.NoError(t, req.Validate())
}

func TestValidate_MissingModel(t *testing.T) {
	t.Parallel()
	req := validRequest()
	req.Model = ""
	err := req.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "model is required")
}

func TestValidate_NoMessages(t *testing.T) {
	t.Parallel()
	req := validRequest()
	req.Messages = nil
	err := req.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "messages must not be empty")
}

func TestValidate_InvalidRole(t *testing.T) {
	t.Parallel()
	req := validRequest()
	req.Messages[0].Role = "tool"
	err := req.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "role must be one of")
}

func TestValidate_EmptyContent(t *testing.T) {
	t.Parallel()
	req := validRequest()
	req.Messages[0].Content = ""
	err := req.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "content must not be empty")
}

func TestValidate_TemperatureOutOfRange(t *testing.T) {
	t.Parallel()
	req := validRequest()
	bad := 2.5
	req.Temperature = &bad
	err := req.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "temperature must be between 0 and 2")
}

func TestValidate_TopPOutOfRange(t *testing.T) {
	t.Parallel()
	req := validRequest()
	bad := 1.5
	req.TopP = &bad
	err := req.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "top_p must be between 0 and 1")
}

func TestValidate_MaxTokensNotPositive(t *testing.T) {
	t.Parallel()
	req := validRequest()
	bad := 0
	req.MaxTokens = &bad
	err := req.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "max_tokens must be positive")
}
