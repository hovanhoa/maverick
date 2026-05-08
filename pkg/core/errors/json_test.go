package errors

import (
	"encoding/json"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFrameMarshalText(t *testing.T) {
	t.Parallel()
	var tests = []struct {
		Frame
		want string
	}{{
		initpc,
		`^github.com/hovanhoa/llmgateway/pkg/core/errors\.init(\.ializers)? .+/gocode(-[^/]+)?/pkg/core/errors/stack_test.go:\d+$`,
	}, {
		0,
		`^unknown$`,
	}}
	for i, tt := range tests {
		got, err := tt.MarshalText()
		require.NoError(t, err)
		assert.True(t, regexp.MustCompile(tt.want).Match(got), "test %d: MarshalJSON:\n got %q\n want %q", i+1, string(got), tt.want)
	}
}

func TestFrameMarshalJSON(t *testing.T) {
	t.Parallel()
	var tests = []struct {
		Frame
		want string
	}{{
		initpc,
		`^"github\.com/hovanhoa/llmgateway/pkg/core/errors\.init(\.ializers)? .+/gocode(-[^/]+)?/pkg/core/errors/stack_test.go:\d+"$`,
	}, {
		0,
		`^"unknown"$`,
	}}
	for i, tt := range tests {
		got, err := json.Marshal(tt.Frame)
		require.NoError(t, err)
		assert.True(t, regexp.MustCompile(tt.want).Match(got), "test %d: MarshalJSON:\n got %q\n want %q", i+1, string(got), tt.want)
	}
}
