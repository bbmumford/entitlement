/*
 * Copyright (c) 2026 Brandon Miller-Mumford.
 * Licensed under the MIT License; see LICENSE.
 */

package entitlement

import "testing"

// A set with no Flag is on whatever flagOn says, as a feature with no Flag is: a direct caller that
// passes false for an unflagged surface is not refused.
func TestAFlaglessSetIsOnWhateverFlagOnSays(t *testing.T) {
	r := NewRegistry()
	r.Register(Feature{Key: "f", Namespace: "orbtr", AllowedPlans: []PlanID{"pro"}})
	r.RegisterSet(FeatureSet{Key: "s", Namespace: "orbtr", Members: []string{"f"}})
	if got := r.Decide("orbtr", "pro", "f", false, true); got != ReasonNone {
		t.Fatalf("Decide(flagless feature, false) = %q", got)
	}
	if got := r.DecideSet("orbtr", "pro", "s", false, true); got != ReasonNone {
		t.Fatalf("DecideSet(flagless set, false) = %q, want available", got)
	}
	r.RegisterSet(FeatureSet{Key: "flagged", Namespace: "orbtr", Members: []string{"f"}, Flag: "release"})
	if got := r.DecideSet("orbtr", "pro", "flagged", false, true); got != ReasonDark {
		t.Fatalf("DecideSet(flagged set, false) = %q, want dark", got)
	}
}
