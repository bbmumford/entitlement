/*
 * Copyright (c) 2026 Brandon Miller-Mumford.
 * Licensed under the MIT License; see LICENSE.
 */

package entitlement

import "context"

// planContextKey is the unexported context key for the request's entitlement plan — a distinct
// zero-size type so it never collides with another package's context value.
type planContextKey struct{}

// WithPlan carries the caller's resolved opaque PlanID on the request context for the gate to read. Each
// consumer resolves its own plan source to a PlanID and stores the RESULT here, so the gate and any
// middleware depend only on the opaque PlanID, never on a consumer's plan store.
func WithPlan(ctx context.Context, plan PlanID) context.Context {
	return context.WithValue(ctx, planContextKey{}, plan)
}

// ExtractPlan returns the PlanID carried on ctx. ok is false when no plan was set — fail-closed: a caller
// with no resolved plan is entitled to nothing, so the gate must treat the missing case as deny, never
// as a default plan.
func ExtractPlan(ctx context.Context) (PlanID, bool) {
	plan, ok := ctx.Value(planContextKey{}).(PlanID)
	return plan, ok
}
