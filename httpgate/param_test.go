/*
 * Copyright (c) 2026 Brandon Miller-Mumford.
 * Licensed under the MIT License; see LICENSE.
 */

package httpgate

import (
	"context"
	"net/http"
	"testing"

	"github.com/bbmumford/entitlement"
)

// TestRequireFeatureParam_AdmitsOnlyAPlanListingTheValue: the key alone is not
// enough; the plan's row must list the value, and the refusal names key(value).
func TestRequireFeatureParam_AdmitsOnlyAPlanListingTheValue(t *testing.T) {
	r := entitlement.NewRegistry()
	r.Register(entitlement.Feature{Key: "assistant", Namespace: "app", AllowedPlans: []entitlement.PlanID{"free", "pro"},
		Params: map[string]entitlement.Param{"action": {Values: []string{"refer"},
			Plans: map[entitlement.PlanID][]string{"pro": {"refer"}}}}})
	var denied string
	g := NewGate(r, entitlement.ExtractPlan, WithDenier(func(w http.ResponseWriter, _ *http.Request, _ entitlement.Namespace, key string) {
		denied = key
		w.WriteHeader(http.StatusForbidden)
	}))
	mw := g.RequireFeatureParam("app", "assistant", "action", "refer")
	if code := serve(t, mw, entitlement.WithPlan(context.Background(), "pro")); code != http.StatusOK {
		t.Fatalf("a plan listing the value must pass; got %d", code)
	}
	if code := serve(t, mw, entitlement.WithPlan(context.Background(), "free")); code != http.StatusForbidden {
		t.Fatalf("a plan holding the key without the value must be refused; got %d", code)
	}
	if denied != "assistant(refer)" {
		t.Errorf("the refusal names %q, want assistant(refer)", denied)
	}
	if code := serve(t, g.RequireFeatureParam("app", "assistant", "action", "other"), entitlement.WithPlan(context.Background(), "pro")); code != http.StatusForbidden {
		t.Fatalf("an undeclared value must be refused; got %d", code)
	}
	if code := serve(t, mw, context.Background()); code != http.StatusForbidden {
		t.Fatalf("an unresolved plan must be refused; got %d", code)
	}
}
