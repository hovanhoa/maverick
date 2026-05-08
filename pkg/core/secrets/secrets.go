package secrets

import (
	"os"

	"github.com/hovanhoa/llmgateway/pkg/core/errors"
)

func Require(secretName string) string {
	secret := Get(secretName)
	if secret == "" {
		panic(errors.New("secret not found: %q", secretName))
	}

	return secret
}

func Get(secretName string) string {
	return os.Getenv(secretName)
}
