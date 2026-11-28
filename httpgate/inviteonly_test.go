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

// With no InviteResolver an InviteOnly feature or set admits no one (fail closed), while an unmarked
// one is not invite-gated; a wired resolver decides both.
func TestAnInviteOnlySurfaceWithNoResolverAdmitsNoOne(t *testing.T) {
	r := entitlement.NewRegistry()
	r.Register(entitlement.Feature{Key: "studio", Namespace: "app", AllowedPlans: []entitlement.PlanID{"pro"}, InviteOnly: true})
	r.Register(entitlement.Feature{Key: "recording", Namespace: "app", AllowedPlans: []entitlement.PlanID{"pro"}})
	r.RegisterSet(entitlement.FeatureSet{Key: "preview", Namespace: "app", Members: []string{"recording"}, InviteOnly: true})
	r.RegisterSet(entitlement.FeatureSet{Key: "open", Namespace: "app", Members: []string{"recording"}})
	pro := entitlement.WithPlan(context.Background(), "pro")
	unwired := NewGate(r, entitlement.ExtractPlan)
	invited := NewGate(r, entitlement.ExtractPlan, WithInviteResolver(func(context.Context, string) bool { return true }))
	uninvited := NewGate(r, entitlement.ExtractPlan, WithInviteResolver(func(context.Context, string) bool { return false }))
	for _, c := range []struct {
		name string
		mw   func(http.Handler) http.Handler
		want int
	}{
		{"invite-only feature, no resolver", unwired.RequireFeature("app", "studio"), http.StatusForbidden},
		{"invite-only feature param, no resolver", unwired.RequireFeatureParam("app", "studio", "mode", "x"), http.StatusForbidden},
		{"invite-only set, no resolver", unwired.RequireSet("app", "preview"), http.StatusForbidden},
		{"unmarked feature, no resolver", unwired.RequireFeature("app", "recording"), http.StatusOK},
		{"unmarked set, no resolver", unwired.RequireSet("app", "open"), http.StatusOK},
		{"invite-only feature, invited", invited.RequireFeature("app", "studio"), http.StatusOK},
		{"invite-only set, invited", invited.RequireSet("app", "preview"), http.StatusOK},
		{"invite-only set, not invited", uninvited.RequireSet("app", "preview"), http.StatusForbidden},
	} {
		if got := serve(t, c.mw, pro); got != c.want {
			t.Errorf("%s: %d, want %d", c.name, got, c.want)
		}
	}
}
