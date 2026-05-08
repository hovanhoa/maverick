package log_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hovanhoa/llmgateway/pkg/core/log"
	"github.com/stretchr/testify/assert"
)

func BenchmarkNew(b *testing.B) {
	for i := 0; i < b.N; i++ {
		log.New()
	}
}

func BenchmarkNewWithField(b *testing.B) {
	for i := 0; i < b.N; i++ {
		log.New().With(
			log.String("key", "value"),
			log.Int("keyi", 1024),
			log.Float64("keyf", 1024.0),
		)
	}
}

func TestContextWithLoggerAndFromContext(t *testing.T) {
	// Create a capture logger to inspect the logs
	logger, buf, err := log.NewCaptureLogger()
	assert.NoError(t, err, "Failed to create capture logger")

	// Test 1: Basic context with logger functionality
	t.Run("BasicContextWithLogger", func(t *testing.T) {
		// Create a context with the logger
		ctx := log.ContextWithLogger(context.Background(), logger)

		// Retrieve logger from context
		retrievedLogger := log.FromContext(ctx)

		// Verify we get the same logger
		assert.Equal(t, logger, retrievedLogger, "FromContext should return the same logger that was stored in context")

		// Test that we can log with the retrieved logger
		retrievedLogger.Info("test message from context logger")

		assert.NotEmpty(t, buf.Logs, "No logs were captured")

		// Clear the buffer for next test
		buf.Logs = buf.Logs[:0]
	})

	// Test 2: Log fields propagate across contexts
	t.Run("LogFieldsPropagateAcrossContexts", func(t *testing.T) {
		// Create a logger with initial fields
		loggerWithFields := logger.With(
			log.String("service", "test-service"),
			log.String("version", "1.0.0"),
			log.Int("request_id", 12345),
		)

		// Store logger with fields in context
		ctx := log.ContextWithLogger(context.Background(), loggerWithFields)

		// Retrieve logger from context
		retrievedLogger := log.FromContext(ctx)

		// Add more fields to the retrieved logger
		loggerWithMoreFields := retrievedLogger.With(
			log.String("user_id", "user123"),
			log.Bool("authenticated", true),
		)

		// Log a message - should include all fields
		loggerWithMoreFields.Info("user action completed")

		assert.NotEmpty(t, buf.Logs, "No logs were captured")

		// Parse the last log entry to verify all fields are present
		lastLog := buf.Logs[len(buf.Logs)-1]
		var logEntry map[string]interface{}
		err := json.Unmarshal([]byte(lastLog), &logEntry)
		assert.NoError(t, err, "Failed to parse log entry")

		// Verify all expected fields are present
		expectedFields := map[string]interface{}{
			"service":       "test-service",
			"version":       "1.0.0",
			"request_id":    float64(12345), // JSON unmarshals numbers as float64
			"user_id":       "user123",
			"authenticated": true,
		}

		for key, expectedValue := range expectedFields {
			value, exists := logEntry[key]
			assert.True(t, exists, "Expected field '%s' not found in log entry", key)
			assert.Equal(t, expectedValue, value, "Field '%s' has wrong value", key)
		}

		// Clear the buffer for next test
		buf.Logs = buf.Logs[:0]
	})

	// Test 3: Nested context propagation
	t.Run("NestedContextPropagation", func(t *testing.T) {
		// Create base logger with fields
		baseLogger := logger.With(
			log.String("component", "api"),
			log.String("endpoint", "/users"),
		)

		// Create base context
		baseCtx := log.ContextWithLogger(context.Background(), baseLogger)

		// Create nested context with additional fields
		nestedLogger := log.FromContext(baseCtx).With(
			log.String("method", "GET"),
			log.String("user_agent", "test-client"),
		)
		nestedCtx := log.ContextWithLogger(baseCtx, nestedLogger)

		// Log from nested context
		log.FromContext(nestedCtx).Info("request processed")

		assert.NotEmpty(t, buf.Logs, "No logs were captured")

		// Parse the last log entry
		lastLog := buf.Logs[len(buf.Logs)-1]
		var logEntry map[string]interface{}
		err := json.Unmarshal([]byte(lastLog), &logEntry)
		assert.NoError(t, err, "Failed to parse log entry")

		// Verify all fields from both contexts are present
		expectedFields := map[string]interface{}{
			"component":  "api",
			"endpoint":   "/users",
			"method":     "GET",
			"user_agent": "test-client",
		}

		for key, expectedValue := range expectedFields {
			value, exists := logEntry[key]
			assert.True(t, exists, "Expected field '%s' not found in log entry", key)
			assert.Equal(t, expectedValue, value, "Field '%s' has wrong value", key)
		}

		// Clear the buffer for next test
		buf.Logs = buf.Logs[:0]
	})

	// Test 4: Context without logger falls back to default
	t.Run("ContextWithoutLoggerFallsBackToDefault", func(t *testing.T) {
		// Create a context without a logger
		ctx := context.Background()

		// Retrieve logger from context - should return default logger
		retrievedLogger := log.FromContext(ctx)

		// Verify we get a logger (not nil)
		assert.NotNil(t, retrievedLogger, "FromContext should not return nil logger for context without logger")

		// Test that we can log with the retrieved logger
		retrievedLogger.Info("test message from default logger")

		// Note: We can't easily verify the content of default logger logs
		// since they go to stdout/stderr, but we can verify the function doesn't panic
	})

	// Test 5: Multiple loggers in different contexts
	t.Run("MultipleLoggersInDifferentContexts", func(t *testing.T) {
		// Create two different loggers with different fields
		logger1 := logger.With(
			log.String("session", "session1"),
			log.String("user", "alice"),
		)

		logger2 := logger.With(
			log.String("session", "session2"),
			log.String("user", "bob"),
		)

		// Create two separate contexts
		ctx1 := log.ContextWithLogger(context.Background(), logger1)
		ctx2 := log.ContextWithLogger(context.Background(), logger2)

		// Log from both contexts
		log.FromContext(ctx1).Info("message from session 1")
		log.FromContext(ctx2).Info("message from session 2")

		assert.GreaterOrEqual(t, len(buf.Logs), 2, "Expected at least 2 log entries")

		// Define expected values for both session and user
		expectedData := []struct {
			session string
			user    string
		}{
			{session: "session1", user: "alice"},
			{session: "session2", user: "bob"},
		}

		// Verify the logs have the correct session and user information
		for i, expected := range expectedData {
			if i >= len(buf.Logs) {
				break
			}

			var logEntry map[string]interface{}
			err := json.Unmarshal([]byte(buf.Logs[i]), &logEntry)
			assert.NoError(t, err, "Failed to parse log entry %d", i)

			// Check session field
			session, exists := logEntry["session"]
			assert.True(t, exists, "Expected 'session' field not found in log entry %d", i)
			assert.Equal(t, expected.session, session, "Session field in log entry %d has wrong value", i)

			// Check user field
			user, exists := logEntry["user"]
			assert.True(t, exists, "Expected 'user' field not found in log entry %d", i)
			assert.Equal(t, expected.user, user, "User field in log entry %d has wrong value", i)
		}

		// Clear the buffer for next test
		buf.Logs = buf.Logs[:0]
	})

	// Test 6: Logger field chaining and context preservation
	t.Run("LoggerFieldChainingAndContextPreservation", func(t *testing.T) {
		// Create initial logger with fields
		initialLogger := logger.With(
			log.String("trace_id", "trace-123"),
			log.String("span_id", "span-456"),
		)

		// Store in context
		ctx := log.ContextWithLogger(context.Background(), initialLogger)

		// Chain multiple field additions
		logger1 := log.FromContext(ctx).With(log.String("step", "authentication"))
		logger2 := logger1.With(log.String("step", "authorization"))
		logger3 := logger2.With(log.String("step", "processing"))

		// Log with the final logger
		logger3.Info("request flow completed")

		assert.NotEmpty(t, buf.Logs, "No logs were captured")

		// Parse the last log entry
		lastLog := buf.Logs[len(buf.Logs)-1]
		var logEntry map[string]interface{}
		err := json.Unmarshal([]byte(lastLog), &logEntry)
		assert.NoError(t, err, "Failed to parse log entry")

		// Verify trace fields are preserved
		expectedFields := map[string]interface{}{
			"trace_id": "trace-123",
			"span_id":  "span-456",
			"step":     "processing", // Last step should override previous ones
		}

		for key, expectedValue := range expectedFields {
			value, exists := logEntry[key]
			assert.True(t, exists, "Expected field '%s' not found in log entry", key)
			assert.Equal(t, expectedValue, value, "Field '%s' has wrong value", key)
		}

		// Clear the buffer for next test
		buf.Logs = buf.Logs[:0]
	})

	// Test 7: Context cancellation doesn't affect logger
	t.Run("ContextCancellationDoesNotAffectLogger", func(t *testing.T) {
		// Create a context with logger
		ctx := log.ContextWithLogger(context.Background(), logger)

		// Cancel the context
		cancelCtx, cancel := context.WithCancel(ctx)
		cancel()

		// Verify the context is cancelled
		select {
		case <-cancelCtx.Done():
			// Expected
		default:
			t.Error("Context should be cancelled")
		}

		// Retrieve logger from cancelled context
		retrievedLogger := log.FromContext(cancelCtx)

		// Should still be able to log (logger is not affected by context cancellation)
		retrievedLogger.Info("message from cancelled context")

		assert.NotEmpty(t, buf.Logs, "No logs were captured from cancelled context")

		// Clear the buffer for next test
		buf.Logs = buf.Logs[:0]
	})

	// Test 8: Verify log message content
	t.Run("VerifyLogMessageContent", func(t *testing.T) {
		// Create logger with fields
		loggerWithFields := logger.With(
			log.String("test_id", "test-789"),
			log.Int("iteration", 42),
		)

		// Store in context
		ctx := log.ContextWithLogger(context.Background(), loggerWithFields)

		// Log different levels
		testMessage := "comprehensive test message"
		log.FromContext(ctx).Info(testMessage)

		assert.NotEmpty(t, buf.Logs, "No logs were captured")

		// Parse the last log entry
		lastLog := buf.Logs[len(buf.Logs)-1]
		var logEntry map[string]interface{}
		err := json.Unmarshal([]byte(lastLog), &logEntry)
		assert.NoError(t, err, "Failed to parse log entry")

		// Verify message field
		message, exists := logEntry["message"]
		assert.True(t, exists, "Expected 'message' field not found in log entry")
		assert.Equal(t, testMessage, message, "Message field has wrong value")

		// Verify level field
		level, exists := logEntry["log.level"]
		assert.True(t, exists, "Expected 'log.level' field not found in log entry")
		assert.Equal(t, "info", level, "Log level has wrong value")

		// Verify custom fields
		expectedFields := map[string]interface{}{
			"test_id":   "test-789",
			"iteration": float64(42),
		}

		for key, expectedValue := range expectedFields {
			value, exists := logEntry[key]
			assert.True(t, exists, "Expected field '%s' not found in log entry", key)
			assert.Equal(t, expectedValue, value, "Field '%s' has wrong value", key)
		}
	})
}
