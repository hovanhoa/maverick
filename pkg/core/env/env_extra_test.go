package env_test

import (
	"os"
	"testing"

	"github.com/hovanhoa/llmgateway/pkg/core/env"
	"github.com/stretchr/testify/assert"
)

func TestGetBuildCommitHash(t *testing.T) {
	assert.Equal(t, env.BuildCommitHash, env.GetBuildCommitHash())
}

func TestCurrentServiceName(t *testing.T) {
	original := os.Getenv("OYSTER_SERVICE")
	t.Cleanup(func() { _ = os.Setenv("OYSTER_SERVICE", original) })

	_ = os.Setenv("OYSTER_SERVICE", "api")
	assert.Equal(t, "api", env.CurrentServiceName())

	_ = os.Setenv("OYSTER_SERVICE", "")
	assert.Equal(t, "", env.CurrentServiceName())
}

func TestGetInternalDomain(t *testing.T) {
	original := os.Getenv("ENVIRONMENT")
	originalUser := os.Getenv("OYSTER_USER")
	t.Cleanup(func() {
		_ = os.Setenv("ENVIRONMENT", original)
		_ = os.Setenv("OYSTER_USER", originalUser)
	})

	_ = os.Setenv("ENVIRONMENT", "production")
	assert.Equal(t, env.InternalProductionDomain, env.GetInternalDomain())

	_ = os.Setenv("ENVIRONMENT", "staging")
	assert.Equal(t, env.InternalStagingDomain, env.GetInternalDomain())

	_ = os.Setenv("ENVIRONMENT", "dev")
	_ = os.Setenv("OYSTER_USER", "testuser")
	assert.Equal(t, "testuser.dev.oysterinc.net", env.GetInternalDomain().String())
}

func TestIngressAPIKeyPrefix(t *testing.T) {
	original := os.Getenv("ENVIRONMENT")
	originalUser := os.Getenv("OYSTER_USER")
	t.Cleanup(func() {
		_ = os.Setenv("ENVIRONMENT", original)
		_ = os.Setenv("OYSTER_USER", originalUser)
	})

	_ = os.Setenv("ENVIRONMENT", "production")
	assert.Equal(t, "production", env.IngressAPIKeyPrefix())

	_ = os.Setenv("ENVIRONMENT", "staging")
	assert.Equal(t, "staging", env.IngressAPIKeyPrefix())

	_ = os.Setenv("ENVIRONMENT", "dev")
	_ = os.Setenv("OYSTER_USER", "myuser")
	assert.Equal(t, "dev-myuser", env.IngressAPIKeyPrefix())

	_ = os.Setenv("ENVIRONMENT", "dev")
	_ = os.Setenv("OYSTER_USER", "")
	assert.Equal(t, "dev-dev", env.IngressAPIKeyPrefix())
}

func TestGetUserDevDomain(t *testing.T) {
	assert.Equal(t, "alice.dev.oysterinc.net", env.GetUserDevDomain("alice", ""))
	assert.Equal(t, "worktree.alice.dev.oysterinc.net", env.GetUserDevDomain("alice", "worktree"))
}

func TestDomain_String(t *testing.T) {
	d := env.ProductionDomain
	assert.Equal(t, "getcara.ai", d.String())
}

func TestService_GetSubDomain(t *testing.T) {
	assert.Equal(t, "s", env.ServiceStatics.GetSubDomain())
	assert.Equal(t, "api", env.ServiceAPI.GetSubDomain())
}

func TestService_GetURLEnv(t *testing.T) {
	assert.Equal(t, "API_HOST_URL", env.ServiceAPI.GetURLEnv())
	assert.Equal(t, "STATICS_HOST_URL", env.ServiceStatics.GetURLEnv())
}
