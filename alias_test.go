package entitlement

import "testing"

// A feature or set handed to Register, or read back through Get, GetSet or Features, shares no slice
// or map with the registry: editing one changes nothing a gate reads.
func TestTheRegistrySharesNoSliceOrMapWithItsCallers(t *testing.T) {
	r := NewRegistry()
	f := Feature{Key: "k", Namespace: "app", AllowedPlans: []PlanID{"pro"}, Limit: map[PlanID]int64{"pro": 5},
		Params: map[string]Param{"mode": {Values: []string{"a"}, Plans: map[PlanID][]string{"pro": {"a"}}}}}
	r.Register(f)
	r.RegisterSet(FeatureSet{Key: "s", Namespace: "app", Members: []string{"k"}})
	f.AllowedPlans[0], f.Limit["pro"], f.Params["mode"].Plans["pro"][0] = "starter", 1, "b"

	got, _ := r.Get("app", "k")
	got.AllowedPlans[0], got.Limit["pro"] = "starter", 1
	got.Params["mode"].Plans["pro"][0] = "b"
	for _, listed := range r.Features() {
		listed.AllowedPlans[0] = "starter"
	}
	set, _ := r.GetSet("app", "s")
	set.Members[0] = "ghost"

	if !r.Allowed("app", "pro", "k") || r.Allowed("app", "starter", "k") {
		t.Fatal("an edit outside the registry changed AllowedPlans")
	}
	if n, ok := r.Limit("app", "pro", "k"); !ok || n != 5 {
		t.Fatalf("an edit outside the registry changed Limit: %d, %v", n, ok)
	}
	if !r.AllowedParam("app", "pro", "k", "mode", "a") {
		t.Fatal("an edit outside the registry changed a parameter's plans")
	}
	if !r.SetAvailable("app", "pro", "s", true, true) {
		t.Fatal("an edit outside the registry changed a set's members")
	}
}
