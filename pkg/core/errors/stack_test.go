package errors

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var initpc = caller()

type X struct{}

// val returns a Frame pointing to itself.
func (x X) val() Frame {
	return caller()
}

// ptr returns a Frame pointing to itself.
func (x *X) ptr() Frame {
	return caller()
}

func TestFrameFormat(t *testing.T) {
	t.Parallel()
	var tests = []struct {
		Frame
		format string
		want   string
	}{{
		initpc,
		"%s",
		"stack_test.go",
	}, {
		initpc,
		"%+s",
		"github.com/hovanhoa/llmgateway/pkg/core/errors.init\n" +
			"\t.+/gocode(-[^/]+)?/pkg/core/errors/stack_test.go",
	}, {
		0,
		"%s",
		"unknown",
	}, {
		0,
		"%+s",
		"unknown",
	}, {
		initpc,
		"%d",
		"12",
	}, {
		0,
		"%d",
		"0",
	}, {
		initpc,
		"%n",
		"init",
	}, {
		func() Frame {
			var x X
			return x.ptr()
		}(),
		"%n",
		`\(\*X\).ptr`,
	}, {
		func() Frame {
			var x X
			return x.val()
		}(),
		"%n",
		"X.val",
	}, {
		0,
		"%n",
		"",
	}, {
		initpc,
		"%v",
		"stack_test.go:12",
	}, {
		initpc,
		"%+v",
		"github.com/hovanhoa/llmgateway/pkg/core/errors.init\n" +
			"\t.+/gocode(-[^/]+)?/pkg/core/errors/stack_test.go:12",
	}, {
		0,
		"%v",
		"unknown:0",
	}}

	for i, tt := range tests {
		testFormatRegexp(t, i, tt.Frame, tt.format, tt.want)
	}
}

func TestFuncname(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, want string
	}{
		{"", ""},
		{"runtime.main", "main"},
		{"github.com/hovanhoa/llmgateway/pkg/core/errors.funcname", "funcname"},
		{"funcname", "funcname"},
		{"io.copyBuffer", "copyBuffer"},
		{"main.(*R).Write", "(*R).Write"},
	}

	for _, tt := range tests {
		got := funcname(tt.name)
		want := tt.want
		assert.Equal(t, want, got, "funcname(%q): want: %q, got %q", tt.name, want, got)
	}
}

func TestStackTrace(t *testing.T) {
	t.Parallel()
	tests := []struct {
		err  error
		want []string
	}{{
		New("ooh"), []string{
			"github.com/hovanhoa/llmgateway/pkg/core/errors.TestStackTrace\n" +
				"\t.+/gocode(-[^/]+)?/pkg/core/errors/stack_test.go:125",
		},
	}, {
		Wrap(New("ooh"), "ahh"), []string{
			"github.com/hovanhoa/llmgateway/pkg/core/errors.TestStackTrace\n" +
				"\t.+/gocode(-[^/]+)?/pkg/core/errors/stack_test.go:130", // this is the stack of Wrap, not New
		},
	}, {
		Cause(Wrap(New("ooh"), "ahh")), []string{
			"github.com/hovanhoa/llmgateway/pkg/core/errors.TestStackTrace\n" +
				"\t.+/gocode(-[^/]+)?/pkg/core/errors/stack_test.go:135", // this is the stack of New
		},
	}, {
		func() error { return New("ooh") }(), []string{
			`github.com/hovanhoa/llmgateway/pkg/core/errors.TestStackTrace.func1` +
				"\n\t.+/gocode(-[^/]+)?/pkg/core/errors/stack_test.go:140", // this is the stack of New
			"github.com/hovanhoa/llmgateway/pkg/core/errors.TestStackTrace\n" +
				"\t.+/gocode(-[^/]+)?/pkg/core/errors/stack_test.go:140", // this is the stack of New's caller
		},
	}, {
		Cause(func() error {
			return func() error {
				return New("hello %s", fmt.Sprintf("world: %s", "ooh"))
			}()
		}()), []string{
			`github.com/hovanhoa/llmgateway/pkg/core/errors.TestStackTrace.TestStackTrace.func2.func3` +
				"\n\t.+/gocode(-[^/]+)?/pkg/core/errors/stack_test.go:149", // this is the stack of Errorf
			`github.com/hovanhoa/llmgateway/pkg/core/errors.TestStackTrace.func2` +
				"\n\t.+/gocode(-[^/]+)?/pkg/core/errors/stack_test.go:150", // this is the stack of Errorf's caller
			"github.com/hovanhoa/llmgateway/pkg/core/errors.TestStackTrace\n" +
				"\t.+/gocode(-[^/]+)?/pkg/core/errors/stack_test.go:151", // this is the stack of Errorf's caller's caller
		},
	}}
	for i, tt := range tests {
		x, ok := tt.err.(interface {
			StackTrace() StackTrace
		})
		require.True(t, ok, "expected %#v to implement StackTrace() StackTrace", tt.err)
		st := x.StackTrace()
		for j, want := range tt.want {
			testFormatRegexp(t, i, st[j], "%+v", want)
		}
	}
}

func stackTrace() StackTrace {
	const depth = 8
	var pcs [depth]uintptr
	n := runtime.Callers(1, pcs[:])
	var st Stack = pcs[0:n]
	return st.StackTrace()
}

func TestStackTraceFormat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		StackTrace
		format string
		want   string
	}{{
		nil,
		"%s",
		`\[\]`,
	}, {
		nil,
		"%v",
		`\[\]`,
	}, {
		nil,
		"%+v",
		"",
	}, {
		nil,
		"%#v",
		`\[\]errors.Frame\(nil\)`,
	}, {
		make(StackTrace, 0),
		"%s",
		`\[\]`,
	}, {
		make(StackTrace, 0),
		"%v",
		`\[\]`,
	}, {
		make(StackTrace, 0),
		"%+v",
		"",
	}, {
		make(StackTrace, 0),
		"%#v",
		`\[\]errors.Frame{}`,
	}, {
		stackTrace()[:2],
		"%s",
		`\[stack_test.go stack_test.go\]`,
	}, {
		stackTrace()[:2],
		"%v",
		`\[stack_test.go:175 stack_test.go:223\]`,
	}, {
		stackTrace()[:2],
		"%+v",
		"\n" +
			"github.com/hovanhoa/llmgateway/pkg/core/errors.stackTrace\n" +
			"\t.+/gocode(-[^/]+)?/pkg/core/errors/stack_test.go:175\n" +
			"github.com/hovanhoa/llmgateway/pkg/core/errors.TestStackTraceFormat\n" +
			"\t.+/gocode(-[^/]+)?/pkg/core/errors/stack_test.go:227",
	}, {
		stackTrace()[:2],
		"%#v",
		`\[\]errors.Frame{stack_test.go:175, stack_test.go:235}`,
	}}

	for i, tt := range tests {
		testFormatRegexp(t, i, tt.StackTrace, tt.format, tt.want)
	}
}

// a version of runtime.Caller that returns a Frame, not a uintptr.
func caller() Frame {
	var pcs [3]uintptr
	n := runtime.Callers(2, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])
	frame, _ := frames.Next()
	return Frame(frame.PC)
}
