package log

import (
	"context"
	"errors"
	"slices"
	"strconv"

	"github.com/hovanhoa/llmgateway/pkg/core/apm"
	"github.com/iancoleman/strcase"
	"go.uber.org/zap/zapcore"
)

// APMCore implements a zap core that logs errors to Slack.
type APMCore struct {
	logCtx context.Context
	fields []zapcore.Field
}

func NewAPMCore(ctx context.Context) *APMCore {
	return &APMCore{logCtx: ctx}
}

// Enabled returns whether this core is enabled or not. APMCore
// is only enabled for error logs and above.
func (c *APMCore) Enabled(lvl zapcore.Level) bool {
	return lvl >= zapcore.ErrorLevel
}

// WrapCore returns zapcore.NewTee(core, c).
// WrapCore is suitable for passing to zap.WrapCore.
func (c *APMCore) WrapCore(core zapcore.Core) zapcore.Core {
	return zapcore.NewTee(core, c)
}

// With adds structured context to the Core.
func (c *APMCore) With(fields []zapcore.Field) zapcore.Core {
	return &APMCore{
		logCtx: c.logCtx,
		fields: append(c.fields, fields...),
	}
}

// Check determines whether the supplied Entry should be logged (using the
// embedded LevelEnabler and possibly some extra logic). If the entry
// should be logged, the Core adds itself to the CheckedEntry and returns
// the result.
//
// Callers must use Check before calling Write.
func (c *APMCore) Check(entry zapcore.Entry, checked *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if entry.Level < zapcore.ErrorLevel {
		return checked
	}

	return checked.AddCore(entry, c)
}

// Write serializes the log Entry and writes a message summarizing the contained
// error to a Slack thread in #notifications-alerts
func (c *APMCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	// use c.Hub to write an error
	hub := apm.GetHubFromContext(c.logCtx)
	if hub == nil {
		return nil
	}

	hub.ConfigureScope(func(scope *apm.Scope) {
		scope.SetTags(c.getTags(fields))
		scope.SetExtra("message", entry.Message)
	})

	err := c.getErrorField(fields)
	if err == nil {
		err = errors.New(entry.Message)
	}

	client, scope := hub.Client(), hub.Scope()
	if client == nil {
		return nil
	}

	client.CaptureException(
		err,
		&apm.EventHint{
			Context:           c.logCtx,
			OriginalException: err,
		},
		scope,
	)

	return nil
}

func shouldExclude(s string) bool {
	return slices.Contains(
		[]string{
			"environment",
			"service_name",
			"service_version",
		},
		s,
	)
}

func (c *APMCore) getTags(fields []zapcore.Field) map[string]string {
	m := make(map[string]string)
	for _, f := range append(c.fields, fields...) {
		// Excluse certain tags from Sentry
		if shouldExclude(f.Key) {
			continue
		}

		if f.String != "" {
			m["app."+strcase.ToSnake(f.Key)] = f.String
		}

		switch v := f.Interface.(type) {
		case int:
			m[f.Key] = strconv.FormatInt(int64(v), 10)
		case int8:
			m[f.Key] = strconv.FormatInt(int64(v), 10)
		case int16:
			m[f.Key] = strconv.FormatInt(int64(v), 10)
		case int32:
			m[f.Key] = strconv.FormatInt(int64(v), 10)
		case int64:
			m[f.Key] = strconv.FormatInt(int64(v), 10)
		case uint:
			m[f.Key] = strconv.FormatUint(uint64(v), 10)
		case uint8:
			m[f.Key] = strconv.FormatUint(uint64(v), 10)
		case uint16:
			m[f.Key] = strconv.FormatUint(uint64(v), 10)
		case uint32:
			m[f.Key] = strconv.FormatUint(uint64(v), 10)
		case uint64:
			m[f.Key] = strconv.FormatUint(uint64(v), 10)
		case float32:
			m[f.Key] = strconv.FormatFloat(float64(v), 'f', -1, 64)
		case float64:
			m[f.Key] = strconv.FormatFloat(float64(v), 'f', -1, 64)
		}
	}

	return m
}

// getErrorField returns any error that may have been logged.
func (c *APMCore) getErrorField(fields []zapcore.Field) error {
	fields = append(fields, c.fields...)
	for _, f := range fields {
		if f.Type == zapcore.ErrorType {
			if v, ok := f.Interface.(error); ok && v != nil {
				return v
			}
		}
	}

	return nil
}

// Sync flushes buffered logs (if any).
func (c *APMCore) Sync() error {
	return nil
}
