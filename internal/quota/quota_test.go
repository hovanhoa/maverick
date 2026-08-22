package quota_test

import (
	"context"
	"testing"

	"github.com/hovanhoa/llmgateway/internal/quota"
	"github.com/hovanhoa/llmgateway/pkg/driver/memkv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReserve_NilBudgetIsUnlimited(t *testing.T) {
	t.Parallel()

	c := quota.NewChecker(memkv.New())
	_, err := c.Reserve(context.Background(), "team_1", nil, 1_000_000)
	require.NoError(t, err)

	usage, err := c.Usage(context.Background(), "team_1")
	require.NoError(t, err)
	assert.Zero(t, usage, "an unlimited reservation must not even touch the counter")
}

func TestReserve_WithinBudget(t *testing.T) {
	t.Parallel()

	c := quota.NewChecker(memkv.New())
	budget := 1000

	_, err := c.Reserve(context.Background(), "team_1", &budget, 400)
	require.NoError(t, err)
	usage, err := c.Usage(context.Background(), "team_1")
	require.NoError(t, err)
	assert.Equal(t, 400, usage)
}

func TestReserve_ExceedsBudget(t *testing.T) {
	t.Parallel()

	c := quota.NewChecker(memkv.New())
	budget := 1000

	_, err := c.Reserve(context.Background(), "team_1", &budget, 900)
	require.NoError(t, err)
	_, err = c.Reserve(context.Background(), "team_1", &budget, 200)
	require.ErrorIs(t, err, quota.ErrExceeded)

	// The rejected reservation must not have been applied.
	usage, err := c.Usage(context.Background(), "team_1")
	require.NoError(t, err)
	assert.Equal(t, 900, usage)
}

func TestReserve_ExactlyAtBudgetSucceeds(t *testing.T) {
	t.Parallel()

	c := quota.NewChecker(memkv.New())
	budget := 1000

	_, err := c.Reserve(context.Background(), "team_1", &budget, 1000)
	require.NoError(t, err)
	usage, err := c.Usage(context.Background(), "team_1")
	require.NoError(t, err)
	assert.Equal(t, 1000, usage)
}

func TestReconcile_AdjustsToActualUsage(t *testing.T) {
	t.Parallel()

	c := quota.NewChecker(memkv.New())
	budget := 10000

	window, err := c.Reserve(context.Background(), "team_1", &budget, 1000)
	require.NoError(t, err)
	require.NoError(t, c.Reconcile(context.Background(), window, &budget, 1000, 250))

	usage, err := c.Usage(context.Background(), "team_1")
	require.NoError(t, err)
	assert.Equal(t, 250, usage, "the counter must reflect actual usage, not the estimate")
}

func TestReconcile_ReleaseOnFailure(t *testing.T) {
	t.Parallel()

	c := quota.NewChecker(memkv.New())
	budget := 10000

	window, err := c.Reserve(context.Background(), "team_1", &budget, 1000)
	require.NoError(t, err)
	require.NoError(t, c.Reconcile(context.Background(), window, &budget, 1000, 0))

	usage, err := c.Usage(context.Background(), "team_1")
	require.NoError(t, err)
	assert.Zero(t, usage)
}

func TestReconcile_NeverGoesNegative(t *testing.T) {
	t.Parallel()

	c := quota.NewChecker(memkv.New())
	budget := 10000

	window, err := c.Reserve(context.Background(), "team_1", &budget, 0)
	require.NoError(t, err)

	// Reconciling a larger estimate than the counter actually holds (e.g. a
	// race where a second Reconcile for the same call runs against an
	// already-zeroed counter) must clamp at zero rather than go negative.
	require.NoError(t, c.Reconcile(context.Background(), window, &budget, 500, 0))

	usage, err := c.Usage(context.Background(), "team_1")
	require.NoError(t, err)
	assert.Zero(t, usage)
}

func TestReconcile_NilBudgetIsNoOp(t *testing.T) {
	t.Parallel()

	c := quota.NewChecker(memkv.New())

	// A nil budget must be a no-op, mirroring Reserve's "unlimited"
	// convention - otherwise an unlimited team's real usage would still
	// accumulate in the counter Reserve never touched, and would wrongly
	// count against a budget assigned to that team later.
	window, err := c.Reserve(context.Background(), "team_1", nil, 1000)
	require.NoError(t, err)
	require.NoError(t, c.Reconcile(context.Background(), window, nil, 1000, 5000))

	usage, err := c.Usage(context.Background(), "team_1")
	require.NoError(t, err)
	assert.Zero(t, usage)
}

func TestUsage_UnknownTeamIsZero(t *testing.T) {
	t.Parallel()

	c := quota.NewChecker(memkv.New())
	usage, err := c.Usage(context.Background(), "team_never_used")
	require.NoError(t, err)
	assert.Zero(t, usage)
}

func TestReserve_SeparateTeamsAreIndependent(t *testing.T) {
	t.Parallel()

	c := quota.NewChecker(memkv.New())
	budget := 100

	_, err := c.Reserve(context.Background(), "team_a", &budget, 100)
	require.NoError(t, err)
	_, err = c.Reserve(context.Background(), "team_b", &budget, 100)
	require.NoError(t, err)

	usageA, _ := c.Usage(context.Background(), "team_a")
	usageB, _ := c.Usage(context.Background(), "team_b")
	assert.Equal(t, 100, usageA)
	assert.Equal(t, 100, usageB)
}

func TestReserve_ReturnsStableWindowAcrossCalls(t *testing.T) {
	t.Parallel()

	c := quota.NewChecker(memkv.New())
	budget := 1000

	window1, err := c.Reserve(context.Background(), "team_1", &budget, 100)
	require.NoError(t, err)
	window2, err := c.Reserve(context.Background(), "team_1", &budget, 100)
	require.NoError(t, err)
	assert.Equal(t, window1, window2, "reservations made in the same month for the same team must target the same window")
}
