package entitlement

import "testing"

// DecideSetFlags resolves the set's flag and every member's through one resolver: a member released by
// it is conferred, one it keeps dark is not, and a nil resolver darks every flagged set and member.
func TestDecideSetFlagsResolvesEveryMembersFlag(t *testing.T) {
	r := NewRegistry()
	r.Register(Feature{Key: "beta", Namespace: "orbtr", AllowedPlans: []PlanID{"pro"}, Flag: "beta_release"})
	r.Register(Feature{Key: "base", Namespace: "orbtr", AllowedPlans: []PlanID{"pro"}})
	r.RegisterSet(FeatureSet{Key: "all", Namespace: "orbtr", Members: []string{"base", "beta"}, RequireAll: true})
	r.RegisterSet(FeatureSet{Key: "dark_only", Namespace: "orbtr", Members: []string{"beta"}})
	r.RegisterSet(FeatureSet{Key: "staged", Namespace: "orbtr", Members: []string{"base"}, Flag: "staged"})
	on := func(string) bool { return true }
	off := func(flag string) bool { return flag != "beta_release" }
	for _, c := range []struct {
		set    string
		flagOn func(string) bool
		plan   PlanID
		want   Reason
	}{
		{"all", on, "pro", ReasonNone},
		{"all", off, "pro", ReasonDark},
		{"all", nil, "pro", ReasonDark},
		{"dark_only", off, "pro", ReasonDark},
		{"dark_only", off, "starter", ReasonPlanGated},
		{"dark_only", on, "pro", ReasonNone},
		{"staged", nil, "pro", ReasonDark},
		{"staged", on, "pro", ReasonNone},
	} {
		if got := r.DecideSetFlags("orbtr", c.plan, c.set, c.flagOn, true); got != c.want {
			t.Errorf("DecideSetFlags(%s, %s) = %q, want %q", c.set, c.plan, got, c.want)
		}
	}
}
