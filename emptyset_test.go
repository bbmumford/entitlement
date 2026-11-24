package entitlement

import "testing"

// A set naming no member entitles no plan, in either mode: an empty
// require-all set must not open its surface to every plan, a plan the
// catalog has never heard of included.
func TestSetAvailable_AnEmptySetEntitlesNoPlan(t *testing.T) {
	r := seeded()
	r.RegisterSet(FeatureSet{Key: "suite", Namespace: "app", RequireAll: true})
	r.RegisterSet(FeatureSet{Key: "bundle", Namespace: "app"})
	for _, plan := range []PlanID{"no-such-plan", "pro", "enterprise"} {
		if r.SetAvailable("app", plan, "suite", true, true) {
			t.Fatalf("an empty require-all set admitted plan %q", plan)
		}
		if r.SetAvailable("app", plan, "bundle", true, true) {
			t.Fatalf("an empty any-mode set admitted plan %q", plan)
		}
	}
}
