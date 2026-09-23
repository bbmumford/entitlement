package entitlement

import (
	"context"
	"testing"
)

func seeded() *Registry {
	r := NewRegistry()
	r.Register(Feature{
		Key:          "recording",
		Namespace:    "app",
		AllowedPlans: []PlanID{"pro", "enterprise"},
		Limit:        map[PlanID]int64{"pro": 100, "enterprise": -1},
	})
	r.Register(Feature{
		Key:          "beta_editor",
		Namespace:    "app",
		AllowedPlans: []PlanID{"pro", "enterprise"},
		Flag:         "beta_editor_flag",
	})
	r.Register(Feature{Key: "research", Namespace: "app", AllowedPlans: []PlanID{"enterprise"}})
	r.RegisterSet(FeatureSet{Key: "studio", Namespace: "app", Members: []string{"recording", "research"}, RequireAll: true})
	r.RegisterSet(FeatureSet{Key: "any_tool", Namespace: "app", Members: []string{"recording", "research"}, RequireAll: false})
	return r
}

func TestAllowed_SetMembershipFailClosed(t *testing.T) {
	r := seeded()
	if !r.Allowed("app", "pro", "recording") {
		t.Fatal("pro must be entitled to recording")
	}
	if r.Allowed("app", "starter", "recording") {
		t.Fatal("a plan not in AllowedPlans must be denied")
	}
	// fail-closed on an unregistered feature, unknown namespace, and unlisted opaque plan
	if r.Allowed("app", "pro", "nonexistent") {
		t.Fatal("an unregistered feature must deny")
	}
	if r.Allowed("other", "pro", "recording") {
		t.Fatal("an unknown namespace must deny")
	}
	if r.Allowed("app", "totally-made-up-plan", "recording") {
		t.Fatal("an unlisted opaque plan must deny (set-membership is validity)")
	}
}

func TestLimit_PerPlanCapFailClosed(t *testing.T) {
	r := seeded()
	if v, ok := r.Limit("app", "pro", "recording"); !ok || v != 100 {
		t.Fatalf("pro limit: v=%d ok=%v", v, ok)
	}
	if v, ok := r.Limit("app", "enterprise", "recording"); !ok || v != -1 {
		t.Fatalf("enterprise unlimited: v=%d ok=%v", v, ok)
	}
	// a feature with no numeric axis, and an unlisted plan, both fail closed to (0,false)
	if _, ok := r.Limit("app", "pro", "research"); ok {
		t.Fatal("a feature with no Limit map must return ok=false")
	}
	if _, ok := r.Limit("app", "starter", "recording"); ok {
		t.Fatal("a plan with no cap must return ok=false (deny, not unlimited)")
	}
}

func TestAvailable_FlagPlanInvite(t *testing.T) {
	r := seeded()
	// a flagless feature: available on plan + invite regardless of flagOn
	if !r.Available("app", "pro", "recording", false, true) {
		t.Fatal("a flagless feature must ignore flagOn=false")
	}
	// a flagged feature stays dark until flagOn
	if r.Available("app", "pro", "beta_editor", false, true) {
		t.Fatal("a flagged feature must be dark when flagOn=false")
	}
	if !r.Available("app", "pro", "beta_editor", true, true) {
		t.Fatal("a flagged feature must be available when flag on + plan + invited")
	}
	// invite gate
	if r.Available("app", "pro", "recording", true, false) {
		t.Fatal("a non-invited caller must be denied")
	}
	// plan gate
	if r.Available("app", "starter", "recording", true, true) {
		t.Fatal("an unentitled plan must be denied even when flag on + invited")
	}
}

func TestSetAvailable_RequireAllVsAny(t *testing.T) {
	r := seeded()
	// pro has recording but NOT research → require-all "studio" fails, any "any_tool" passes
	if r.SetAvailable("app", "pro", "studio", true, true) {
		t.Fatal("require-all set must fail when a member is denied")
	}
	if !r.SetAvailable("app", "pro", "any_tool", true, true) {
		t.Fatal("any-mode set must pass when at least one member is allowed")
	}
	// enterprise has both → require-all passes
	if !r.SetAvailable("app", "enterprise", "studio", true, true) {
		t.Fatal("require-all set must pass when every member is allowed")
	}
	// fail-closed: unregistered set, or flag/invite false
	if r.SetAvailable("app", "enterprise", "nope", true, true) {
		t.Fatal("an unregistered set must deny")
	}
	if r.SetAvailable("app", "enterprise", "studio", false, true) || r.SetAvailable("app", "enterprise", "studio", true, false) {
		t.Fatal("a false flag or invite must deny the set")
	}
}

func TestContext_WithAndExtractPlan(t *testing.T) {
	ctx := WithPlan(context.Background(), "pro")
	if p, ok := ExtractPlan(ctx); !ok || p != "pro" {
		t.Fatalf("extract: p=%q ok=%v", p, ok)
	}
	// fail-closed: no plan on the context → not ok (deny, never a default)
	if _, ok := ExtractPlan(context.Background()); ok {
		t.Fatal("a context with no plan must report ok=false")
	}
}

func TestRegistry_FeaturesEnumerates(t *testing.T) {
	r := seeded()
	if got := len(r.Features()); got != 3 {
		t.Fatalf("Features() must enumerate the 3 registered features; got %d", got)
	}
}

// TestNilRegistry_RefusesEveryGate: a composition root that never built its
// registry hands the gates a nil pointer, and every question asked of it must
// deny rather than panic inside a request.
func TestNilRegistry_RefusesEveryGate(t *testing.T) {
	var r *Registry
	if _, ok := r.Get("app", "recording"); ok {
		t.Fatal("Get on a nil registry must report no feature")
	}
	if _, ok := r.GetSet("app", "studio"); ok {
		t.Fatal("GetSet on a nil registry must report no set")
	}
	if got := r.Features(); len(got) != 0 {
		t.Fatalf("Features on a nil registry must be empty; got %d", len(got))
	}
	if r.Allowed("app", "pro", "recording") {
		t.Fatal("Allowed on a nil registry must deny")
	}
	if _, ok := r.Limit("app", "pro", "recording"); ok {
		t.Fatal("Limit on a nil registry must report no allowance")
	}
	if r.Available("app", "pro", "recording", true, true) {
		t.Fatal("Available on a nil registry must deny")
	}
	if r.SetAvailable("app", "pro", "studio", true, true) {
		t.Fatal("SetAvailable on a nil registry must deny")
	}
	// Seeding a nil registry stores nothing, so the gate still denies.
	r.Register(Feature{Key: "recording", Namespace: "app", AllowedPlans: []PlanID{"pro"}})
	r.RegisterSet(FeatureSet{Key: "studio", Namespace: "app", Members: []string{"recording"}})
	if r.Allowed("app", "pro", "recording") || r.SetAvailable("app", "pro", "studio", true, true) {
		t.Fatal("a nil registry must hold nothing after Register, so every gate still denies")
	}
}

// TestZeroValueRegistry_Seeds: a Registry declared as a value rather than built
// by NewRegistry has nil maps, and seeding it must store the rows instead of
// panicking on the first write.
func TestZeroValueRegistry_Seeds(t *testing.T) {
	var r Registry
	if r.Allowed("app", "pro", "recording") {
		t.Fatal("an unseeded zero-value registry must deny")
	}
	r.Register(Feature{Key: "recording", Namespace: "app", AllowedPlans: []PlanID{"pro"}})
	r.RegisterSet(FeatureSet{Key: "studio", Namespace: "app", Members: []string{"recording"}})
	if !r.Allowed("app", "pro", "recording") {
		t.Fatal("a zero-value registry must hold what was registered into it")
	}
	if !r.SetAvailable("app", "pro", "studio", true, true) {
		t.Fatal("a zero-value registry must hold the set registered into it")
	}
}
