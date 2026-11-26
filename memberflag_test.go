package entitlement

import "testing"

// A set never confers a member whose own release flag is off. With a bool flagOn only the set's own
// flag (and a member flag that is the same key) is answered; every other flagged member is dark.
func TestASetNeverConfersADarkMember(t *testing.T) {
	r := NewRegistry()
	r.Register(Feature{Key: "beta", Namespace: "orbtr", AllowedPlans: []PlanID{"pro"}, Flag: "beta_release"})
	r.Register(Feature{Key: "base", Namespace: "orbtr", AllowedPlans: []PlanID{"pro"}})
	r.RegisterSet(FeatureSet{Key: "bundle", Namespace: "orbtr", Members: []string{"beta"}})
	r.RegisterSet(FeatureSet{Key: "all", Namespace: "orbtr", Members: []string{"base", "beta"}, RequireAll: true})
	r.RegisterSet(FeatureSet{Key: "either", Namespace: "orbtr", Members: []string{"base", "beta"}})
	r.RegisterSet(FeatureSet{Key: "shared", Namespace: "orbtr", Members: []string{"beta"}, Flag: "beta_release"})
	for _, c := range []struct {
		set    string
		flagOn bool
		want   Reason
	}{
		{"bundle", true, ReasonDark},
		{"all", true, ReasonDark},
		{"either", true, ReasonNone},
		{"shared", true, ReasonNone},
		{"shared", false, ReasonDark},
	} {
		if got := r.DecideSet("orbtr", "pro", c.set, c.flagOn, true); got != c.want {
			t.Errorf("DecideSet(%s, flagOn=%v) = %q, want %q", c.set, c.flagOn, got, c.want)
		}
		if r.SetAvailable("orbtr", "pro", c.set, c.flagOn, true) != (c.want == ReasonNone) {
			t.Errorf("SetAvailable(%s) disagrees with DecideSet", c.set)
		}
	}
}
