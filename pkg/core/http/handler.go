package http

import (
	"context"
	"encoding/json"
	"io"
	"maps"
	"mime"
	"net"
	"net/http"
	"reflect"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/hovanhoa/llmgateway/pkg/core/apm"
	"github.com/hovanhoa/llmgateway/pkg/core/errors"
)

// requestReadSource is an opaque type that indicates from where HandlerRequest
// should try to read a particular struct field from. The value indicates the
// source and should be used as the key in a struct tag.
type requestReadSource string

const (
	// Read the struct field from a header with the given name
	requestReadSourceHeader requestReadSource = "header"
	// Read the struct field from a path parameter with the given name
	requestReadSourceParam requestReadSource = "param"
	// Read the struct field from a query parameter with the given name
	requestReadSourceQuery requestReadSource = "query"
	// Read the struct field from given JSON entry in the body
	requestReadSourceBody requestReadSource = "body"
)

// HandlerRequest provides an interface for accessing properties of an HTTP request.
// The type parameter indicates the shape of the data expected by the handler,
// and can be annotated with struct tags to provide instructions on the data source.
type HandlerRequest[T any] struct {
	req T
	c   *Context
}

// Data returns the parsed data from the request, based on the struct tags defined
// on the struct for the type parameter T.
func (r *HandlerRequest[T]) Data() T {
	return r.req
}

// Request returns the http.Request that triggered this handler.
func (r *HandlerRequest[T]) Request() *Request {
	return r.c.Request
}

// Context returns the HTTP context for this request.
func (r *HandlerRequest[T]) Context() *Context {
	return r.c
}

// UserIP returns the IP address of the client that made this HTTP request.
func (r *HandlerRequest[T]) UserIP() net.IP {
	return net.ParseIP(r.c.ClientIP())
}

// getReadSourceAndKey returns the source that should be read for a particular
// field according to its struct tag, and the key within that source to read.
// If a valid key is not provided, the method returns false as the final parameter.
func getReadSourceAndKey(tag reflect.StructTag) (requestReadSource, string, bool) {
	if v, ok := tag.Lookup(string(requestReadSourceHeader)); ok {
		return requestReadSourceHeader, v, true
	}
	if v, ok := tag.Lookup(string(requestReadSourceParam)); ok {
		return requestReadSourceParam, v, true
	}
	if v, ok := tag.Lookup(string(requestReadSourceQuery)); ok {
		return requestReadSourceQuery, v, true
	}
	if v, ok := tag.Lookup(string(requestReadSourceBody)); ok {
		return requestReadSourceBody, v, true
	}
	return "", "", false
}

// parseRequestSource parses `val` based on the type of the struct field `f`
// and sets `f` if possible. It returns an error if the function failed to
// parse `val` or set `f`.
func parseRequestSource(f reflect.Value, val string, readKey string) *Error {
	if !f.CanSet() || val == "" {
		return nil
	}

	switch f.Kind() {
	case reflect.String:
		f.SetString(val)

	case reflect.Bool:
		parsedVal, err := strconv.ParseBool(val)
		if err != nil {
			return NewError(StatusBadRequest, "invalid value for %q", readKey).With(CauseOption(err))
		}

		f.SetBool(parsedVal)
	}

	return nil
}

// ReadRequest takes the request context and type parameter indicating the desired
// shape of the request data and produces an APIRequest object instantiated with
// data of that type, reading the subfields according to the struct tags.
func ReadRequest[T any](c *Context) (*HandlerRequest[T], *Error) {
	// create an empty handler request with type parameter T
	req := HandlerRequest[T]{c: c}

	// sanity check the type parameter is a struct
	rv := reflect.Indirect(reflect.ValueOf(&req.req))
	if rv.Kind() != reflect.Struct {
		panic(errors.New("invalid kind of type parameter: %v, expected struct", rv.Kind()))
	}

	// check if the request has a body, and if so, shallow-read it.
	var body map[string]json.RawMessage
	if c.Request.Body != nil {
		mediaType, _, err := mime.ParseMediaType(c.Request.Header.Get("content-type"))
		if err == nil && mediaType == "application/json" {
			if err := c.ShouldBindJSON(&body); err != nil && err != io.EOF {
				return nil, NewInternalServerError().With(CauseOption(errors.Wrap(err, "ShouldBindJSON")))
			}
		}
	}

	// for each field in T, read its struct tag to determine where to populate its
	// data from, read that data source, and set the value on the struct.
	for i := 0; i < rv.NumField(); i++ {
		f := rv.Field(i)
		t := rv.Type().Field(i)

		if !f.CanSet() {
			continue
		}

		// determine the read source and key
		readFrom, readKey, ok := getReadSourceAndKey(t.Tag)
		if !ok {
			continue
		}

		switch readFrom {
		// handle reading from the request headers
		case requestReadSourceHeader:
			if err := parseRequestSource(f, c.Request.Header.Get(readKey), readKey); err != nil {
				return nil, err
			}

		// handle reading from the request path parameters
		case requestReadSourceParam:
			if err := parseRequestSource(f, c.Param(readKey), readKey); err != nil {
				return nil, err
			}

		// handle reading from the request query string
		case requestReadSourceQuery:
			// Repeated query keys populate []string only; other slice element kinds still use Query()
			// (single value) via parseRequestSource below.
			// For slice fields, collect all repeated values (e.g. ?foo=a&foo=b → []string{"a","b"}).
			if f.Kind() == reflect.Slice && f.Type().Elem().Kind() == reflect.String {
				vals := c.QueryArray(readKey)
				if len(vals) > 0 {
					f.Set(reflect.ValueOf(vals))
				}
			} else if err := parseRequestSource(f, c.Query(readKey), readKey); err != nil {
				return nil, err
			}

		// handle reading from the body
		case requestReadSourceBody:
			val, ok := body[readKey]
			if !ok {
				continue
			}

			refVal := reflect.New(t.Type)
			if err := json.Unmarshal(val, refVal.Interface()); err != nil {
				return nil, NewInternalServerError().With(CauseOption(err))
			}

			f.Set(reflect.Indirect(refVal))
		}
	}

	return &req, nil
}

// HandlerResponse provides methods to set HTTP response metadata and provide
// and object to return as the response body of the HTTP request.
type HandlerResponse struct {
	writeBody func(w http.ResponseWriter) error
	header    http.Header
	status    int
}

// HandlerResponseJSON creates a response that returns the given object with
// response code 200.
func HandlerResponseJSON(res interface{}) *HandlerResponse {
	header := make(http.Header)
	header.Set("Content-Type", "application/json;charset=utf-8")

	return &HandlerResponse{
		writeBody: func(w http.ResponseWriter) error {
			data, err := json.Marshal(res)
			if err != nil {
				return errors.Wrap(err, "json.Marshal")
			}

			_, err = w.Write(data)
			return err
		},
		header: header,
		status: StatusOK,
	}
}

// NewHandlerResponse creates a blank response with status 200.
func NewHandlerResponse() *HandlerResponse {
	return &HandlerResponse{
		header: make(http.Header),
		status: StatusOK,
	}
}

// WithStatus sets the HTTP status code on the response.
func (r HandlerResponse) WithStatus(statusCode int) *HandlerResponse {
	r.status = statusCode
	return &r
}

// WithBody sets the body to write back to the client.
func (r HandlerResponse) WithBody(body io.ReadCloser) *HandlerResponse {
	r.writeBody = func(w http.ResponseWriter) error {
		defer func() { _ = body.Close() }()
		_, err := io.Copy(w, body)
		return err
	}
	return &r
}

// WithHeader sets the given key-value pair as a header on the response.
func (r HandlerResponse) WithHeader(key string, value string) *HandlerResponse {
	r.header.Set(key, value)
	return &r
}

// ResponseHandler defines the type of an API response handler.
type ResponseHandler[T any] func(ctx context.Context, req *HandlerRequest[T]) (*HandlerResponse, *Error)

// HandleAPIResponse parses the request based on the ReqType type parameter,
// calls the handler with the parsed request, and uses the handler's response
// to construct an HTTP response to the user.
func HandleAPIResponse[T any](handler ResponseHandler[T]) HandlerFunc {
	return func(c *Context) {
		req, err := ReadRequest[T](c)
		if err != nil {
			handleError(c, err)
			return
		}

		res, err := handler(c.Request.Context(), req)
		if err != nil {
			handleError(c, err)
			return
		}

		c.Status(res.status)
		for key := range res.header {
			c.Writer.Header().Add(key, res.header.Get(key))
		}

		writeErr := res.writeBody(c.Writer)
		if writeErr != nil {
			panic(writeErr)
		}
	}
}

// HandleCustomResponse allows for the implementation of middleware using
// this HTTP framework.
func HandleCustomResponse(handler func(c *Context) *Error) HandlerFunc {
	return func(c *Context) {
		if err := handler(c); err != nil {
			handleError(c, err)
		}
	}
}

func FromHTTPHandler(handler http.Handler) HandlerFunc {
	return func(c *Context) {
		var errorSetter RequestErrorSetter = func(err error) {
			c.Set(string(RequestErrorContextKey), err)
		}

		var extraFieldsSetter RequestExtraFieldsSetter = func(fields map[string]interface{}) {
			extraFields := make(map[string]interface{})
			extraFieldsValue, ok := c.Get(string(RequestExtraFieldsContextKey))
			if ok {
				extraFields = extraFieldsValue.(map[string]interface{})
			}

			maps.Copy(extraFields, fields)
			c.Set(string(RequestExtraFieldsContextKey), extraFields)
		}

		c.Set(string(RequestErrorSetterContextKey), errorSetter)
		c.Set(string(RequestExtraFieldsSetterContextKey), extraFieldsSetter)

		handler.ServeHTTP(
			c.Writer,
			c.Request.WithContext(
				context.WithValue(
					context.WithValue(
						c.Request.Context(),
						RequestErrorSetterContextKey,
						errorSetter,
					),
					RequestExtraFieldsSetterContextKey,
					extraFieldsSetter,
				),
			),
		)
	}
}

func handleError(c *Context, err *Error) {
	// If there was an error, don't run any pending handlers.
	c.Abort()

	// If the error was an authorization error, clear the auth cookie
	// and return a message indicating that the user must log in
	if err.StatusCode() == http.StatusUnauthorized {
		UnsetAuthCookie(c.Writer, GetSessionCookieID())
	}

	// if the cause of this error is not an internal server error, send it back as is
	if err.StatusCode() < http.StatusInternalServerError {
		c.JSON(err.StatusCode(), gin.H{"Error": err})
		return
	}

	// Log the error as part of this transaction
	if hub := apm.GetHubFromContext(c.Request.Context()); hub != nil {
		if client, scope := hub.Client(), hub.Scope(); client != nil {
			client.CaptureException(err, &apm.EventHint{Context: c.Request.Context(), OriginalException: err}, scope)
		}
	}

	// Ensure error is written in the canonical log line
	c.Set(string(RequestErrorContextKey), err)

	// Return a generic error to the user
	c.JSON(err.StatusCode(), gin.H{"Error": NewInternalServerError()})
}

type ErrorResponse struct {
	Error struct {
		Message string
	}
}
