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

// RequireSet resolves each member's release flag through the Gate's FlagResolver: a set whose only
// member is dark answers 404 however the set's own flag reads, and admits once the member is released.
func TestRequireSetNeverConfersADarkMember(t *testing.T) {
	r := entitlement.NewRegistry()
	r.Register(entitlement.Feature{Key: "beta", Namespace: "app", AllowedPlans: []entitlement.PlanID{"pro"}, Flag: "beta_release"})
	r.RegisterSet(entitlement.FeatureSet{Key: "bundle", Namespace: "app", Members: []string{"beta"}})
	pro := entitlement.WithPlan(context.Background(), "pro")
	off := WithFlagResolver(func(_ context.Context, flag string) bool { return flag != "beta_release" })
	on := WithFlagResolver(func(context.Context, string) bool { return true })
	if code := serve(t, NewGate(r, entitlement.ExtractPlan, off).RequireSet("app", "bundle"), pro); code != http.StatusNotFound {
		t.Fatalf("a set holding a dark member admitted it; got %d", code)
	}
	if code := serve(t, NewGate(r, entitlement.ExtractPlan).RequireSet("app", "bundle"), pro); code != http.StatusNotFound {
		t.Fatalf("with no flag resolver a flagged member must stay dark; got %d", code)
	}
	if code := serve(t, NewGate(r, entitlement.ExtractPlan, on).RequireSet("app", "bundle"), pro); code != http.StatusOK {
		t.Fatalf("a released member must be conferred; got %d", code)
	}
}
