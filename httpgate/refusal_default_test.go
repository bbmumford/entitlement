package httpgate

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bbmumford/entitlement"
)

func refusalReg() *entitlement.Registry {
	r := entitlement.NewRegistry()
	r.Register(entitlement.Feature{Key: "recording", Namespace: "app", AllowedPlans: []entitlement.PlanID{"pro"},
		Display: entitlement.Meta{UpgradeURL: "/upgrade/recording"}})
	r.Register(entitlement.Feature{Key: "beta", Namespace: "app", AllowedPlans: []entitlement.PlanID{"pro"}, Flag: "beta_flag"})
	return r
}

func answer(t *testing.T, mw func(http.Handler) http.Handler, ctx context.Context) (int, string) {
	t.Helper()
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil).WithContext(ctx))
	return rec.Code, rec.Body.String()
}

// The default gate answers each refusal its own way, so a client can tell a
// plan that lacks a feature (sell an upgrade) from a dark feature (hidden),
// a missing invite and an unresolved plan.
func TestDefaultGateAnswersEachRefusalByItsReason(t *testing.T) {
	pro := entitlement.WithPlan(context.Background(), "pro")
	starter := entitlement.WithPlan(context.Background(), "starter")
	g := NewGate(refusalReg(), entitlement.ExtractPlan)
	uninvited := NewGate(refusalReg(), entitlement.ExtractPlan, WithInviteResolver(func(context.Context, string) bool { return false }))

	if code, body := answer(t, g.RequireFeature("app", "beta"), pro); code != http.StatusNotFound || !strings.Contains(body, `"not_found"`) {
		t.Fatalf("dark feature: got %d %q, want 404 not_found", code, body)
	}
	if code, body := answer(t, g.RequireFeature("app", "recording"), starter); code != http.StatusForbidden ||
		!strings.Contains(body, `"feature_required"`) || !strings.Contains(body, `"recording"`) || !strings.Contains(body, `"/upgrade/recording"`) {
		t.Fatalf("plan lacks the feature: got %d %q, want 403 feature_required naming the feature and its upgrade URL", code, body)
	}
	if code, body := answer(t, uninvited.RequireFeature("app", "recording"), pro); code != http.StatusForbidden || !strings.Contains(body, `"not_invited"`) {
		t.Fatalf("not invited: got %d %q, want 403 not_invited", code, body)
	}
	if code, body := answer(t, g.RequireFeature("app", "recording"), context.Background()); code != http.StatusForbidden || !strings.Contains(body, `"plan_unresolved"`) {
		t.Fatalf("no plan: got %d %q, want 403 plan_unresolved", code, body)
	}
}
