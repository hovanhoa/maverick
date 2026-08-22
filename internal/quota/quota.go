// Package quota implements per-team monthly token budgets for the LLM
// proxy: a pre-call reservation against a fast Redis counter, and a
// post-call reconciliation once the provider reports actual token usage.
//
// Team/account/key/model are all plausible quota dimensions; this phase
// implements exactly one - a per-team, calendar-month token budget - since
// it composes naturally with the Phase 2 per-team model allowlist. Other
// dimensions can reuse the same key-per-window pattern later.
package quota

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/hovanhoa/llmgateway/pkg/core/errors"
	"github.com/hovanhoa/llmgateway/pkg/driver"
)

// ErrExceeded is returned by Reserve when granting the request would push
// the team over its monthly token budget.
var ErrExceeded = errors.New("quota exceeded")

// Checker enforces per-team monthly token budgets against a KVStore.
type Checker struct {
	kv driver.KVStore
}

// NewChecker returns a new Checker backed by the given KVStore.
func NewChecker(kv driver.KVStore) *Checker {
	return &Checker{kv: kv}
}

// windowKey is the counter key for a team's current calendar-month window.
func windowKey(teamID string, now time.Time) string {
	return fmt.Sprintf("quota:%s:%s", teamID, now.UTC().Format("2006-01"))
}

func parseCount(raw string) int {
	if raw == "" {
		return 0
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	return n
}

// Reserve atomically adds estimate to the team's running total for the
// current month and returns ErrExceeded (without applying the reservation)
// if doing so would exceed budget. A nil budget means unlimited - no
// reservation is made and Reserve always succeeds, matching the "empty
// allowlist = unrestricted" convention used for the Phase 2 model
// allowlist.
//
// It returns the window key the reservation was made against, which the
// caller must pass back to the matching Reconcile call unchanged - the two
// must agree on which calendar-month counter they're adjusting even if the
// reconciliation happens to run after Reserve's month has just rolled over
// (e.g. a call reserved in the last second of one month and reconciled in
// the first second of the next). Recomputing "now" independently at
// Reconcile time would target the new month's counter instead.
//
// A successful Reserve must eventually be balanced by a call to Reconcile
// (with actual=0 to fully release it if the call never completed).
func (c *Checker) Reserve(ctx context.Context, teamID string, budget *int, estimate int) (window string, err error) {
	window = windowKey(teamID, time.Now())
	if budget == nil {
		return window, nil
	}

	var exceeded bool

	err = c.kv.GetAndSet(ctx, func(kv map[string]string) (map[string]string, error) {
		current := parseCount(kv[window])
		if current+estimate > *budget {
			exceeded = true
			return map[string]string{}, nil
		}
		return map[string]string{window: strconv.Itoa(current + estimate)}, nil
	}, window)
	if err != nil {
		return window, errors.Wrap(err, "quota.Reserve")
	}
	if exceeded {
		return window, ErrExceeded
	}

	return window, nil
}

// Reconcile adjusts the running total by (actual - estimate), so the
// counter reflects real usage rather than the upfront estimate used to
// reserve it. Call with actual=0 to fully release a reservation (e.g. the
// upstream call failed before producing any usage). window must be the
// value Reserve returned for the reservation being reconciled.
//
// budget must match whatever was passed to the Reserve call being
// reconciled: a nil budget is a no-op, mirroring Reserve's "unlimited"
// convention. Without this guard, a call for a nil-budget (unlimited) team
// would still be reconciled into the same Redis counter Reserve never
// touched, leaving a stale nonzero balance that would incorrectly count
// against that team's budget if one is configured later.
func (c *Checker) Reconcile(ctx context.Context, window string, budget *int, estimate int, actual int) error {
	if budget == nil {
		return nil
	}

	delta := actual - estimate
	if delta == 0 {
		return nil
	}

	err := c.kv.GetAndSet(ctx, func(kv map[string]string) (map[string]string, error) {
		next := parseCount(kv[window]) + delta
		if next < 0 {
			next = 0
		}
		return map[string]string{window: strconv.Itoa(next)}, nil
	}, window)
	if err != nil {
		return errors.Wrap(err, "quota.Reconcile")
	}

	return nil
}

// Usage returns the current month's running total for a team.
func (c *Checker) Usage(ctx context.Context, teamID string) (int, error) {
	found, v, err := c.kv.Get(ctx, windowKey(teamID, time.Now()))
	if err != nil {
		return 0, errors.Wrap(err, "quota.Usage")
	}
	if !found {
		return 0, nil
	}
	return parseCount(v), nil
}
