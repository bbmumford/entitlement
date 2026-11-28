/*
 * Copyright (c) 2026 Brandon Miller-Mumford.
 * Licensed under the MIT License; see LICENSE.
 */

package httpgate

import (
	"context"
	"net/http"
	"reflect"
	"testing"

	"github.com/bbmumford/entitlement"
)

// A wired InviteResolver decides only the records marked InviteOnly: an unmarked feature or set
// passes without the resolver being asked, so one resolver can serve every surface a host gates
// and a plan-locked-only surface is never refused as not invited.
func TestAWiredInviteResolverGatesOnlyInviteOnlyRecords(t *testing.T) {
	r := entitlement.NewRegistry()
	mode := map[string]entitlement.Param{"mode": {Values: []string{"x"}, Plans: map[entitlement.PlanID][]string{"pro": {"x"}}}}
	r.Register(entitlement.Feature{Key: "studio", Namespace: "app", AllowedPlans: []entitlement.PlanID{"pro"}, Params: mode, InviteOnly: true})
	r.Register(entitlement.Feature{Key: "recording", Namespace: "app", AllowedPlans: []entitlement.PlanID{"pro"}, Params: mode})
	r.RegisterSet(entitlement.FeatureSet{Key: "open", Namespace: "app", Members: []string{"recording"}})
	r.RegisterSet(entitlement.FeatureSet{Key: "preview", Namespace: "app", Members: []string{"recording"}, InviteOnly: true})
	pro := entitlement.WithPlan(context.Background(), "pro")
	var asked []string
	refusing := NewGate(r, entitlement.ExtractPlan, WithInviteResolver(func(_ context.Context, key string) bool {
		asked = append(asked, key)
		return false
	}))
	for _, c := range []struct {
		name string
		mw   func(http.Handler) http.Handler
		want int
	}{
		{"unmarked feature", refusing.RequireFeature("app", "recording"), http.StatusOK},
		{"unmarked feature param", refusing.RequireFeatureParam("app", "recording", "mode", "x"), http.StatusOK},
		{"unmarked set", refusing.RequireSet("app", "open"), http.StatusOK},
		{"invite-only feature", refusing.RequireFeature("app", "studio"), http.StatusForbidden},
		{"invite-only feature param", refusing.RequireFeatureParam("app", "studio", "mode", "x"), http.StatusForbidden},
		{"invite-only set", refusing.RequireSet("app", "preview"), http.StatusForbidden},
	} {
		if got := serve(t, c.mw, pro); got != c.want {
			t.Errorf("%s: %d, want %d", c.name, got, c.want)
		}
	}
	if want := []string{"studio", "studio", "preview"}; !reflect.DeepEqual(asked, want) {
		t.Fatalf("the resolver was asked for %v, want %v", asked, want)
	}
}
