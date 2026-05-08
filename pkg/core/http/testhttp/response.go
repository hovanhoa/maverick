package testhttp

import (
	"encoding/json"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/hovanhoa/llmgateway/pkg/core/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ResponseAsserter wraps a response from the HTTP service under test
// and provides some methods to perform common assertions against it.
type ResponseAsserter struct {
	*httptest.ResponseRecorder
	t *testing.T
}

// AssertStatusCode asserts that the reponse has the given status code.
func (r ResponseAsserter) AssertStatusCode(expectedCode int) ResponseAsserter {
	assert.Equal(r.t, expectedCode, r.Code)
	return r
}

// AssertHeader asserts that the response has the given key-value pair as a header.
func (r ResponseAsserter) AssertHeader(key, value string) ResponseAsserter {
	assert.Equal(r.t, value, r.Result().Header.Get(key))
	return r
}

// AssertHeaderMatches asserts that the response has a header that matches the given regex.
func (r ResponseAsserter) AssertHeaderMatches(key string, regex *regexp.Regexp) ResponseAsserter {
	assert.Regexp(r.t, regex, r.Result().Header.Get(key))
	return r
}

func (r ResponseAsserter) AssertNonEmptyHeader(key string) ResponseAsserter {
	assert.NotEmpty(r.t, r.Result().Header.Get(key))
	return r
}

// AssertNoError asserts that the response did not return a JSON error.
func (r ResponseAsserter) AssertNoError() ResponseAsserter {
	e := make(map[string]json.RawMessage)

	err := json.Unmarshal(r.Body.Bytes(), &e)
	require.NoError(r.t, err)

	_, ok := e["Error"]
	assert.False(r.t, ok, "Response should not have an error. Got: %s", r.Body.String())

	return r
}

// AssertError checks that a JSON error is returned in the body of this response
// with the given type, code, message, and optional extra error fields.
func (r ResponseAsserter) AssertError(errorType http.ErrorType, errorCode http.ErrorCode, msg string, extraFields ...ErrorField) ResponseAsserter {
	e := make(map[string]json.RawMessage)

	err := json.Unmarshal(r.Body.Bytes(), &e)
	require.NoError(r.t, err)

	errData, ok := e["Error"]
	require.True(r.t, ok, "Response does not have an error")

	e2 := make(map[string]string)

	err = json.Unmarshal(errData, &e2)
	require.NoError(r.t, err)

	assert.Equal(r.t, string(errorType), e2["Type"])
	assert.Equal(r.t, string(errorCode), e2["Code"])
	assert.Equal(r.t, msg, e2["Message"])

	for _, pair := range extraFields {
		assert.Equal(r.t, pair.Value, e2[pair.Key])
	}

	return r
}

// UnmarshalJSONBody reads the given response's body and attempts to
// read it into an object of the given type, requiring that no error
// occurs along the way.
func UnmarshalJSONBody[T any](r ResponseAsserter) (d T) {
	require.NoError(r.t, json.Unmarshal(r.Body.Bytes(), &d))
	return d
}

type ErrorField struct {
	Key   string
	Value string
}

// NewErrorField provides a key-value pair to AssertError that checks that
// the given key-value pair exists on the error response.
func NewErrorField(key, value string) ErrorField { return ErrorField{key, value} }
