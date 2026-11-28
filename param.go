/*
 * Copyright (c) 2026 Brandon Miller-Mumford.
 * Licensed under the MIT License; see LICENSE.
 */

package entitlement

// param.go — a feature key's parameters: one key confers its feature and carries named parameters
// whose values, or count, each plan's row states, so a plan narrows or widens what the key allows by
// its parameters and never by a second key. A gate asking for one value admits only a plan whose row
// lists it, and only when the plan holds the key at all (Allowed); a gate asking for the key alone reads
// no parameter. Every answer fails closed: an unregistered key, an undeclared parameter, an undeclared
// value, a plan the parameter does not list, or a count with no allowance for the plan denies.

import (
	"errors"
	"fmt"
)

// ErrInvalidParam is wrapped by Validate for a parameter declaration a gate could not answer exactly.
var ErrInvalidParam = errors.New("entitlement: invalid feature parameter")

// Validate checks f's parameters, so a seed is refused before it is served: a parameter with no name,
// one that is both a set and a count or neither, a plan's value its parameter does not declare, and a
// plan named by a parameter but absent from AllowedPlans (a parameter never widens the key itself), and
// a count below -1 in the key's Limit or a parameter's (-1 is unlimited, 0 and up a count; anything
// lower is neither, and a gate comparing usage against it would answer as if it were a cap).
// Registering does not call it; the composition root that builds a seed does, before Register.
func (f Feature) Validate() error {
	allowed := make(map[PlanID]bool, len(f.AllowedPlans))
	for _, p := range f.AllowedPlans {
		allowed[p] = true
	}
	for plan, n := range f.Limit {
		if n < -1 {
			return fmt.Errorf("%w: %s gives plan %q the count %d (want -1 for unlimited, or 0 and up)", ErrInvalidParam, f.Key, plan, n)
		}
	}
	for name, p := range f.Params {
		if name == "" {
			return fmt.Errorf("%w: %s has a parameter with no name", ErrInvalidParam, f.Key)
		}
		set, count := len(p.Values) > 0 || len(p.Plans) > 0, len(p.Limit) > 0
		if set == count {
			return fmt.Errorf("%w: %s(%s) must be a set of values or a count, not both or neither", ErrInvalidParam, f.Key, name)
		}
		declared := make(map[string]bool, len(p.Values))
		for _, v := range p.Values {
			if v == "" {
				return fmt.Errorf("%w: %s(%s) declares an empty value", ErrInvalidParam, f.Key, name)
			}
			declared[v] = true
		}
		for plan, vs := range p.Plans {
			if !allowed[plan] {
				return fmt.Errorf("%w: %s(%s) lists plan %q, which does not hold %s", ErrInvalidParam, f.Key, name, plan, f.Key)
			}
			for _, v := range vs {
				if !declared[v] {
					return fmt.Errorf("%w: %s(%s) gives plan %q the undeclared value %q", ErrInvalidParam, f.Key, name, plan, v)
				}
			}
		}
		for plan, n := range p.Limit {
			if !allowed[plan] {
				return fmt.Errorf("%w: %s(%s) counts plan %q, which does not hold %s", ErrInvalidParam, f.Key, name, plan, f.Key)
			}
			if n < -1 {
				return fmt.Errorf("%w: %s(%s) gives plan %q the count %d (want -1 for unlimited, or 0 and up)", ErrInvalidParam, f.Key, name, plan, n)
			}
		}
	}
	return nil
}

// AllowedParam reports whether plan may use value of the key's parameter param: the plan holds the key
// (Allowed), the parameter is a declared set, value is one of its values, and the plan's row lists it.
func (r *Registry) AllowedParam(ns Namespace, plan PlanID, key, param, value string) bool {
	if !r.Allowed(ns, plan, key) {
		return false
	}
	f, _ := r.get(ns, key)
	p, ok := f.Params[param]
	if !ok || value == "" {
		return false
	}
	declared := false
	for _, v := range p.Values {
		declared = declared || v == value
	}
	if !declared {
		return false
	}
	for _, v := range p.Plans[plan] {
		if v == value {
			return true
		}
	}
	return false
}

// ParamLimit returns the plan's count for the key's parameter param and whether one is defined; -1 means
// unlimited. A plan that does not hold the key, an undeclared parameter, a parameter that is a set, or a
// plan with no count gives (0, false), which a caller reads as no allowance.
func (r *Registry) ParamLimit(ns Namespace, plan PlanID, key, param string) (int64, bool) {
	if !r.Allowed(ns, plan, key) {
		return 0, false
	}
	f, _ := r.get(ns, key)
	p, ok := f.Params[param]
	if !ok || p.Limit == nil {
		return 0, false
	}
	v, ok := p.Limit[plan]
	return v, ok
}
