package entitlement

import "testing"

// Decide and DecideSet name the gate that refused, in the order unregistered,
// dark, plan, invite, and answer ReasonNone exactly when Available and
// SetAvailable admit.
func TestDecideNamesTheRefusingGate(t *testing.T) {
	r := seeded()
	cases := []struct {
		name            string
		key             string
		plan            PlanID
		flagOn, invited bool
		want            Reason
	}{
		{"unregistered", "ghost", "pro", true, true, ReasonUnregistered},
		{"dark", "beta_editor", "pro", false, true, ReasonDark},
		{"dark beats plan", "beta_editor", "starter", false, true, ReasonDark},
		{"plan", "recording", "starter", true, true, ReasonPlanGated},
		{"plan beats invite", "recording", "starter", true, false, ReasonPlanGated},
		{"invite", "recording", "pro", true, false, ReasonNotInvited},
		{"available", "recording", "pro", true, true, ReasonNone},
		{"flagless ignores flagOn", "recording", "pro", false, true, ReasonNone},
	}
	for _, c := range cases {
		got := r.Decide("app", c.plan, c.key, c.flagOn, c.invited)
		if got != c.want {
			t.Errorf("%s: Decide = %q, want %q", c.name, got, c.want)
		}
		if r.Available("app", c.plan, c.key, c.flagOn, c.invited) != (c.want == ReasonNone) {
			t.Errorf("%s: Available disagrees with Decide %q", c.name, got)
		}
	}
	r.RegisterSet(FeatureSet{Key: "empty", Namespace: "app", RequireAll: true})
	sets := []struct {
		name            string
		key             string
		plan            PlanID
		flagOn, invited bool
		want            Reason
	}{
		{"unregistered set", "nope", "enterprise", true, true, ReasonUnregistered},
		{"dark set", "studio", "enterprise", false, true, ReasonDark},
		{"require-all lacks a member", "studio", "pro", true, true, ReasonPlanGated},
		{"any-mode lacks every member", "any_tool", "starter", true, true, ReasonPlanGated},
		{"empty set", "empty", "enterprise", true, true, ReasonPlanGated},
		{"set invite", "studio", "enterprise", true, false, ReasonNotInvited},
		{"set available", "any_tool", "pro", true, true, ReasonNone},
	}
	for _, c := range sets {
		got := r.DecideSet("app", c.plan, c.key, c.flagOn, c.invited)
		if got != c.want {
			t.Errorf("%s: DecideSet = %q, want %q", c.name, got, c.want)
		}
		if r.SetAvailable("app", c.plan, c.key, c.flagOn, c.invited) != (c.want == ReasonNone) {
			t.Errorf("%s: SetAvailable disagrees with DecideSet %q", c.name, got)
		}
	}
}
