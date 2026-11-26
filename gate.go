package entitlement

// gate.go — the fail-closed entitlement resolver. Two orthogonal axes, never conflated: Allowed answers
// "does the plan include this feature?" (boolean, set-membership); Limit answers "what is the plan's
// numeric cap?"; Available composes the full access decision flag ∧ plan ∧ invite for a staged,
// invite-gateable surface. EVERY gate FAILS CLOSED — an unregistered feature or an unlisted plan denies,
// so a misconfiguration becomes a lockout that surfaces the mistake, never a silent leak. Each gate reads
// the registry only through Get or GetSet, which answer "absent" on a nil registry, so an unbuilt
// registry denies every question here rather than panicking.

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
	return r.Decide(ns, plan, key, flagOn, invited) == ReasonNone
}

// Reason names why a gate refused. A gate's refusals are distinguishable by it, so a caller can render
// each its own way: a plan that lacks a feature sells an upgrade, a dark feature stays hidden, a missing
// invite reassures, an unresolved plan is a fault of the caller's session.
type Reason string

const (
	// ReasonNone is no refusal: the feature or set is available.
	ReasonNone Reason = ""
	// ReasonUnregistered: no feature or set is registered under the key.
	ReasonUnregistered Reason = "unregistered"
	// ReasonDark: the key is registered but its release flag is not on; it exists and is unreleased.
	ReasonDark Reason = "dark"
	// ReasonPlanGated: the plan does not include the feature, or not every (any) member of the set.
	ReasonPlanGated Reason = "plan_gated"
	// ReasonNotInvited: the caller has not passed the surface's invite gate.
	ReasonNotInvited Reason = "not_invited"
	// ReasonPlanUnresolved: no plan was resolved for the caller. The registry never answers it (it is
	// handed a plan); a gate that resolves the plan itself answers it before asking the registry.
	ReasonPlanUnresolved Reason = "plan_unresolved"
)

// Decide is Available with the reason for a refusal: ReasonNone when the feature is available, else the
// first gate that refused, in the order unregistered, dark (flag), plan, invite. Checking the flag before
// the plan keeps a dark feature hidden from a plan that lacks it as well.
func (r *Registry) Decide(ns Namespace, plan PlanID, key string, flagOn, invited bool) Reason {
	f, ok := r.Get(ns, key)
	if !ok {
		return ReasonUnregistered
	}
	if f.Flag != "" && !flagOn {
		return ReasonDark // flag gate: dark until the release flag resolves on
	}
	if !r.Allowed(ns, plan, key) {
		return ReasonPlanGated // plan gate
	}
	if !invited {
		return ReasonNotInvited // invite gate (caller-resolved; true when the surface isn't invite-gated)
	}
	return ReasonNone
}

// SetAvailable resolves a FeatureSet: a whole surface unlocks as a unit. flagOn/invited apply to the SET
// as a surface (the caller resolves them; flagOn answers the set's Flag). Member entitlement is per-member Allowed: RequireAll => every
// member must be allowed; otherwise ANY allowed member suffices. Fail-closed: an unregistered set, a
// false flag/invite, a set with no members, a RequireAll set with any denied member, or an any-mode set
// with no allowed member => false. A set with no members entitles no plan in either mode: "every member
// allowed" holds vacuously for an empty RequireAll set, which would otherwise open it to every plan. A
// set with no Flag is on whatever flagOn says, as a feature with no Flag is, so a mis-passed false
// cannot dark it.
func (r *Registry) SetAvailable(ns Namespace, plan PlanID, setKey string, flagOn, invited bool) bool {
	return r.DecideSet(ns, plan, setKey, flagOn, invited) == ReasonNone
}

// DecideSet is SetAvailable with the reason for a refusal, in Decide's order: unregistered, dark (the
// set's flag), plan (the members), invite.
func (r *Registry) DecideSet(ns Namespace, plan PlanID, setKey string, flagOn, invited bool) Reason {
	s, ok := r.GetSet(ns, setKey)
	if !ok {
		return ReasonUnregistered
	}
	if s.Flag != "" && !flagOn {
		return ReasonDark // flag gate: a set with no flag is on, as a feature with no flag is
	}
	if len(s.Members) == 0 {
		return ReasonPlanGated
	}
	anyAllowed := false
	for _, m := range s.Members {
		allowed := r.Allowed(ns, plan, m)
		if s.RequireAll && !allowed {
			return ReasonPlanGated // one denied member fails an all-required set
		}
		if allowed {
			anyAllowed = true
		}
	}
	if !s.RequireAll && !anyAllowed {
		return ReasonPlanGated
	}
	if !invited {
		return ReasonNotInvited
	}
	return ReasonNone
}
