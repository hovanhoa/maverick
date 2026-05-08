package errors

import (
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	t.Parallel()

	tests := []struct {
		err  error
		want string
	}{
		{New(""), ""},
		{New("foo"), "foo"},
		{New("read error with %d format specifier", 1), "read error with 1 format specifier"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.want, tt.err.Error())
	}
}

func TestWrapNil(t *testing.T) {
	t.Parallel()
	got := Wrap(nil, "no error")
	assert.Nil(t, got)
}

func TestWrap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		err     error
		message string
		want    string
	}{
		{io.EOF, "read error", "read error: EOF"},
		{Wrap(io.EOF, "read error"), "client error", "client error: read error: EOF"},
	}

	for _, tt := range tests {
		got := Wrap(tt.err, "%s", tt.message).Error()
		assert.Equal(t, tt.want, got)
	}
}

type nilError struct{}

func (nilError) Error() string { return "nil error" }

func TestCause(t *testing.T) {
	t.Parallel()

	x := New("error")
	tests := []struct {
		err  error
		want error
	}{{
		// nil error is nil
		err:  nil,
		want: nil,
	}, {
		// explicit nil error is nil
		err:  (error)(nil),
		want: nil,
	}, {
		// typed nil is nil
		err:  (*nilError)(nil),
		want: (*nilError)(nil),
	}, {
		// uncaused error is unaffected
		err:  io.EOF,
		want: io.EOF,
	}, {
		// caused error returns cause
		err:  Wrap(io.EOF, "ignored"),
		want: io.EOF,
	}, {
		err:  x, // return from errors.New
		want: x,
	}, {
		WithMessage(nil, "whoops"),
		nil,
	}, {
		WithMessage(io.EOF, "whoops"),
		io.EOF,
	}, {
		WithStack(nil),
		nil,
	}, {
		WithStack(io.EOF),
		io.EOF,
	}}

	for i, tt := range tests {
		got := Cause(tt.err)
		assert.Equal(t, tt.want, got, "test %d", i+1)
	}
}

func TestWrapfNil(t *testing.T) {
	t.Parallel()
	got := Wrapf(nil, "no error")
	assert.Nil(t, got)
}

func TestWrapf(t *testing.T) {
	t.Parallel()

	tests := []struct {
		err     error
		message string
		want    string
	}{
		{io.EOF, "read error", "read error: EOF"},
		{Wrapf(io.EOF, "read error without format specifiers"), "client error", "client error: read error without format specifiers: EOF"},
		{Wrapf(io.EOF, "read error with %d format specifier", 1), "client error", "client error: read error with 1 format specifier: EOF"},
	}

	for _, tt := range tests {
		got := Wrapf(tt.err, "%s", tt.message).Error()
		assert.Equal(t, tt.want, got)
	}
}

func TestWithStackNil(t *testing.T) {
	t.Parallel()
	got := WithStack(nil)
	assert.Nil(t, got)
}

func TestWithStack(t *testing.T) {
	t.Parallel()

	tests := []struct {
		err  error
		want string
	}{
		{io.EOF, "EOF"},
		{WithStack(io.EOF), "EOF"},
	}

	for _, tt := range tests {
		got := WithStack(tt.err).Error()
		assert.Equal(t, tt.want, got)
	}
}

func TestWithMessageNil(t *testing.T) {
	t.Parallel()
	got := WithMessage(nil, "no error")
	assert.Nil(t, got)
}

func TestWithMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		err     error
		message string
		want    string
	}{
		{io.EOF, "read error", "read error: EOF"},
		{WithMessage(io.EOF, "read error"), "client error", "client error: read error: EOF"},
	}

	for _, tt := range tests {
		got := WithMessage(tt.err, tt.message).Error()
		assert.Equal(t, tt.want, got)
	}
}

func TestWithMessagefNil(t *testing.T) {
	t.Parallel()
	got := WithMessagef(nil, "no error")
	assert.Nil(t, got)
}

func TestWithMessagef(t *testing.T) {
	t.Parallel()

	tests := []struct {
		err     error
		message string
		want    string
	}{
		{io.EOF, "read error", "read error: EOF"},
		{WithMessagef(io.EOF, "read error without format specifier"), "client error", "client error: read error without format specifier: EOF"},
		{WithMessagef(io.EOF, "read error with %d format specifier", 1), "client error", "client error: read error with 1 format specifier: EOF"},
	}

	for _, tt := range tests {
		got := WithMessagef(tt.err, "%s", tt.message).Error()
		assert.Equal(t, tt.want, got)
	}
}

// errors.New, etc values are not expected to be compared by value
// but the change in errors#27 made them incomparable. Assert that
// various kinds of errors have a functional equality operator, even
// if the result of that equality is always false.
func TestErrorEquality(t *testing.T) {
	t.Parallel()

	vals := []error{
		nil,
		io.EOF,
		errors.New("EOF"),
		New("EOF"),
		New("EOF%d", 1),
		Wrap(io.EOF, "EOF"),
		Wrapf(io.EOF, "EOF%d", 2),
		WithMessage(nil, "whoops"),
		WithMessage(io.EOF, "whoops"),
		WithStack(io.EOF),
		WithStack(nil),
	}

	for i := range vals {
		for j := range vals {
			_ = vals[i] == vals[j] // mustn't panic
		}
	}
}
