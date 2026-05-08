package log

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/hovanhoa/llmgateway/pkg/core/apm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
)

func TestAPMCore_Enabled(t *testing.T) {
	t.Parallel()

	c := NewAPMCore(context.Background())
	assert.False(t, c.Enabled(zapcore.DebugLevel))
	assert.False(t, c.Enabled(zapcore.InfoLevel))
	assert.False(t, c.Enabled(zapcore.WarnLevel))
	assert.True(t, c.Enabled(zapcore.ErrorLevel))
	assert.True(t, c.Enabled(zapcore.PanicLevel))
	assert.True(t, c.Enabled(zapcore.FatalLevel))
}

func TestAPMCore_With_AppendsFields(t *testing.T) {
	t.Parallel()

	parent := NewAPMCore(context.Background())
	child := parent.With([]zapcore.Field{
		String("k1", "v1"),
		String("k2", "v2"),
	})

	apmChild, ok := child.(*APMCore)
	require.True(t, ok)
	require.Len(t, apmChild.fields, 2)
	assert.Equal(t, "k1", apmChild.fields[0].Key)
	assert.Equal(t, "k2", apmChild.fields[1].Key)

	// Calling With again should preserve earlier fields and append.
	grand := child.With([]zapcore.Field{Int("count", 5)})
	apmGrand, ok := grand.(*APMCore)
	require.True(t, ok)
	require.Len(t, apmGrand.fields, 3)
	assert.Equal(t, "count", apmGrand.fields[2].Key)
}

func TestAPMCore_Check(t *testing.T) {
	t.Parallel()

	c := NewAPMCore(context.Background())

	t.Run("low level not added", func(t *testing.T) {
		t.Parallel()

		entry := zapcore.Entry{Level: zapcore.InfoLevel, Message: "hello"}
		ce := &zapcore.CheckedEntry{}
		got := c.Check(entry, ce)
		assert.Same(t, ce, got)
	})

	t.Run("error level adds core", func(t *testing.T) {
		t.Parallel()

		entry := zapcore.Entry{Level: zapcore.ErrorLevel, Message: "boom"}
		ce := &zapcore.CheckedEntry{}
		got := c.Check(entry, ce)
		assert.NotNil(t, got)
	})
}

func TestAPMCore_WrapCore_Tees(t *testing.T) {
	t.Parallel()

	c := NewAPMCore(context.Background())
	inner := zapcore.NewNopCore()
	wrapped := c.WrapCore(inner)
	require.NotNil(t, wrapped)

	// The tee should also reject sub-error levels via the inner Nop core
	// while still considering APMCore's threshold for error+ entries.
	assert.False(t, wrapped.Enabled(zapcore.DebugLevel))
}

func TestAPMCore_Sync(t *testing.T) {
	t.Parallel()

	c := NewAPMCore(context.Background())
	assert.NoError(t, c.Sync())
}

func TestAPMCore_GetTags_StringsAndExclusions(t *testing.T) {
	t.Parallel()

	c := NewAPMCore(context.Background())
	c.fields = []zapcore.Field{String("preexisting", "yes")}

	tags := c.getTags([]zapcore.Field{
		String("environment", "production"), // excluded by key
		String("service_name", "api"),       // excluded by key
		String("service_version", "abc123"), // excluded by key
		String("Name", "alice"),             // recorded as app.name
		String("UserID", "u-123"),           // recorded as app.user_id (snake_case)
		String("empty", ""),                 // skipped because f.String == ""
	})

	for _, exclude := range []string{
		"environment", "service_name", "service_version",
		"app.environment", "app.service_name", "app.service_version",
	} {
		_, found := tags[exclude]
		assert.False(t, found, "expected %q to be excluded from tags", exclude)
	}

	assert.Equal(t, "alice", tags["app.name"])
	assert.Equal(t, "u-123", tags["app.user_id"])
	assert.Equal(t, "yes", tags["app.preexisting"])

	_, hasEmpty := tags["app.empty"]
	assert.False(t, hasEmpty, "fields with empty string value should not be recorded")
}

func TestAPMCore_GetTags_NumericInterfaceFields(t *testing.T) {
	t.Parallel()

	// zap's standard typed constructors (Int, Float64, etc.) put the value
	// into f.Integer rather than f.Interface, so the numeric switch in
	// getTags only fires when the value is actually present on f.Interface.
	// Build the fields directly to exercise every branch.
	mk := func(key string, v any) zapcore.Field {
		return zapcore.Field{Key: key, Type: zapcore.ReflectType, Interface: v}
	}

	c := NewAPMCore(context.Background())
	tags := c.getTags([]zapcore.Field{
		mk("count", int(42)),
		mk("i8", int8(1)),
		mk("i16", int16(2)),
		mk("i32", int32(3)),
		mk("i64", int64(4)),
		mk("u", uint(5)),
		mk("u8", uint8(6)),
		mk("u16", uint16(7)),
		mk("u32", uint32(8)),
		mk("u64", uint64(9)),
		mk("f32", float32(1.5)),
		mk("f64", float64(2.5)),
		// A type the switch does not handle should be silently dropped.
		mk("ignored", []string{"a", "b"}),
	})

	assert.Equal(t, "42", tags["count"])
	assert.Equal(t, "1", tags["i8"])
	assert.Equal(t, "2", tags["i16"])
	assert.Equal(t, "3", tags["i32"])
	assert.Equal(t, "4", tags["i64"])
	assert.Equal(t, "5", tags["u"])
	assert.Equal(t, "6", tags["u8"])
	assert.Equal(t, "7", tags["u16"])
	assert.Equal(t, "8", tags["u32"])
	assert.Equal(t, "9", tags["u64"])
	assert.Equal(t, "1.5", tags["f32"])
	assert.Equal(t, "2.5", tags["f64"])

	_, ignored := tags["ignored"]
	assert.False(t, ignored, "non-primitive Interface values should not be tagged")
}

func TestAPMCore_GetErrorField(t *testing.T) {
	t.Parallel()

	c := NewAPMCore(context.Background())
	want := errors.New("expected")

	t.Run("finds error in passed fields", func(t *testing.T) {
		t.Parallel()

		got := c.getErrorField([]zapcore.Field{
			String("k", "v"),
			Error(want),
		})
		assert.Equal(t, want, got)
	})

	t.Run("finds error in core's fields", func(t *testing.T) {
		t.Parallel()

		other := errors.New("from-core")
		core := &APMCore{logCtx: context.Background(), fields: []zapcore.Field{Error(other)}}
		got := core.getErrorField([]zapcore.Field{String("k", "v")})
		assert.Equal(t, other, got)
	})

	t.Run("returns nil when no error fields", func(t *testing.T) {
		t.Parallel()

		got := c.getErrorField([]zapcore.Field{String("k", "v"), Int("n", 1)})
		assert.Nil(t, got)
	})

	t.Run("returns nil when error value is nil", func(t *testing.T) {
		t.Parallel()

		got := c.getErrorField([]zapcore.Field{Error(nil)})
		assert.Nil(t, got)
	})
}

// recordingTransport implements sentry.Transport and captures sent events
// in memory, so tests can assert what would be reported without making any
// network calls.
type recordingTransport struct {
	mu     sync.Mutex
	events []*sentry.Event
}

func (t *recordingTransport) Configure(_ sentry.ClientOptions) {}

func (t *recordingTransport) SendEvent(event *sentry.Event) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.events = append(t.events, event)
}

func (t *recordingTransport) Flush(_ time.Duration) bool { return true }

func (t *recordingTransport) FlushWithContext(_ context.Context) bool { return true }

func (t *recordingTransport) Close() {}

func (t *recordingTransport) snapshot() []*sentry.Event {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]*sentry.Event, len(t.events))
	copy(out, t.events)
	return out
}

func TestAPMCore_Write_NoHubReturnsNil(t *testing.T) {
	t.Parallel()

	// Use context.TODO so sentry's GetHubFromContext returns no embedded hub.
	// apm.GetHubFromContext then falls back to sentry.CurrentHub().Clone(),
	// which does have a hub but no configured client (in tests). The Write
	// method should still return nil without panicking.
	c := NewAPMCore(context.TODO())
	entry := zapcore.Entry{Level: zapcore.ErrorLevel, Message: "boom"}
	err := c.Write(entry, []zapcore.Field{String("k", "v")})
	assert.NoError(t, err)
}

func TestAPMCore_Write_WithRecordingClientCapturesError(t *testing.T) {
	t.Parallel()

	// Build a Sentry client that uses our recording transport, attach it to
	// a hub, and stick it on a context. Then Write should call CaptureException
	// against that hub and the transport should record an event.
	rec := &recordingTransport{}
	client, err := sentry.NewClient(sentry.ClientOptions{
		Dsn:       "https://public@example.com/1",
		Transport: rec,
	})
	require.NoError(t, err)

	hub := sentry.NewHub(client, sentry.NewScope())
	ctx := apm.SetHubOnContext(context.Background(), hub)

	c := NewAPMCore(ctx)
	c = c.With([]zapcore.Field{
		String("Account", "acct-1"),
		// Direct construction so the numeric switch in getTags actually fires.
		{Key: "retry", Type: zapcore.ReflectType, Interface: int(2)},
	}).(*APMCore)

	wantErr := errors.New("the underlying problem")
	logErr := c.Write(
		zapcore.Entry{Level: zapcore.ErrorLevel, Message: "something failed"},
		[]zapcore.Field{Error(wantErr), String("RequestID", "r-1")},
	)
	require.NoError(t, logErr)
	client.Flush(0)

	events := rec.snapshot()
	require.Len(t, events, 1)
	got := events[0]

	// Tags should include the snake-cased string fields (with app. prefix) and
	// the int field (no prefix).
	assert.Equal(t, "acct-1", got.Tags["app.account"])
	assert.Equal(t, "r-1", got.Tags["app.request_id"])
	assert.Equal(t, "2", got.Tags["retry"])

	// The "message" extra is set from the entry's Message.
	assert.Equal(t, "something failed", got.Extra["message"])

	// The captured exception should reflect the error field, not the entry's
	// Message text.
	require.NotEmpty(t, got.Exception)
	assert.Equal(t, wantErr.Error(), got.Exception[0].Value)
}

func TestAPMCore_Write_FallsBackToEntryMessageWhenNoErrorField(t *testing.T) {
	t.Parallel()

	rec := &recordingTransport{}
	client, err := sentry.NewClient(sentry.ClientOptions{
		Dsn:       "https://public@example.com/1",
		Transport: rec,
	})
	require.NoError(t, err)

	hub := sentry.NewHub(client, sentry.NewScope())
	ctx := apm.SetHubOnContext(context.Background(), hub)

	c := NewAPMCore(ctx)

	logErr := c.Write(
		zapcore.Entry{Level: zapcore.ErrorLevel, Message: "no error attached"},
		[]zapcore.Field{String("k", "v")},
	)
	require.NoError(t, logErr)
	client.Flush(0)

	events := rec.snapshot()
	require.Len(t, events, 1)

	require.NotEmpty(t, events[0].Exception)
	assert.Equal(t, "no error attached", events[0].Exception[0].Value)
}
