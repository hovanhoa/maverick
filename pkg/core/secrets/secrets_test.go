package secrets

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGet(t *testing.T) {
	tests := []struct {
		name       string
		secretName string
		envValue   string
		expected   string
	}{
		{
			name:       "returns value when env var is set",
			secretName: "TEST_SECRET_GET",
			envValue:   "test-secret-value",
			expected:   "test-secret-value",
		},
		{
			name:       "returns empty string when env var is not set",
			secretName: "TEST_SECRET_GET_UNSET",
			envValue:   "",
			expected:   "",
		},
		{
			name:       "handles special characters in value",
			secretName: "TEST_SECRET_GET_SPECIAL",
			envValue:   "test=value&with!special@chars#",
			expected:   "test=value&with!special@chars#",
		},
		{
			name:       "handles whitespace in value",
			secretName: "TEST_SECRET_GET_WHITESPACE",
			envValue:   "  value with spaces  ",
			expected:   "  value with spaces  ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				t.Setenv(tt.secretName, tt.envValue)
			}

			result := Get(tt.secretName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRequire(t *testing.T) {
	t.Run("returns value when env var is set", func(t *testing.T) {
		secretName := "TEST_SECRET_REQUIRE_SET"
		expectedValue := "required-secret-value"
		t.Setenv(secretName, expectedValue)

		result := Require(secretName)
		assert.Equal(t, expectedValue, result)
	})

	t.Run("panics when env var is not set", func(t *testing.T) {
		secretName := "TEST_SECRET_REQUIRE_UNSET_PANIC"

		defer func() {
			// r := recover()
			// if r == nil {
			// 	t.Error("Require() should have panicked for unset secret")
			// }
			assert.Panics(t, func() {
				Require(secretName)
			})
		}()
	})

	t.Run("panics when env var is empty string", func(t *testing.T) {
		secretName := "TEST_SECRET_REQUIRE_EMPTY"
		t.Setenv(secretName, "")

		defer func() {
			assert.Panics(t, func() {
				Require(secretName)
			})
		}()
	})
}
