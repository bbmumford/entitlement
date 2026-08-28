package entitlement

// gate.go — the fail-closed entitlement resolver. Two orthogonal axes, never conflated: Allowed answers
// "does the plan include this feature?" (boolean, set-membership); Limit answers "what is the plan's
// numeric cap?"; Available composes the full access decision flag ∧ plan ∧ invite for a staged,
// invite-gateable surface. EVERY gate FAILS CLOSED — an unregistered feature or an unlisted plan denies,
// so a misconfiguration becomes a lockout that surfaces the mistake, never a silent leak.

// Allowed reports whether plan is entitled to the feature — the boolean gate. Fail-closed: an
// unregistered feature or a plan absent from AllowedPlans denies.
func (r *Registry) Allowed(ns Namespace, plan PlanID, key string) bool {
	f, ok := r.Get(ns, key)
	if !ok {
		return false // unregistered key denies (never fail-open)
	}
	for _, p := range f.AllowedPlans {
		if p == plan {
			return true
		}
	}
	return false
}

// Limit returns the plan's numeric cap for the feature and whether one is defined. -1 means unlimited.
// Fail-closed: an unregistered feature, a feature with no numeric axis, or no cap for this plan =>
// (0, false), which a caller reads as "no allowance" (deny) rather than "unlimited".
func (r *Registry) Limit(ns Namespace, plan PlanID, key string) (int64, bool) {
	f, ok := r.Get(ns, key)
	if !ok || f.Limit == nil {
		return 0, false
	}
	v, ok := f.Limit[plan]
	if !ok {
		return 0, false
	}
	return v, true
}

// Available is the full access decision: the feature is available iff its release flag is on AND the
// plan is entitled AND the caller passes the invite gate — Available = flag ∧ plan ∧ invite.
//   - flagOn: the caller resolves the release flag; a feature with an empty Flag is always on (this func
//     treats an empty Flag as on regardless, so a mis-passed false on a flagless feature can't dark it).
//   - invited: the caller resolves the surface's invite requirement; an un-invite-gated surface passes true.
//
// Fail-closed: an unregistered feature => false before any AND.
func (r *Registry) Available(ns Namespace, plan PlanID, key string, flagOn, invited bool) bool {
	f, ok := r.Get(ns, key)
	if !ok {
		return false
	}
	if f.Flag != "" && !flagOn {
		return false // flag gate: dark until the release flag resolves on
	}
	if !r.Allowed(ns, plan, key) {
		return false // plan gate
	}
	return invited // invite gate (caller-resolved; true when the surface isn't invite-gated)
}

// SetAvailable resolves a FeatureSet: a whole surface unlocks as a unit. flagOn/invited apply to the SET
// as a surface (the caller resolves them). Member entitlement is per-member Allowed: RequireAll => every
// member must be allowed; otherwise ANY allowed member suffices. Fail-closed: an unregistered set, a
// false flag/invite, a RequireAll set with any denied member, or an any-mode set with no allowed member
// => false.
func (r *Registry) SetAvailable(ns Namespace, plan PlanID, setKey string, flagOn, invited bool) bool {
	s, ok := r.GetSet(ns, setKey)
	if !ok {
		return false
	}
	if !invited || !flagOn {
		return false
	}
	anyAllowed := false
	for _, m := range s.Members {
		allowed := r.Allowed(ns, plan, m)
		if s.RequireAll && !allowed {
			return false // one denied member fails an all-required set
		}
		if allowed {
			anyAllowed = true
		}
	}
	if s.RequireAll {
		return true // reached here => every member allowed
	}
	return anyAllowed
}
