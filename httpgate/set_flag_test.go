package httpgate

import (
	"context"
	"net/http"
	"testing"

	"github.com/bbmumford/entitlement"
)

func setReg() *entitlement.Registry {
	r := seededReg()
	r.RegisterSet(entitlement.FeatureSet{Key: "open_surface", Namespace: "app", Members: []string{"recording"}})
	r.RegisterSet(entitlement.FeatureSet{Key: "staged_surface", Namespace: "app", Members: []string{"recording"}, Flag: "staged_flag"})
	return r
}

// TestRequireSet_FlagHoldsTheWholeSurfaceDark: a set's release flag gates the
// surface exactly as a feature's does, and a set without one is unaffected.
func TestRequireSet_FlagHoldsTheWholeSurfaceDark(t *testing.T) {
	pro := entitlement.WithPlan(context.Background(), "pro")
	var askedFor string
	on := WithFlagResolver(func(_ context.Context, flag string) bool { askedFor = flag; return true })
	off := WithFlagResolver(func(context.Context, string) bool { return false })

	if code := serve(t, NewGate(setReg(), entitlement.ExtractPlan).RequireSet("app", "open_surface"), pro); code != http.StatusOK {
		t.Fatalf("a flagless set with no resolver must admit an entitled plan; got %d", code)
	}
	if code := serve(t, NewGate(setReg(), entitlement.ExtractPlan).RequireSet("app", "staged_surface"), pro); code != http.StatusForbidden {
		t.Fatalf("a flagged set with no resolver must stay dark; got %d", code)
	}
	if code := serve(t, NewGate(setReg(), entitlement.ExtractPlan, off).RequireSet("app", "staged_surface"), pro); code != http.StatusForbidden {
		t.Fatalf("a flagged set whose flag is off must deny; got %d", code)
	}
	if code := serve(t, NewGate(setReg(), entitlement.ExtractPlan, on).RequireSet("app", "staged_surface"), pro); code != http.StatusOK {
		t.Fatalf("a flagged set whose flag is on must admit an entitled plan; got %d", code)
	}
	if askedFor != "staged_flag" {
		t.Fatalf("the resolver was asked about %q, want the set's declared flag", askedFor)
	}
	if code := serve(t, NewGate(setReg(), entitlement.ExtractPlan, on).RequireSet("app", "staged_surface"), entitlement.WithPlan(context.Background(), "starter")); code != http.StatusForbidden {
		t.Fatalf("a flag that is on must not admit a plan the set does not allow; got %d", code)
	}
}
