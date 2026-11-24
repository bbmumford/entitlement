package httpgate

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bbmumford/entitlement"
)

func seededReg() *entitlement.Registry {
	r := entitlement.NewRegistry()
	r.Register(entitlement.Feature{Key: "recording", Namespace: "app", AllowedPlans: []entitlement.PlanID{"pro"}})
	r.Register(entitlement.Feature{Key: "beta", Namespace: "app", AllowedPlans: []entitlement.PlanID{"pro"}, Flag: "beta_flag"})
	return r
}

// serve runs a request through mw wrapping a 200 handler and returns the status code.
func serve(t *testing.T, mw func(http.Handler) http.Handler, ctx context.Context) int {
	t.Helper()
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
	h.ServeHTTP(rec, req)
	return rec.Code
}

func TestRequireFeature_AdmitsEntitledDeniesOthers(t *testing.T) {
	g := NewGate(seededReg(), entitlement.ExtractPlan)
	mw := g.RequireFeature("app", "recording")

	// entitled plan → 200
	if code := serve(t, mw, entitlement.WithPlan(context.Background(), "pro")); code != http.StatusOK {
		t.Fatalf("entitled plan must pass; got %d", code)
	}
	// unentitled plan → 403
	if code := serve(t, mw, entitlement.WithPlan(context.Background(), "starter")); code != http.StatusForbidden {
		t.Fatalf("unentitled plan must be 403; got %d", code)
	}
	// no plan on context → 403 (fail-closed)
	if code := serve(t, mw, context.Background()); code != http.StatusForbidden {
		t.Fatalf("an unresolved plan must be 403; got %d", code)
	}
	// unregistered feature → 403
	if code := serve(t, g.RequireFeature("app", "ghost"), entitlement.WithPlan(context.Background(), "pro")); code != http.StatusForbidden {
		t.Fatalf("an unregistered feature must be 403; got %d", code)
	}
}

func TestRequireFeature_FlaggedFeatureDarkWithoutResolver(t *testing.T) {
	// a flagged feature with NO flag resolver stays dark (fail-closed) even for an entitled plan
	g := NewGate(seededReg(), entitlement.ExtractPlan)
	if code := serve(t, g.RequireFeature("app", "beta"), entitlement.WithPlan(context.Background(), "pro")); code != http.StatusNotFound {
		t.Fatalf("a flagged feature with no resolver must be dark (404); got %d", code)
	}
	// with a resolver returning true, it opens
	gOn := NewGate(seededReg(), entitlement.ExtractPlan, WithFlagResolver(func(context.Context, string) bool { return true }))
	if code := serve(t, gOn.RequireFeature("app", "beta"), entitlement.WithPlan(context.Background(), "pro")); code != http.StatusOK {
		t.Fatalf("a flagged feature with flag-on resolver + entitled plan must pass; got %d", code)
	}
}

func TestRequireFeature_CustomDenier(t *testing.T) {
	called := false
	g := NewGate(seededReg(), entitlement.ExtractPlan, WithDenier(func(w http.ResponseWriter, _ *http.Request, _ entitlement.Namespace, _ string) {
		called = true
		w.WriteHeader(http.StatusPaymentRequired)
	}))
	if code := serve(t, g.RequireFeature("app", "recording"), context.Background()); code != http.StatusPaymentRequired || !called {
		t.Fatalf("custom denier must render; code=%d called=%v", code, called)
	}
}
