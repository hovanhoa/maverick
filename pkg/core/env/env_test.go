package env_test

import (
	"os"
	"testing"

	"github.com/hovanhoa/llmgateway/pkg/core/env"
	"github.com/stretchr/testify/assert"
)

func TestEnvironment(t *testing.T) {
	original := os.Getenv("ENVIRONMENT")
	t.Cleanup(func() {
		_ = os.Setenv("ENVIRONMENT", original)
	})

	_ = os.Setenv("ENVIRONMENT", "production")
	assert.Same(t, env.Production, env.GetEnvironment())
	assert.Equal(t, env.ProductionDomain, env.GetDomain())

	_ = os.Setenv("ENVIRONMENT", "staging")
	assert.Same(t, env.Staging, env.GetEnvironment())
	assert.Equal(t, env.StagingDomain, env.GetDomain())

	_ = os.Setenv("ENVIRONMENT", "dev")
	_ = os.Setenv("OYSTER_USER", "test")
	assert.Same(t, env.Dev, env.GetEnvironment())
	assert.Equal(t, "test.dev.oysterinc.net", env.GetDomain().String())
}

func TestService_ExternalURL(t *testing.T) {
	original := os.Getenv("ENVIRONMENT")
	t.Cleanup(func() {
		_ = os.Setenv("ENVIRONMENT", original)
		_ = os.Setenv("API_HOST_URL", "")
		_ = os.Setenv("DASHBOARD_HOST_URL", "")
		_ = os.Setenv("STATICS_HOST_URL", "")
	})

	_ = os.Setenv("ENVIRONMENT", "production")
	assert.Equal(t, "https://api.getcara.ai", env.ServiceAPI.ExternalURL())
	assert.Equal(t, "https://s.getcara.ai", env.ServiceStatics.ExternalURL())
	assert.Equal(t, "https://webhooks.getcara.ai", env.ServiceWebhooks.ExternalURL())

	_ = os.Setenv("ENVIRONMENT", "staging")
	assert.Equal(t, "https://api.staging.getcara.ai", env.ServiceAPI.ExternalURL())
	assert.Equal(t, "https://s.staging.getcara.ai", env.ServiceStatics.ExternalURL())
	assert.Equal(t, "https://webhooks.staging.getcara.ai", env.ServiceWebhooks.ExternalURL())

	_ = os.Setenv("ENVIRONMENT", "dev")
	_ = os.Setenv("OYSTER_USER", "test")
	_ = os.Setenv("OYSTER_WORKTREE", "")
	assert.Equal(t, "https://api.test.dev.oysterinc.net", env.ServiceAPI.ExternalURL())
	assert.Equal(t, "https://statics.test.dev.oysterinc.net", env.ServiceStatics.ExternalURL())
	assert.Equal(t, "https://webhooks.test.dev.oysterinc.net", env.ServiceWebhooks.ExternalURL())

	_ = os.Setenv("OYSTER_WORKTREE", "worktree")
	assert.Equal(t, "https://api.worktree.test.dev.oysterinc.net", env.ServiceAPI.ExternalURL())
	assert.Equal(t, "https://statics.worktree.test.dev.oysterinc.net", env.ServiceStatics.ExternalURL())
	assert.Equal(t, "https://webhooks.worktree.test.dev.oysterinc.net", env.ServiceWebhooks.ExternalURL())

	_ = os.Setenv("API_HOST_URL", "my_api_url")
	assert.Equal(t, "my_api_url", env.ServiceAPI.ExternalURL())

	_ = os.Setenv("STATICS_HOST_URL", "my_statics_url")
	assert.Equal(t, "my_statics_url", env.ServiceStatics.ExternalURL())
}

func TestService_InternalURL(t *testing.T) {
	original := os.Getenv("ENVIRONMENT")
	t.Cleanup(func() {
		_ = os.Setenv("ENVIRONMENT", original)
	})

	_ = os.Setenv("ENVIRONMENT", "production")
	assert.Equal(t, "http://api.production.svc", env.ServiceAPI.InternalURL())
	assert.Equal(t, "http://statics.production.svc", env.ServiceStatics.InternalURL())
	assert.Equal(t, "http://webhooks.production.svc", env.ServiceWebhooks.InternalURL())

	_ = os.Setenv("ENVIRONMENT", "staging")
	assert.Equal(t, "http://api.staging.svc", env.ServiceAPI.InternalURL())
	assert.Equal(t, "http://statics.staging.svc", env.ServiceStatics.InternalURL())
	assert.Equal(t, "http://webhooks.staging.svc", env.ServiceWebhooks.InternalURL())

	_ = os.Setenv("ENVIRONMENT", "dev")
	_ = os.Setenv("OYSTER_USER", "test")
	_ = os.Setenv("OYSTER_WORKTREE", "")
	assert.Equal(t, "http://api.dev-test.svc", env.ServiceAPI.InternalURL())
	assert.Equal(t, "http://statics.dev-test.svc", env.ServiceStatics.InternalURL())
	assert.Equal(t, "http://webhooks.dev-test.svc", env.ServiceWebhooks.InternalURL())

	_ = os.Setenv("OYSTER_WORKTREE", "worktree")
	assert.Equal(t, "http://api.dev-test-worktree.svc", env.ServiceAPI.InternalURL())
	assert.Equal(t, "http://statics.dev-test-worktree.svc", env.ServiceStatics.InternalURL())
	assert.Equal(t, "http://webhooks.dev-test-worktree.svc", env.ServiceWebhooks.InternalURL())
}
