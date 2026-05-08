package testhttp_test

import (
	"bytes"
	"io"
	"testing"

	"github.com/hovanhoa/llmgateway/pkg/core/http"
	"github.com/hovanhoa/llmgateway/pkg/core/http/testhttp"
	"github.com/stretchr/testify/assert"
)

func TestNewMockClientWithResponse_NoResponseBody(t *testing.T) {
	client := testhttp.NewMockClientWithResponse(http.StatusNotFound, nil)

	req, err := http.NewRequest("GET", "/", bytes.NewBuffer([]byte(`test`)))
	assert.NoError(t, err)

	res, err := client.Do(req)
	assert.NoError(t, err)

	assert.Equal(t, http.StatusNotFound, res.StatusCode)

	body, err := io.ReadAll(res.Body)
	assert.NoError(t, err)
	assert.NoError(t, req.Body.Close())
	assert.Empty(t, body)
}

func TestNewMockClientWithResponse_ResponseBody(t *testing.T) {
	client := testhttp.NewMockClientWithResponse(http.StatusNotFound, []byte(`response`))

	req, err := http.NewRequest("GET", "/", bytes.NewBuffer([]byte(`test`)))
	assert.NoError(t, err)

	res, err := client.Do(req)
	assert.NoError(t, err)

	assert.Equal(t, http.StatusNotFound, res.StatusCode)

	body, err := io.ReadAll(res.Body)
	assert.NoError(t, err)
	assert.NoError(t, req.Body.Close())
	assert.Equal(t, []byte(`response`), body)
}
