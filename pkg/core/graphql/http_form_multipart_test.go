package graphql

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/errcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

func TestSupports(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		method   string
		content  string
		upgrade  string
		expected bool
	}{
		{
			name:     "POST with multipart/form-data and boundary",
			method:   "POST",
			content:  "multipart/form-data; boundary=something",
			expected: true,
		},
		{
			name:     "POST with multipart/form-data no params",
			method:   "POST",
			content:  "multipart/form-data",
			expected: true,
		},
		{
			name:     "GET with multipart/form-data",
			method:   "GET",
			content:  "multipart/form-data; boundary=something",
			expected: false,
		},
		{
			name:     "PUT with multipart/form-data",
			method:   "PUT",
			content:  "multipart/form-data; boundary=something",
			expected: false,
		},
		{
			name:     "POST with application/json",
			method:   "POST",
			content:  "application/json",
			expected: false,
		},
		{
			name:     "POST with empty content type",
			method:   "POST",
			content:  "",
			expected: false,
		},
		{
			name:     "POST with invalid content type",
			method:   "POST",
			content:  "not a valid media type %%%",
			expected: false,
		},
		{
			name:     "POST with multipart/form-data but upgrade header set",
			method:   "POST",
			content:  "multipart/form-data; boundary=something",
			upgrade:  "websocket",
			expected: false,
		},
		{
			name:     "DELETE with multipart/form-data",
			method:   "DELETE",
			content:  "multipart/form-data; boundary=something",
			expected: false,
		},
		{
			name:     "PATCH with multipart/form-data",
			method:   "PATCH",
			content:  "multipart/form-data; boundary=something",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := httptest.NewRequest(tt.method, "/graphql", nil)
			if tt.content != "" {
				r.Header.Set("Content-Type", tt.content)
			}
			if tt.upgrade != "" {
				r.Header.Set("Upgrade", tt.upgrade)
			}

			f := MultipartForm{}
			assert.Equal(t, tt.expected, f.Supports(r))
		})
	}
}

func TestMaxUploadSize(t *testing.T) {
	t.Parallel()

	t.Run("returns default 32MB when zero", func(t *testing.T) {
		t.Parallel()
		f := MultipartForm{}
		assert.Equal(t, int64(32<<20), f.maxUploadSize())
	})

	t.Run("returns custom value when set", func(t *testing.T) {
		t.Parallel()
		f := MultipartForm{MaxUploadSize: 64 << 20}
		assert.Equal(t, int64(64<<20), f.maxUploadSize())
	})

	t.Run("returns small value when set", func(t *testing.T) {
		t.Parallel()
		f := MultipartForm{MaxUploadSize: 1024}
		assert.Equal(t, int64(1024), f.maxUploadSize())
	})
}

func TestMaxMemory(t *testing.T) {
	t.Parallel()

	t.Run("returns default 32MB when zero", func(t *testing.T) {
		t.Parallel()
		f := MultipartForm{}
		assert.Equal(t, int64(32<<20), f.maxMemory())
	})

	t.Run("returns custom value when set", func(t *testing.T) {
		t.Parallel()
		f := MultipartForm{MaxMemory: 16 << 20}
		assert.Equal(t, int64(16<<20), f.maxMemory())
	})

	t.Run("returns small value when set", func(t *testing.T) {
		t.Parallel()
		f := MultipartForm{MaxMemory: 2048}
		assert.Equal(t, int64(2048), f.maxMemory())
	})
}

func TestWriteHeaders(t *testing.T) {
	t.Parallel()

	t.Run("writes default content-type when nil headers", func(t *testing.T) {
		t.Parallel()
		w := httptest.NewRecorder()
		writeHeaders(w, nil)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	})

	t.Run("writes default content-type when empty map", func(t *testing.T) {
		t.Parallel()
		w := httptest.NewRecorder()
		writeHeaders(w, map[string][]string{})
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	})

	t.Run("writes custom headers with multiple values", func(t *testing.T) {
		t.Parallel()
		w := httptest.NewRecorder()
		writeHeaders(w, map[string][]string{
			"X-Custom": {"value1", "value2"},
			"X-Other":  {"other-value"},
		})
		assert.Equal(t, []string{"value1", "value2"}, w.Header().Values("X-Custom"))
		assert.Equal(t, "other-value", w.Header().Get("X-Other"))
		// When custom headers are provided, Content-Type is NOT automatically added
		assert.Empty(t, w.Header().Get("Content-Type"))
	})

	t.Run("custom headers can override content-type", func(t *testing.T) {
		t.Parallel()
		w := httptest.NewRecorder()
		writeHeaders(w, map[string][]string{
			"Content-Type": {"application/graphql-response+json"},
		})
		assert.Equal(t, "application/graphql-response+json", w.Header().Get("Content-Type"))
	})
}

func TestWriteJson(t *testing.T) {
	t.Parallel()

	t.Run("writes response with data", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		resp := &graphql.Response{
			Data: json.RawMessage(`{"hello":"world"}`),
		}
		writeJson(&buf, resp)

		var result map[string]json.RawMessage
		err := json.Unmarshal(buf.Bytes(), &result)
		require.NoError(t, err)
		assert.JSONEq(t, `{"hello":"world"}`, string(result["data"]))
	})

	t.Run("writes response with errors", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		resp := &graphql.Response{
			Errors: gqlerror.List{{Message: "something went wrong"}},
		}
		writeJson(&buf, resp)

		var result struct {
			Errors []struct {
				Message string `json:"message"`
			} `json:"errors"`
		}
		err := json.Unmarshal(buf.Bytes(), &result)
		require.NoError(t, err)
		require.Len(t, result.Errors, 1)
		assert.Equal(t, "something went wrong", result.Errors[0].Message)
	})

	t.Run("writes empty response", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		resp := &graphql.Response{}
		writeJson(&buf, resp)
		assert.JSONEq(t, `{"data":null}`, buf.String())
	})
}

func TestWriteJsonError(t *testing.T) {
	t.Parallel()

	t.Run("writes error message", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		writeJsonError(&buf, "upload failed")

		var result struct {
			Errors []struct {
				Message string `json:"message"`
			} `json:"errors"`
		}
		err := json.Unmarshal(buf.Bytes(), &result)
		require.NoError(t, err)
		require.Len(t, result.Errors, 1)
		assert.Equal(t, "upload failed", result.Errors[0].Message)
	})

	t.Run("writes empty string message", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		writeJsonError(&buf, "")

		var result struct {
			Errors []struct {
				Message string `json:"message"`
			} `json:"errors"`
		}
		err := json.Unmarshal(buf.Bytes(), &result)
		require.NoError(t, err)
		require.Len(t, result.Errors, 1)
		assert.Equal(t, "", result.Errors[0].Message)
	})
}

func TestWriteJsonErrorf(t *testing.T) {
	t.Parallel()

	t.Run("formats message with args", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		writeJsonErrorf(&buf, "failed to process key %s with code %d", "abc", 42)

		var result struct {
			Errors []struct {
				Message string `json:"message"`
			} `json:"errors"`
		}
		err := json.Unmarshal(buf.Bytes(), &result)
		require.NoError(t, err)
		require.Len(t, result.Errors, 1)
		assert.Equal(t, "failed to process key abc with code 42", result.Errors[0].Message)
	})

	t.Run("formats message without args", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		writeJsonErrorf(&buf, "simple error")

		var result struct {
			Errors []struct {
				Message string `json:"message"`
			} `json:"errors"`
		}
		err := json.Unmarshal(buf.Bytes(), &result)
		require.NoError(t, err)
		require.Len(t, result.Errors, 1)
		assert.Equal(t, "simple error", result.Errors[0].Message)
	})
}

func TestWriteJsonGraphqlError(t *testing.T) {
	t.Parallel()

	t.Run("writes single graphql error", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		gqlErr := &gqlerror.Error{
			Message: "graphql validation error",
			Extensions: map[string]interface{}{
				"code": "VALIDATION_FAILED",
			},
		}
		writeJsonGraphqlError(&buf, gqlErr)

		var result struct {
			Errors []struct {
				Message    string                 `json:"message"`
				Extensions map[string]interface{} `json:"extensions"`
			} `json:"errors"`
		}
		err := json.Unmarshal(buf.Bytes(), &result)
		require.NoError(t, err)
		require.Len(t, result.Errors, 1)
		assert.Equal(t, "graphql validation error", result.Errors[0].Message)
		assert.Equal(t, "VALIDATION_FAILED", result.Errors[0].Extensions["code"])
	})

	t.Run("writes multiple graphql errors", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		writeJsonGraphqlError(&buf,
			&gqlerror.Error{Message: "error one"},
			&gqlerror.Error{Message: "error two"},
		)

		var result struct {
			Errors []struct {
				Message string `json:"message"`
			} `json:"errors"`
		}
		err := json.Unmarshal(buf.Bytes(), &result)
		require.NoError(t, err)
		require.Len(t, result.Errors, 2)
		assert.Equal(t, "error one", result.Errors[0].Message)
		assert.Equal(t, "error two", result.Errors[1].Message)
	})
}

func TestJsonDecode(t *testing.T) {
	t.Parallel()

	t.Run("decodes valid JSON object", func(t *testing.T) {
		t.Parallel()
		reader := strings.NewReader(`{"name":"test","count":42}`)
		var result map[string]interface{}
		err := jsonDecode(reader, &result)
		require.NoError(t, err)
		assert.Equal(t, "test", result["name"])
	})

	t.Run("uses json.Number for numeric values to preserve precision", func(t *testing.T) {
		t.Parallel()
		reader := strings.NewReader(`{"value":123456789012345678}`)
		var result map[string]interface{}
		err := jsonDecode(reader, &result)
		require.NoError(t, err)
		num, ok := result["value"].(json.Number)
		require.True(t, ok, "expected json.Number, got %T", result["value"])
		assert.Equal(t, "123456789012345678", num.String())
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		t.Parallel()
		reader := strings.NewReader(`{invalid json}`)
		var result map[string]interface{}
		err := jsonDecode(reader, &result)
		assert.Error(t, err)
	})

	t.Run("decodes into GraphQL RawParams struct", func(t *testing.T) {
		t.Parallel()
		reader := strings.NewReader(`{"query":"{ users { id } }","operationName":"GetUsers"}`)
		var params graphql.RawParams
		err := jsonDecode(reader, &params)
		require.NoError(t, err)
		assert.Equal(t, "{ users { id } }", params.Query)
		assert.Equal(t, "GetUsers", params.OperationName)
	})
}

func TestStatusFor(t *testing.T) {
	t.Parallel()

	t.Run("returns 422 for protocol errors", func(t *testing.T) {
		t.Parallel()
		err := &gqlerror.Error{Message: "protocol error"}
		errcode.Set(err, errcode.ValidationFailed)
		assert.Equal(t, http.StatusUnprocessableEntity, statusFor(gqlerror.List{err}))
	})

	t.Run("returns 200 for non-protocol errors", func(t *testing.T) {
		t.Parallel()
		errs := gqlerror.List{{Message: "some error"}}
		assert.Equal(t, http.StatusOK, statusFor(errs))
	})

	t.Run("returns 200 for nil list", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, http.StatusOK, statusFor(nil))
	})

	t.Run("returns 200 for empty list", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, http.StatusOK, statusFor(gqlerror.List{}))
	})
}

func TestBytesReaderRead(t *testing.T) {
	t.Parallel()

	t.Run("reads full content in one call", func(t *testing.T) {
		t.Parallel()
		data := []byte("hello world")
		r := &bytesReader{s: &data, i: 0}

		buf := make([]byte, 20)
		n, err := r.Read(buf)
		assert.NoError(t, err)
		assert.Equal(t, 11, n)
		assert.Equal(t, "hello world", string(buf[:n]))
	})

	t.Run("reads in successive chunks", func(t *testing.T) {
		t.Parallel()
		data := []byte("hello world")
		r := &bytesReader{s: &data, i: 0}

		buf := make([]byte, 5)
		n, err := r.Read(buf)
		assert.NoError(t, err)
		assert.Equal(t, 5, n)
		assert.Equal(t, "hello", string(buf[:n]))

		n, err = r.Read(buf)
		assert.NoError(t, err)
		assert.Equal(t, 5, n)
		assert.Equal(t, " worl", string(buf[:n]))

		n, err = r.Read(buf)
		assert.NoError(t, err)
		assert.Equal(t, 1, n)
		assert.Equal(t, "d", string(buf[:n]))

		n, err = r.Read(buf)
		assert.Equal(t, 0, n)
		assert.Equal(t, io.EOF, err)
	})

	t.Run("returns EOF when reading index is at end", func(t *testing.T) {
		t.Parallel()
		data := []byte("hi")
		r := &bytesReader{s: &data, i: 2}

		buf := make([]byte, 5)
		n, err := r.Read(buf)
		assert.Equal(t, 0, n)
		assert.Equal(t, io.EOF, err)
	})

	t.Run("returns error when byte slice pointer is nil", func(t *testing.T) {
		t.Parallel()
		r := &bytesReader{s: nil, i: 0}

		buf := make([]byte, 5)
		n, err := r.Read(buf)
		assert.Equal(t, 0, n)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "byte slice pointer is nil")
	})

	t.Run("returns EOF for empty byte slice", func(t *testing.T) {
		t.Parallel()
		data := []byte{}
		r := &bytesReader{s: &data, i: 0}

		buf := make([]byte, 5)
		n, err := r.Read(buf)
		assert.Equal(t, 0, n)
		assert.Equal(t, io.EOF, err)
	})

	t.Run("works with io.ReadAll", func(t *testing.T) {
		t.Parallel()
		data := []byte("test content for ReadAll")
		r := &bytesReader{s: &data, i: 0}

		result, err := io.ReadAll(r)
		require.NoError(t, err)
		assert.Equal(t, "test content for ReadAll", string(result))
	})
}

func TestBytesReaderSeek(t *testing.T) {
	t.Parallel()

	t.Run("SeekStart sets position from beginning", func(t *testing.T) {
		t.Parallel()
		data := []byte("hello world")
		r := &bytesReader{s: &data, i: 5}

		pos, err := r.Seek(0, io.SeekStart)
		assert.NoError(t, err)
		assert.Equal(t, int64(0), pos)
		assert.Equal(t, int64(0), r.i)

		pos, err = r.Seek(6, io.SeekStart)
		assert.NoError(t, err)
		assert.Equal(t, int64(6), pos)

		buf := make([]byte, 5)
		n, err := r.Read(buf)
		assert.NoError(t, err)
		assert.Equal(t, "world", string(buf[:n]))
	})

	t.Run("SeekCurrent moves position relative to current", func(t *testing.T) {
		t.Parallel()
		data := []byte("hello world")
		r := &bytesReader{s: &data, i: 3}

		pos, err := r.Seek(2, io.SeekCurrent)
		assert.NoError(t, err)
		assert.Equal(t, int64(5), pos)

		pos, err = r.Seek(-3, io.SeekCurrent)
		assert.NoError(t, err)
		assert.Equal(t, int64(2), pos)

		buf := make([]byte, 3)
		n, err := r.Read(buf)
		assert.NoError(t, err)
		assert.Equal(t, "llo", string(buf[:n]))
	})

	t.Run("SeekEnd sets position relative to end", func(t *testing.T) {
		t.Parallel()
		data := []byte("hello world")
		r := &bytesReader{s: &data, i: 0}

		pos, err := r.Seek(0, io.SeekEnd)
		assert.NoError(t, err)
		assert.Equal(t, int64(11), pos)

		pos, err = r.Seek(-5, io.SeekEnd)
		assert.NoError(t, err)
		assert.Equal(t, int64(6), pos)

		buf := make([]byte, 10)
		n, err := r.Read(buf)
		assert.NoError(t, err)
		assert.Equal(t, "world", string(buf[:n]))
	})

	t.Run("returns error for negative position via SeekStart", func(t *testing.T) {
		t.Parallel()
		data := []byte("hello")
		r := &bytesReader{s: &data, i: 0}

		_, err := r.Seek(-1, io.SeekStart)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "negative position")
	})

	t.Run("returns error for negative position via SeekCurrent", func(t *testing.T) {
		t.Parallel()
		data := []byte("hello")
		r := &bytesReader{s: &data, i: 2}

		_, err := r.Seek(-5, io.SeekCurrent)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "negative position")
	})

	t.Run("returns error for negative position via SeekEnd", func(t *testing.T) {
		t.Parallel()
		data := []byte("hi")
		r := &bytesReader{s: &data, i: 0}

		_, err := r.Seek(-10, io.SeekEnd)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "negative position")
	})

	t.Run("returns error for invalid whence", func(t *testing.T) {
		t.Parallel()
		data := []byte("hello")
		r := &bytesReader{s: &data, i: 0}

		_, err := r.Seek(0, 99)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid whence")
	})

	t.Run("returns error when byte slice pointer is nil", func(t *testing.T) {
		t.Parallel()
		r := &bytesReader{s: nil, i: 0}

		_, err := r.Seek(0, io.SeekStart)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "byte slice pointer is nil")
	})

	t.Run("can seek past end of data without error", func(t *testing.T) {
		t.Parallel()
		data := []byte("hi")
		r := &bytesReader{s: &data, i: 0}

		pos, err := r.Seek(100, io.SeekStart)
		assert.NoError(t, err)
		assert.Equal(t, int64(100), pos)

		// Reading from past end returns EOF
		buf := make([]byte, 5)
		n, readErr := r.Read(buf)
		assert.Equal(t, 0, n)
		assert.Equal(t, io.EOF, readErr)
	})
}

func TestBytesReaderSeekThenRead(t *testing.T) {
	t.Parallel()

	t.Run("seek back to start and re-read full content", func(t *testing.T) {
		t.Parallel()
		data := []byte("abcdef")
		r := &bytesReader{s: &data, i: 0}

		// Read all
		all, err := io.ReadAll(r)
		require.NoError(t, err)
		assert.Equal(t, "abcdef", string(all))

		// Seek back to start
		pos, err := r.Seek(0, io.SeekStart)
		require.NoError(t, err)
		assert.Equal(t, int64(0), pos)

		// Read again
		all, err = io.ReadAll(r)
		require.NoError(t, err)
		assert.Equal(t, "abcdef", string(all))
	})

	t.Run("seek to middle and read remainder", func(t *testing.T) {
		t.Parallel()
		data := []byte("0123456789")
		r := &bytesReader{s: &data, i: 0}

		_, err := r.Seek(5, io.SeekStart)
		require.NoError(t, err)

		remainder, err := io.ReadAll(r)
		require.NoError(t, err)
		assert.Equal(t, "56789", string(remainder))
	})
}

func TestMultipartFormImplementsTransport(t *testing.T) {
	t.Parallel()
	var _ graphql.Transport = MultipartForm{}
}

func TestDoContentLengthTooLarge(t *testing.T) {
	t.Parallel()

	f := MultipartForm{MaxUploadSize: 100}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/graphql", strings.NewReader("x"))
	r.ContentLength = 200
	r.Header.Set("Content-Type", "multipart/form-data; boundary=test")

	f.Do(w, r, nil)

	var result struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &result)
	require.NoError(t, err)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Message, "256 MB limit")
}

func TestDoInvalidMultipartBody(t *testing.T) {
	t.Parallel()

	f := MultipartForm{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/graphql", strings.NewReader("not multipart at all"))
	r.Header.Set("Content-Type", "multipart/form-data; boundary=testboundary")

	f.Do(w, r, nil)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)

	var result struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &result)
	require.NoError(t, err)
	require.Len(t, result.Errors, 1)
}

func TestDoMissingOperationsPart(t *testing.T) {
	t.Parallel()

	var body bytes.Buffer
	body.WriteString("--testboundary\r\n")
	body.WriteString("Content-Disposition: form-data; name=\"wrong\"\r\n\r\n")
	body.WriteString(`{"query":"{ users { id } }"}`)
	body.WriteString("\r\n--testboundary--\r\n")

	f := MultipartForm{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/graphql", &body)
	r.Header.Set("Content-Type", "multipart/form-data; boundary=testboundary")

	f.Do(w, r, nil)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)

	var result struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &result)
	require.NoError(t, err)
	require.Len(t, result.Errors, 1)
	assert.Equal(t, "first part must be operations", result.Errors[0].Message)
}

func TestDoInvalidOperationsJSON(t *testing.T) {
	t.Parallel()

	var body bytes.Buffer
	body.WriteString("--testboundary\r\n")
	body.WriteString("Content-Disposition: form-data; name=\"operations\"\r\n\r\n")
	body.WriteString(`{invalid json}`)
	body.WriteString("\r\n--testboundary--\r\n")

	f := MultipartForm{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/graphql", &body)
	r.Header.Set("Content-Type", "multipart/form-data; boundary=testboundary")

	f.Do(w, r, nil)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)

	var result struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &result)
	require.NoError(t, err)
	require.Len(t, result.Errors, 1)
	assert.Equal(t, "operations form field could not be decoded", result.Errors[0].Message)
}

func TestDoMissingMapPart(t *testing.T) {
	t.Parallel()

	var body bytes.Buffer
	body.WriteString("--testboundary\r\n")
	body.WriteString("Content-Disposition: form-data; name=\"operations\"\r\n\r\n")
	body.WriteString(`{"query":"{ users { id } }"}`)
	body.WriteString("\r\n--testboundary\r\n")
	body.WriteString("Content-Disposition: form-data; name=\"wrong\"\r\n\r\n")
	body.WriteString(`{}`)
	body.WriteString("\r\n--testboundary--\r\n")

	f := MultipartForm{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/graphql", &body)
	r.Header.Set("Content-Type", "multipart/form-data; boundary=testboundary")

	f.Do(w, r, nil)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)

	var result struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &result)
	require.NoError(t, err)
	require.Len(t, result.Errors, 1)
	assert.Equal(t, "second part must be map", result.Errors[0].Message)
}

func TestDoMissingMapPartNoSecondPart(t *testing.T) {
	t.Parallel()

	// Only operations, no second part at all
	var body bytes.Buffer
	body.WriteString("--testboundary\r\n")
	body.WriteString("Content-Disposition: form-data; name=\"operations\"\r\n\r\n")
	body.WriteString(`{"query":"{ users { id } }"}`)
	body.WriteString("\r\n--testboundary--\r\n")

	f := MultipartForm{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/graphql", &body)
	r.Header.Set("Content-Type", "multipart/form-data; boundary=testboundary")

	f.Do(w, r, nil)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)

	var result struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &result)
	require.NoError(t, err)
	require.Len(t, result.Errors, 1)
	assert.Equal(t, "second part must be map", result.Errors[0].Message)
}

func TestDoInvalidMapJSON(t *testing.T) {
	t.Parallel()

	var body bytes.Buffer
	body.WriteString("--testboundary\r\n")
	body.WriteString("Content-Disposition: form-data; name=\"operations\"\r\n\r\n")
	body.WriteString(`{"query":"{ users { id } }"}`)
	body.WriteString("\r\n--testboundary\r\n")
	body.WriteString("Content-Disposition: form-data; name=\"map\"\r\n\r\n")
	body.WriteString(`{not valid json}`)
	body.WriteString("\r\n--testboundary--\r\n")

	f := MultipartForm{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/graphql", &body)
	r.Header.Set("Content-Type", "multipart/form-data; boundary=testboundary")

	f.Do(w, r, nil)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)

	var result struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &result)
	require.NoError(t, err)
	require.Len(t, result.Errors, 1)
	assert.Equal(t, "map form field could not be decoded", result.Errors[0].Message)
}

func TestDoEmptyPathsForKey(t *testing.T) {
	t.Parallel()

	var body bytes.Buffer
	body.WriteString("--testboundary\r\n")
	body.WriteString("Content-Disposition: form-data; name=\"operations\"\r\n\r\n")
	body.WriteString(`{"query":"mutation($file: Upload!) { upload(file: $file) }","variables":{"file":null}}`)
	body.WriteString("\r\n--testboundary\r\n")
	body.WriteString("Content-Disposition: form-data; name=\"map\"\r\n\r\n")
	body.WriteString(`{"0":[]}`)
	body.WriteString("\r\n--testboundary\r\n")
	body.WriteString("Content-Disposition: form-data; name=\"0\"; filename=\"test.txt\"\r\n")
	body.WriteString("Content-Type: text/plain\r\n\r\n")
	body.WriteString("file content here")
	body.WriteString("\r\n--testboundary--\r\n")

	f := MultipartForm{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/graphql", &body)
	r.Header.Set("Content-Type", "multipart/form-data; boundary=testboundary")

	f.Do(w, r, nil)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)

	var result struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &result)
	require.NoError(t, err)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Message, "invalid empty operations paths list for key 0")
}

func TestDoUnmappedKeysInMap(t *testing.T) {
	t.Parallel()

	var body bytes.Buffer
	body.WriteString("--testboundary\r\n")
	body.WriteString("Content-Disposition: form-data; name=\"operations\"\r\n\r\n")
	body.WriteString(`{"query":"mutation($file: Upload!) { upload(file: $file) }","variables":{"file":null}}`)
	body.WriteString("\r\n--testboundary\r\n")
	body.WriteString("Content-Disposition: form-data; name=\"map\"\r\n\r\n")
	body.WriteString(`{"0":["variables.file"]}`)
	body.WriteString("\r\n--testboundary--\r\n")

	f := MultipartForm{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/graphql", &body)
	r.Header.Set("Content-Type", "multipart/form-data; boundary=testboundary")

	f.Do(w, r, nil)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)

	var result struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &result)
	require.NoError(t, err)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Message, "failed to get key 0 from form")
}

func TestDoResponseHeaders(t *testing.T) {
	t.Parallel()

	f := MultipartForm{
		MaxUploadSize: 100,
		ResponseHeaders: map[string][]string{
			"X-Request-Id": {"abc123"},
		},
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/graphql", strings.NewReader("x"))
	r.ContentLength = 200
	r.Header.Set("Content-Type", "multipart/form-data; boundary=test")

	f.Do(w, r, nil)

	assert.Equal(t, "abc123", w.Header().Get("X-Request-Id"))
}

func TestDoDefaultResponseHeaders(t *testing.T) {
	t.Parallel()

	f := MultipartForm{MaxUploadSize: 100}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/graphql", strings.NewReader("x"))
	r.ContentLength = 200
	r.Header.Set("Content-Type", "multipart/form-data; boundary=test")

	f.Do(w, r, nil)

	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
}

func TestDoFilePartKeyNotInMap(t *testing.T) {
	t.Parallel()

	// File part has a key that is not in the map at all
	var body bytes.Buffer
	body.WriteString("--testboundary\r\n")
	body.WriteString("Content-Disposition: form-data; name=\"operations\"\r\n\r\n")
	body.WriteString(`{"query":"mutation($file: Upload!) { upload(file: $file) }","variables":{"file":null}}`)
	body.WriteString("\r\n--testboundary\r\n")
	body.WriteString("Content-Disposition: form-data; name=\"map\"\r\n\r\n")
	body.WriteString(`{"1":["variables.file"]}`)
	body.WriteString("\r\n--testboundary\r\n")
	body.WriteString("Content-Disposition: form-data; name=\"0\"; filename=\"test.txt\"\r\n")
	body.WriteString("Content-Type: text/plain\r\n\r\n")
	body.WriteString("file content")
	body.WriteString("\r\n--testboundary--\r\n")

	f := MultipartForm{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/graphql", &body)
	r.Header.Set("Content-Type", "multipart/form-data; boundary=testboundary")

	f.Do(w, r, nil)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)

	var result struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &result)
	require.NoError(t, err)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Message, "invalid empty operations paths list for key 0")
}
