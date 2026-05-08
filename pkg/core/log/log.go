package log

import (
	"context"
	"os"

	"github.com/hovanhoa/llmgateway/pkg/core/apm"
	"github.com/hovanhoa/llmgateway/pkg/core/env"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const sentryTraceKey string = "sentryTraceId"

type loggerKeyType string

const loggerKey loggerKeyType = "logger"

var overrideLogger *Logger

// New returns a logger instance configured for the environment
// the service is running in.
func New(opts ...zap.Option) *Logger {
	return FromContext(context.Background(), opts...)
}

func ContextWithLogger(ctx context.Context, logger *Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

func FromContext(ctx context.Context, opts ...zap.Option) *Logger {
	if overrideLogger != nil {
		return overrideLogger
	}

	if logger, ok := ctx.Value(loggerKey).(*Logger); ok {
		return logger
	}

	var logger *Logger
	var err error

	switch env.GetEnvironment() {
	case env.Production, env.Staging:
		encoder := zap.NewProductionEncoderConfig()
		encoder.TimeKey = "@timestamp"
		encoder.LevelKey = "log.level"
		encoder.MessageKey = "message"
		encoder.FunctionKey = "function.name"

		cfg := zap.NewProductionConfig()
		cfg.EncoderConfig = encoder

		allOptions := append([]zap.Option{
			zap.WrapCore(func(c zapcore.Core) zapcore.Core {
				return zapcore.NewTee(
					// send error traces to our APM
					NewAPMCore(ctx),
					// log errors to stderr as well
					zapcore.NewCore(
						zapcore.NewJSONEncoder(encoder),
						zapcore.Lock(os.Stderr),
						zap.LevelEnablerFunc(func(l zapcore.Level) bool { return l >= zapcore.ErrorLevel }),
					),
					// send all logs to stdout
					zapcore.NewCore(
						zapcore.NewJSONEncoder(encoder),
						zapcore.Lock(os.Stdout),
						zap.LevelEnablerFunc(func(l zapcore.Level) bool { return l >= zapcore.DebugLevel }),
					),
				)
			}),
		}, opts...)

		logger, err = cfg.Build(allOptions...)
		if err != nil {
			panic(err)
		}

	default:
		encoder := zap.NewDevelopmentEncoderConfig()
		encoder.TimeKey = "@timestamp"
		encoder.LevelKey = "log.level"
		encoder.MessageKey = "message"
		encoder.FunctionKey = "function.name"

		cfg := zap.NewDevelopmentConfig()
		cfg.EncoderConfig = encoder

		allOptions := append([]zap.Option{
			zap.WrapCore(func(c zapcore.Core) zapcore.Core {
				return zapcore.NewTee(
					// log errors to stderr
					zapcore.NewCore(
						zapcore.NewConsoleEncoder(encoder),
						zapcore.Lock(os.Stderr),
						zap.LevelEnablerFunc(func(l zapcore.Level) bool { return l >= zapcore.ErrorLevel }),
					),
					// send all logs to stdout
					zapcore.NewCore(
						zapcore.NewConsoleEncoder(encoder),
						zapcore.Lock(os.Stdout),
						zap.LevelEnablerFunc(func(l zapcore.Level) bool { return l >= zapcore.DebugLevel }),
					),
				)
			}),
		}, opts...)

		logger, err = cfg.Build(allOptions...)
		if err != nil {
			panic(err)
		}
	}

	if name := env.CurrentServiceName(); name != "" {
		logger = logger.Named(name).With(String("serviceName", name))
	}
	if version := env.GetBuildCommitHash(); version != "" {
		logger = logger.With(String("serviceVersion", version))
	}
	if txn := apm.TransactionFromContext(ctx); txn != nil {
		logger = logger.With(String(sentryTraceKey, txn.ToSentryTrace()))
	}

	logger = logger.With(String("environment", env.GetEnvironment().Name))
	return logger
}

// NewCLI creates a logger instance more suitable for CLI logging
func NewCLI() *Logger {
	logger, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}

	return logger
}

// SetLogger sets the default logger to the given logger. Subsequent
// attempts to access a logger via `New` or `FromContext` will use
// this logger.
func SetLogger(logger *Logger) {
	overrideLogger = logger
}

// NewCaptureLogger creates a logger that mimics production, but writes
// logs to the returned LogBuffer instead. Each log is written as a separate
// entry. This is useful for testing purposes and making assertions on logs.
func NewCaptureLogger() (logger *Logger, buf *LogBuffer, err error) {
	buf = new(LogBuffer)

	encoder := zap.NewProductionEncoderConfig()
	encoder.TimeKey = "@timestamp"
	encoder.LevelKey = "log.level"
	encoder.MessageKey = "message"
	encoder.FunctionKey = "function.name"

	cfg := zap.NewProductionConfig()
	cfg.EncoderConfig = encoder

	logger, err = cfg.Build(
		zap.WrapCore(func(c zapcore.Core) zapcore.Core {
			return zapcore.NewCore(
				zapcore.NewJSONEncoder(encoder),
				buf,
				zap.LevelEnablerFunc(func(l zapcore.Level) bool { return l >= zapcore.DebugLevel }),
			)
		}),
	)

	return
}

// LogBuffer is an in-memory store for logs, used by `NewCaptureLogger`
// to record and return the written logs for inspection.
type LogBuffer struct {
	Logs []string
}

// Write a log to the in-memory store.
func (l *LogBuffer) Write(p []byte) (int, error) {
	l.Logs = append(l.Logs, string(p))
	return len(p), nil
}

func (l *LogBuffer) Sync() error { return nil }
