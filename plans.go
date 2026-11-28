/*
 * Copyright (c) 2026 Brandon Miller-Mumford.
 * Licensed under the MIT License; see LICENSE.
 */

package entitlement

import "fmt"

// PlanValidator reports whether a plan id is one of the plans of the product that owns a namespace.
// The registry interprets no plan vocabulary: each product supplies its own validator for its own
// namespace (DeclarePlans), so one registry seeded by several products, as io's composition root seeds
// io and AI rows into one, checks each row against the plans of the product it belongs to rather than
// against a union in which one product's plan id would pass under another's namespace.
type PlanValidator func(PlanID) bool

// DeclarePlans installs valid as ns's plan validator, replacing any earlier one; a nil valid withdraws
// the declaration. A product's seed calls it for its own namespace before registering that namespace's
// rows through CheckPlans. On a nil registry it stores nothing, so every check against it refuses.
func (r *Registry) DeclarePlans(ns Namespace, valid PlanValidator) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if valid == nil {
		delete(r.plans, ns)
		return
	}
	if r.plans == nil {
		r.plans = map[Namespace]PlanValidator{}
	}
	r.plans[ns] = valid
}

// Declared reports whether a product has declared ns's plans. A nil registry declares none.
func (r *Registry) Declared(ns Namespace) bool {
	return r.validator(ns) != nil
}

// CheckPlans refuses a feature whose namespace no product has declared, and one that names a plan its
// namespace's validator refuses: in AllowedPlans, in Limit, or in a parameter's Plans or Limit. It
// stores nothing; a seed that wants rows checked calls it before Register, whose own contract (store
// what it is given) the gates' set-membership reads rely on.
func (r *Registry) CheckPlans(f Feature) error {
	valid := r.validator(f.Namespace)
	if valid == nil {
		return fmt.Errorf("entitlement: feature %q has namespace %q, whose plans no product has declared", f.Key, f.Namespace)
	}
	for _, p := range f.AllowedPlans {
		if !valid(p) {
			return fmt.Errorf("entitlement: feature %q allows plan %q, which namespace %q does not declare", f.Key, p, f.Namespace)
		}
	}
	for p := range f.Limit {
		if !valid(p) {
			return fmt.Errorf("entitlement: feature %q limits plan %q, which namespace %q does not declare", f.Key, p, f.Namespace)
		}
	}
	for name, param := range f.Params {
		for p := range param.Plans {
			if !valid(p) {
				return fmt.Errorf("entitlement: feature %q parameter %q names plan %q, which namespace %q does not declare", f.Key, name, p, f.Namespace)
			}
		}
		for p := range param.Limit {
			if !valid(p) {
				return fmt.Errorf("entitlement: feature %q parameter %q limits plan %q, which namespace %q does not declare", f.Key, name, p, f.Namespace)
			}
		}
	}
	return nil
}

// validator returns ns's declared validator, or nil when none is declared or r is nil.
func (r *Registry) validator(ns Namespace) PlanValidator {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.plans[ns]
}
