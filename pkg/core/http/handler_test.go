package http_test

import (
	"bytes"
	"context"
	"io"
	"regexp"
	"testing"

	"github.com/hovanhoa/llmgateway/pkg/core/http"
	"github.com/hovanhoa/llmgateway/pkg/core/http/testhttp"
	"github.com/stretchr/testify/assert"
)

func TestHandlerRequest(t *testing.T) {
	type reqType struct {
		A string `param:"id"`
		B bool   `query:"b"`
		C string `header:"X-Test"`
		D int
	}

	service := http.NewService()
	service.Router().GET("/a/:id/test", http.HandleAPIResponse(func(ctx context.Context, req *http.HandlerRequest[reqType]) (*http.HandlerResponse, *http.Error) {
		assert.Equal(t, reqType{"someID", true, "someHeader", 0}, req.Data())
		assert.Equal(t, "/a/someID/test", req.Request().URL.Path)
		assert.Equal(t, "b=true", req.Request().URL.RawQuery)
		assert.Equal(t, req.Request(), req.Context().Request)
		assert.Regexp(t, regexp.MustCompile(`^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}$`), req.UserIP().String())

		return http.HandlerResponseJSON(struct{}{}), nil
	}))

	r := testhttp.NewHTTPTester(t, service).Run(
		testhttp.NewRequestBuilder("GET", "/a/someID/test?b=true").
			WithHeader("X-Test", "someHeader").
			Build(),
	).
		AssertStatusCode(http.StatusOK).
		AssertNoError()

	testhttp.UnmarshalJSONBody[struct{}](r)
}

func TestHandlerRequest_QueryStringSlice(t *testing.T) {
	type reqType struct {
		IDs []string `query:"id"`
	}

	service := http.NewService()
	service.Router().GET("/slice", http.HandleAPIResponse(func(ctx context.Context, req *http.HandlerRequest[reqType]) (*http.HandlerResponse, *http.Error) {
		assert.Equal(t, []string{"a", "b"}, req.Data().IDs)
		return http.HandlerResponseJSON(struct{}{}), nil
	}))

	r := testhttp.NewHTTPTester(t, service).Run(
		testhttp.NewRequestBuilder("GET", "/slice?id=a&id=b").Build(),
	).
		AssertStatusCode(http.StatusOK).
		AssertNoError()

	testhttp.UnmarshalJSONBody[struct{}](r)
}

func TestHandlerRequest_InvalidRequestRead(t *testing.T) {
	type reqType struct {
		A bool `param:"id"`
		B bool `query:"b"`
		C bool `header:"X-Test"`
	}

	service := http.NewService()
	service.Router().GET("/a/:id/test", http.HandleAPIResponse(func(ctx context.Context, req *http.HandlerRequest[reqType]) (*http.HandlerResponse, *http.Error) {
		assert.Fail(t, "should not have run handler")
		return http.HandlerResponseJSON(reqType{}), nil
	}))

	testhttp.NewHTTPTester(t, service).Run(
		testhttp.NewRequestBuilder("GET", "/a/non-bool/test").
			Build(),
	).
		AssertStatusCode(http.StatusBadRequest).
		AssertError(
			http.TypeUnknown,
			http.CodeUnknown,
			"invalid value for \"id\": strconv.ParseBool: parsing \"non-bool\": invalid syntax",
		)

	testhttp.NewHTTPTester(t, service).Run(
		testhttp.NewRequestBuilder("GET", "/a/true/test?b=non-bool").
			Build(),
	).
		AssertStatusCode(http.StatusBadRequest).
		AssertError(
			http.TypeUnknown,
			http.CodeUnknown,
			"invalid value for \"b\": strconv.ParseBool: parsing \"non-bool\": invalid syntax",
		)

	testhttp.NewHTTPTester(t, service).Run(
		testhttp.NewRequestBuilder("GET", "/a/true/test?b=false").
			WithHeader("X-Test", "non-bool").
			Build(),
	).
		AssertStatusCode(http.StatusBadRequest).
		AssertError(
			http.TypeUnknown,
			http.CodeUnknown,
			"invalid value for \"X-Test\": strconv.ParseBool: parsing \"non-bool\": invalid syntax",
		)
}

func TestHandlerRequeset_NonJSONBody(t *testing.T) {
	type reqType struct {
		A string `body:"A"`
	}

	service := http.NewService()
	service.Router().GET("/", http.HandleAPIResponse(func(ctx context.Context, req *http.HandlerRequest[reqType]) (*http.HandlerResponse, *http.Error) {
		assert.Equal(t, reqType{}, req.Data())
		return http.HandlerResponseJSON(struct{}{}), nil
	}))

	r := testhttp.NewHTTPTester(t, service).Run(
		testhttp.NewRequestBuilder("GET", "/").
			WithHeader("Content-type", "application/octet-stream").
			Build(),
	).
		AssertStatusCode(http.StatusOK).
		AssertNoError()

	testhttp.UnmarshalJSONBody[struct{}](r)
}

func TestHandlerRequest_JSONHeaderWithNonJSONBody(t *testing.T) {
	service := http.NewService()
	service.Router().GET("/", http.HandleAPIResponse(func(ctx context.Context, req *http.HandlerRequest[struct{}]) (*http.HandlerResponse, *http.Error) {
		assert.Fail(t, "should not have run handler")
		return http.HandlerResponseJSON(struct{}{}), nil
	}))

	testhttp.NewHTTPTester(t, service).Run(
		testhttp.NewRequestBuilder("GET", "/").
			WithHeader("Content-type", "application/json").
			WithBodyString("invalid-json-body").
			Build(),
	).
		AssertStatusCode(http.StatusInternalServerError)
}

func TestHandlerRequest_JSONBody(t *testing.T) {
	type innerReqType struct {
		BA int
		BB string
		BC []string
		BD map[string]string
	}

	type reqType struct {
		A string                 `body:"A"`
		B innerReqType           `body:"B"`
		C []float64              `body:"C"`
		D map[string]interface{} `body:"D"`
	}

	body := reqType{
		A: "test",
		B: innerReqType{
			BA: 1,
			BB: "test inner",
			BC: []string{"test", "test"},
			BD: map[string]string{"test_key": "test_value"},
		},
		C: []float64{1, 2, 3},
		D: map[string]interface{}{
			"DA": "test",
			"DB": 2.0,
			"DC": map[string]interface{}{
				"DCA": []interface{}{1.0, 2.0, 3.0},
			},
		},
	}

	service := http.NewService()
	service.Router().GET("/", http.HandleAPIResponse(func(ctx context.Context, req *http.HandlerRequest[reqType]) (*http.HandlerResponse, *http.Error) {
		assert.Equal(t, body, req.Data())
		return http.HandlerResponseJSON(struct{}{}), nil
	}))

	r := testhttp.NewHTTPTester(t, service).Run(
		testhttp.NewRequestBuilder("GET", "/").
			WithHeader("Content-type", "application/json").
			WithBodyJSON(body).
			Build(),
	).
		AssertStatusCode(http.StatusOK).
		AssertNoError()

	testhttp.UnmarshalJSONBody[struct{}](r)
}

func TestHandlerRequest_JSONBody_Pointers(t *testing.T) {
	type innerReqType struct {
		BA int
		BB string
		BC []string
		BD map[string]string
	}

	type reqType struct {
		A innerReqType  `body:"A"`
		B innerReqType  `body:"B"`
		C *innerReqType `body:"C"`
		D *innerReqType `body:"D"`
	}

	body := reqType{
		B: innerReqType{
			BA: 1,
			BB: "test inner",
			BC: []string{"test", "test"},
			BD: map[string]string{"test_key": "test_value"},
		},
		D: &innerReqType{
			BA: 1,
			BB: "test inner",
			BC: []string{"test", "test"},
			BD: map[string]string{"test_key": "test_value"},
		},
	}

	service := http.NewService()
	service.Router().GET("/", http.HandleAPIResponse(func(ctx context.Context, req *http.HandlerRequest[reqType]) (*http.HandlerResponse, *http.Error) {
		assert.Equal(t, body, req.Data())
		return http.HandlerResponseJSON(struct{}{}), nil
	}))

	r := testhttp.NewHTTPTester(t, service).Run(
		testhttp.NewRequestBuilder("GET", "/").
			WithHeader("Content-type", "application/json").
			WithBodyJSON(body).
			Build(),
	).
		AssertStatusCode(http.StatusOK).
		AssertNoError()

	testhttp.UnmarshalJSONBody[struct{}](r)
}

func TestHandleAPIResponse(t *testing.T) {
	service := http.NewService()
	service.Router().GET("/", http.HandleAPIResponse(func(ctx context.Context, req *http.HandlerRequest[struct{}]) (*http.HandlerResponse, *http.Error) {
		return http.NewHandlerResponse().
				WithStatus(http.StatusAccepted).
				WithHeader("X-Response-1", "1").
				WithHeader("X-Response-2", "2").
				WithBody(io.NopCloser(bytes.NewReader([]byte("asdf")))),
			nil
	}))

	r := testhttp.NewHTTPTester(t, service).Run(testhttp.NewRequestBuilder("GET", "/").Build()).
		AssertStatusCode(http.StatusAccepted).
		AssertHeader("X-Response-1", "1").
		AssertHeader("X-Response-2", "2")

	assert.Equal(t, []byte("asdf"), r.Body.Bytes())
}

func TestHandleAPIResponse_HandlerError(t *testing.T) {
	service := http.NewService()
	service.Router().GET("/", http.HandleAPIResponse(func(ctx context.Context, req *http.HandlerRequest[struct{}]) (*http.HandlerResponse, *http.Error) {
		return nil, http.NewError(http.StatusPreconditionFailed, "test error")
	}))

	testhttp.NewHTTPTester(t, service).Run(testhttp.NewRequestBuilder("GET", "/").Build()).
		AssertStatusCode(http.StatusPreconditionFailed).
		AssertError(
			http.TypeUnknown,
			http.CodeUnknown,
			"test error",
		)
}

func TestHandleCustomResponse(t *testing.T) {
	service := http.NewService()
	service.Router().GET("/", http.HandleCustomResponse(func(c *http.Context) *http.Error {
		c.JSON(http.StatusAccepted, map[string]string{"Key": "Value"})
		return nil
	}))

	r := testhttp.NewHTTPTester(t, service).Run(testhttp.NewRequestBuilder("GET", "/").Build()).
		AssertStatusCode(http.StatusAccepted).
		AssertNoError()

	assert.Equal(t, []byte(`{"Key":"Value"}`), r.Body.Bytes())
}

func TestHandleCustomResponse_Error(t *testing.T) {
	service := http.NewService()
	service.Router().GET("/", http.HandleCustomResponse(func(c *http.Context) *http.Error {
		return http.NewError(http.StatusBadRequest, "test error")
	}))

	testhttp.NewHTTPTester(t, service).Run(testhttp.NewRequestBuilder("GET", "/").Build()).
		AssertStatusCode(http.StatusBadRequest).
		AssertError(
			http.TypeUnknown,
			http.CodeUnknown,
			"test error",
		)
}
