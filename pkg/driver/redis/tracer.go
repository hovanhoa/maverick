package redis

import (
	"bytes"
	"context"
	"strconv"
	"strings"

	"github.com/go-redis/redis/v8"
	"github.com/hovanhoa/llmgateway/pkg/core/apm"
)

// tracingHook is an implementation of redis.Hook that reports cmds as spans to Elastic APM.
type tracingHook struct {
	config Config
}

// NewHook returns a redis.Hook that reports cmds as spans to Elastic APM.
func NewTracingHook(config Config) redis.Hook {
	return &tracingHook{config}
}

// BeforeProcess initiates the span for the redis cmd
func (r *tracingHook) BeforeProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error) {
	span := apm.StartSpan(
		ctx,
		"db.query",
		apm.WithDescription(getCmdName(cmd)),
	)
	if span == nil {
		return ctx, nil
	}

	span.SetData("db.system", "redis")
	span.SetData("db.operation", getCmdOperation(cmd))
	span.SetData("db.redis.database_index", strconv.Itoa(int(r.config.DB)))
	span.SetData("server.port", r.config.Port)
	span.SetData("server.address", r.config.Host)

	return span.Context(), nil
}

// AfterProcess ends the initiated span from BeforeProcess
func (r *tracingHook) AfterProcess(ctx context.Context, cmd redis.Cmder) error {
	if span := apm.SpanFromContext(ctx); span != nil {
		span.Finish()
	}
	return nil
}

// BeforeProcessPipeline initiates the span for the redis cmds
func (r *tracingHook) BeforeProcessPipeline(ctx context.Context, cmds []redis.Cmder) (context.Context, error) {
	// Join all cmd names with ", ".
	var cmdNameBuf bytes.Buffer
	for i, cmd := range cmds {
		if i != 0 {
			cmdNameBuf.WriteString(", ")
		}
		cmdNameBuf.WriteString(getCmdName(cmd))
	}

	span := apm.StartSpan(
		ctx,
		"redis.query",
		apm.WithDescription(cmdNameBuf.String()),
	)
	if span == nil {
		return ctx, nil
	}
	return span.Context(), nil
}

// AfterProcess ends the initiated span from BeforeProcessPipeline
func (r *tracingHook) AfterProcessPipeline(ctx context.Context, cmds []redis.Cmder) error {
	if span := apm.SpanFromContext(ctx); span != nil {
		span.Finish()
	}
	return nil
}

func getCmdOperation(cmd redis.Cmder) string {
	return strings.ToUpper(cmd.Name())
}

func getCmdName(cmd redis.Cmder) string {
	cmdName := strings.ToUpper(cmd.Name())
	if cmdName == "" {
		return "(empty command)"
	}
	if cmdName == "GET" {
		if len(cmd.Args()) > 1 {
			if arg, ok := cmd.Args()[1].(string); ok {
				cmdName += " " + arg
			}
		}
	}
	return cmdName
}
