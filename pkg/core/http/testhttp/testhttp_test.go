package testhttp_test

import (
	"context"
	"io"
	"net/url"
	"testing"

	"github.com/hovanhoa/llmgateway/pkg/core/http"
	"github.com/hovanhoa/llmgateway/pkg/core/http/testhttp"
	"github.com/stretchr/testify/assert"
)

func TestHTTPTester_Success(t *testing.T) {
	type reqType struct {
		Key string
	}

	service := http.NewService()
	service.Router().POST("/test", http.HandleAPIResponse(func(ctx context.Context, req *http.HandlerRequest[reqType]) (*http.HandlerResponse, *http.Error) {
		return http.HandlerResponseJSON(reqType{"Value"}).
				WithHeader("X-Test-1", "1").
				WithStatus(http.StatusAccepted),
			nil
	}))

	tester := testhttp.NewHTTPTester(t, service)
	r := tester.Run(
		testhttp.NewRequestBuilder("POST", "/test").
			WithBodyJSON(map[string]string{"Key": "Value"}).
			Build(),
	).
		AssertStatusCode(http.StatusAccepted).
		AssertNonEmptyHeader("X-Test-1").
		AssertHeader("X-Test-1", "1").
		AssertNoError()

	b := testhttp.UnmarshalJSONBody[reqType](r)
	assert.Equal(t, "Value", b.Key)

	tester.AssertRequestsLen(1)
	tester.Request(0).AssertMethod("POST").AssertPathEquals("/test")
}

func TestHTTPTester_Error(t *testing.T) {
	service := http.NewService()
	service.Router().POST("/test", http.HandleAPIResponse(func(ctx context.Context, req *http.HandlerRequest[struct{}]) (*http.HandlerResponse, *http.Error) {
		return nil, http.NewError(http.StatusBadRequest, "test error").With(
			http.FieldOption("Extra", "Test"),
			http.ErrorTypeOption(http.TypeProcessingError),
		)
	}))

	testhttp.NewHTTPTester(t, service).Run(
		testhttp.NewRequestBuilder("POST", "/test").
			Build(),
	).
		AssertStatusCode(http.StatusBadRequest).
		AssertError(
			http.TypeProcessingError,
			http.CodeUnknown,
			"test error",
			testhttp.NewErrorField("Extra", "Test"),
		)
}

func TestConnectedHTTPTester(t *testing.T) {
	type reqType struct {
		Key string
	}

	service := http.NewService()
	service.Router().POST("/test", http.HandleAPIResponse(func(ctx context.Context, req *http.HandlerRequest[reqType]) (*http.HandlerResponse, *http.Error) {
		return http.HandlerResponseJSON(reqType{"Value"}).
				WithHeader("X-Test-1", "1").
				WithStatus(http.StatusAccepted),
			nil
	}))

	client, tester := testhttp.NewConnectedHTTPTester(t, service)
	req, err := http.NewRequest("POST", "/test", nil)
	assert.NoError(t, err)

	res, err := client.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusAccepted, res.StatusCode)
	assert.Equal(t, "1", res.Header.Get("X-Test-1"))

	data, err := io.ReadAll(res.Body)
	assert.NoError(t, err)
	assert.NoError(t, res.Body.Close())
	assert.Equal(t, []byte(`{"Key":"Value"}`), data)

	tester.AssertRequestsLen(1)
	tester.Request(0).AssertMethod("POST").AssertPathEquals("/test")
}

func TestConnectedHTTPTester_Error(t *testing.T) {
	type reqType struct {
		Key string
	}

	service := http.NewService()
	service.Router().POST("/test", http.HandleAPIResponse(func(ctx context.Context, req *http.HandlerRequest[reqType]) (*http.HandlerResponse, *http.Error) {
		return http.HandlerResponseJSON(reqType{"Value"}).
				WithHeader("X-Test-1", "1").
				WithStatus(http.StatusAccepted),
			nil
	}))

	url, err := url.Parse("/test")
	assert.NoError(t, err)
	req := http.Request{
		Method: "\n",
		URL:    url,
	}

	client, tester := testhttp.NewConnectedHTTPTester(t, service)
	_, err = client.Do(&req)
	assert.Error(t, err)
	tester.AssertRequestsLen(0)
}
