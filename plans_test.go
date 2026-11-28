/*
 * Copyright (c) 2026 Brandon Miller-Mumford.
 * Licensed under the MIT License; see LICENSE.
 */

package entitlement

import "testing"

// planSet is a PlanValidator over a fixed list, the shape a product's seed declares.
func planSet(plans ...PlanID) PlanValidator {
	set := map[PlanID]bool{}
	for _, p := range plans {
		set[p] = true
	}
	return func(p PlanID) bool { return set[p] }
}

// TestEachNamespaceIsCheckedAgainstItsOwnPlans seeds two products into one registry, as io's
// composition root does, where one plan id is spelled the same in both: a row passes only against
// the plans its own namespace declares.
func TestEachNamespaceIsCheckedAgainstItsOwnPlans(t *testing.T) {
	r := NewRegistry()
	r.DeclarePlans("devices", planSet("starter", "pro"))
	r.DeclarePlans("app", planSet("free", "pro"))

	if err := r.CheckPlans(Feature{Key: "devices.x", Namespace: "devices", AllowedPlans: []PlanID{"starter"}}); err != nil {
		t.Fatalf("a devices row allowing starter: %v", err)
	}
	if err := r.CheckPlans(Feature{Key: "app.x", Namespace: "app", AllowedPlans: []PlanID{"starter"}}); err == nil {
		t.Fatal("an app row allowing devices' starter passed")
	}
	if err := r.CheckPlans(Feature{Key: "app.y", Namespace: "app", AllowedPlans: []PlanID{"pro"}}); err != nil {
		t.Fatalf("an app row allowing its own pro: %v", err)
	}
	if !r.Declared("app") || r.Declared("other") {
		t.Fatalf("Declared(app)=%v Declared(other)=%v", r.Declared("app"), r.Declared("other"))
	}
}

// TestCheckPlansReadsEveryPlanAFeatureNames covers each place a feature names a plan, and a
// namespace no product declared.
func TestCheckPlansReadsEveryPlanAFeatureNames(t *testing.T) {
	r := NewRegistry()
	r.DeclarePlans("app", planSet("free", "pro"))
	cases := map[string]Feature{
		"undeclared namespace": {Key: "k", Namespace: "nope", AllowedPlans: []PlanID{"pro"}},
		"limit plan":           {Key: "k", Namespace: "app", AllowedPlans: []PlanID{"pro"}, Limit: map[PlanID]int64{"max": 1}},
		"parameter plan": {Key: "k", Namespace: "app", AllowedPlans: []PlanID{"pro"},
			Params: map[string]Param{"action": {Values: []string{"a"}, Plans: map[PlanID][]string{"max": {"a"}}}}},
		"parameter limit": {Key: "k", Namespace: "app", AllowedPlans: []PlanID{"pro"},
			Params: map[string]Param{"seats": {Limit: map[PlanID]int64{"max": 3}}}},
	}
	for label, f := range cases {
		if err := r.CheckPlans(f); err == nil {
			t.Errorf("%s: passed", label)
		}
	}
}

// TestAWithdrawnOrUnbuiltDeclarationRefuses: a nil validator withdraws a namespace's plans, and a
// nil registry declares none, so both refuse every row.
func TestAWithdrawnOrUnbuiltDeclarationRefuses(t *testing.T) {
	f := Feature{Key: "k", Namespace: "app", AllowedPlans: []PlanID{"pro"}}
	r := NewRegistry()
	r.DeclarePlans("app", planSet("pro"))
	r.DeclarePlans("app", nil)
	if err := r.CheckPlans(f); err == nil || r.Declared("app") {
		t.Fatal("a withdrawn declaration still passed")
	}
	var unbuilt *Registry
	unbuilt.DeclarePlans("app", planSet("pro"))
	if err := unbuilt.CheckPlans(f); err == nil || unbuilt.Declared("app") {
		t.Fatal("a nil registry passed a row")
	}
	var zero Registry
	zero.DeclarePlans("app", planSet("pro"))
	if err := zero.CheckPlans(f); err != nil {
		t.Fatalf("a zero Registry value: %v", err)
	}
}
