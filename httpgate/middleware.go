/*
 * Copyright (c) 2026 Brandon Miller-Mumford.
 * Licensed under the MIT License; see LICENSE.
 */

// Package httpgate is the stdlib net/http adapter for the entitlement gate: middleware that fails closed
// when the caller's plan does not include the requested feature. It imports only net/http, context, and
// the entitlement leaf, so any consumer wraps its routes identically. The plan (and the optional
// flag/invite) is resolved through INJECTED seams, not a hard import of a consumer symbol, keeping the
// adapter free of any product dependency.
package httpgate

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/bbmumford/entitlement"
)

// PlanExtractor resolves the caller's opaque plan from the request context. The bool is false when the
// plan is UNRESOLVED, which every gate treats as deny (fail-closed): a request reaching a gated route
// without a resolved plan is a misconfiguration, never an anonymous allow. The entitlement leaf's
// ExtractPlan satisfies this directly.
type PlanExtractor func(context.Context) (entitlement.PlanID, bool)

// FlagResolver reports whether a feature's release flag is on for the caller. A flagLESS feature (empty
// Feature.Flag) is always on and the resolver is not consulted; a FLAGGED feature with a nil resolver
// stays DARK (fail-closed) — a dark feature must not become reachable because the flag seam was unwired.
type FlagResolver func(ctx context.Context, flagKey string) bool

// InviteResolver reports whether the caller passes a surface's invite gate. With a nil resolver a
// feature or set not marked InviteOnly is not invite-gated, so a plan-locked-only surface omits it,
// and one marked InviteOnly admits nobody: an invite-only surface whose resolver was never wired
// fails closed rather than opening to every entitled plan.
type InviteResolver func(ctx context.Context, featureKey string) bool

// Denier renders a refusal knowing only what was refused. A consumer installs one with WithDenier to
// match its own error envelope; a RefusalDenier, which is also told why, takes precedence.
type Denier func(w http.ResponseWriter, r *http.Request, ns entitlement.Namespace, key string)

// Refusal is one refusal of a gate: what was refused and why. Key is the feature or set key, or for
// RequireFeatureParam the key with its value, key(value). UpgradeURL is the refused feature's
// Display.UpgradeURL, when the refusal is ReasonPlanGated and the feature names one.
type Refusal struct {
	Namespace  entitlement.Namespace
	Key        string
	Reason     entitlement.Reason
	UpgradeURL string
}

// RefusalDenier renders a refusal with its reason (WithRefusalDenier).
type RefusalDenier func(w http.ResponseWriter, r *http.Request, refusal Refusal)

// DefaultRefusalDenier is the gate's renderer when the consumer installs none: each reason has its own
// response, so a client can tell them apart. A dark feature answers 404 not_found, hidden as if absent;
// a plan that lacks it answers 403 feature_required with the feature and its upgrade URL; a missing
// invite answers 403 not_invited; an unresolved plan answers 403 plan_unresolved; an unregistered key
// answers 403 not_registered. Bodies are JSON objects whose error field carries the code.
func DefaultRefusalDenier(w http.ResponseWriter, _ *http.Request, refusal Refusal) {
	status, body := http.StatusForbidden, map[string]string{}
	switch refusal.Reason {
	case entitlement.ReasonDark:
		status, body["error"] = http.StatusNotFound, "not_found"
	case entitlement.ReasonPlanGated:
		body["error"], body["feature"] = "feature_required", refusal.Key
		if refusal.UpgradeURL != "" {
			body["upgradeUrl"] = refusal.UpgradeURL
		}
	case entitlement.ReasonNotInvited:
		body["error"] = "not_invited"
	case entitlement.ReasonPlanUnresolved:
		body["error"] = "plan_unresolved"
	default:
		body["error"] = "not_registered"
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// Gate wires the entitlement registry to the injected request seams and produces middleware. Build one
// at the composition root and reuse its RequireFeature/RequireSet across routes.
type Gate struct {
	reg     *entitlement.Registry
	extract PlanExtractor
	flag    FlagResolver
	invite  InviteResolver
	denier  Denier
	refuse  RefusalDenier
}

// NewGate builds a Gate. reg and extract are required, and a missing one refuses rather than panics:
// a nil registry reports every feature and set unregistered, so each request is denied; a nil extractor
// is replaced by one that never resolves a plan, so each request is denied. That is safe and useless,
// which is what an unwired gate should be. flag/invite/denier are optional: a nil flag resolver darks
// any FLAGGED feature (fail-closed); a nil invite resolver admits no one to an InviteOnly feature or set
// (fail-closed) and gates no other on invites; with neither
// a Denier nor a RefusalDenier (a nil one passed counts as none), DefaultRefusalDenier renders.
func NewGate(reg *entitlement.Registry, extract PlanExtractor, opts ...Option) *Gate {
	g := &Gate{reg: reg, extract: extract}
	for _, o := range opts {
		o(g)
	}
	if g.extract == nil {
		g.extract = unresolvedPlan
	}
	if g.refuse == nil && g.denier == nil {
		g.refuse = DefaultRefusalDenier
	}
	return g
}

// deny renders one refusal: through the RefusalDenier when one is installed (the default included),
// else through the consumer's Denier, which is told what was refused but not why.
func (g *Gate) deny(w http.ResponseWriter, r *http.Request, refusal Refusal) {
	if g.refuse != nil {
		g.refuse(w, r, refusal)
		return
	}
	g.denier(w, r, refusal.Namespace, refusal.Key)
}

// planGated builds the refusal of a plan that lacks the feature key, carrying the feature's upgrade URL.
func (g *Gate) planGated(ns entitlement.Namespace, named, key string) Refusal {
	f, _ := g.reg.Get(ns, key)
	return Refusal{Namespace: ns, Key: named, Reason: entitlement.ReasonPlanGated, UpgradeURL: f.Display.UpgradeURL}
}

// unresolvedPlan stands in for a missing PlanExtractor. RequireFeature and RequireSet call the extractor
// first on every request, so it must be callable, and its answer must be the unresolved case they deny.
func unresolvedPlan(context.Context) (entitlement.PlanID, bool) { return "", false }

// Option configures a Gate.
type Option func(*Gate)

// WithFlagResolver injects the release-flag seam.
func WithFlagResolver(f FlagResolver) Option { return func(g *Gate) { g.flag = f } }

// WithInviteResolver injects the invite-gate seam.
func WithInviteResolver(i InviteResolver) Option { return func(g *Gate) { g.invite = i } }

// WithDenier installs a renderer that is told what was refused but not why. It renders only when no
// RefusalDenier is installed.
func WithDenier(d Denier) Option { return func(g *Gate) { g.denier = d } }

// WithRefusalDenier installs a renderer that is told what was refused and why; it takes precedence over
// a Denier. A nil one leaves the choice as if it were not given.
func WithRefusalDenier(d RefusalDenier) Option { return func(g *Gate) { g.refuse = d } }

// resolveFlag reports the flag decision for a feature's or a set's release flag: an empty flag is always
// on; a flag with no resolver stays dark.
func (g *Gate) resolveFlag(ctx context.Context, flag string) bool {
	if flag == "" {
		return true
	}
	if g.flag == nil {
		return false // flagged surface, unwired resolver → dark (fail-closed)
	}
	return g.flag(ctx, flag)
}

// resolveInvite reports the invite decision for a feature or set key: the InviteResolver's answer when
// one is wired, else invited exactly when the record is not InviteOnly.
func (g *Gate) resolveInvite(ctx context.Context, key string, inviteOnly bool) bool {
	if g.invite == nil {
		return !inviteOnly
	}
	return g.invite(ctx, key)
}

// RequireFeature returns middleware that admits a request only when the caller's plan makes the feature
// Available (flag ∧ plan ∧ invite). An unresolved plan or unregistered feature denies. The signature is
// the stdlib func(http.Handler) http.Handler that chi's Router.With accepts directly.
func (g *Gate) RequireFeature(ns entitlement.Namespace, key string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			plan, ok := g.extract(r.Context())
			if !ok {
				g.deny(w, r, Refusal{Namespace: ns, Key: key, Reason: entitlement.ReasonPlanUnresolved})
				return
			}
			f, found := g.reg.Get(ns, key)
			if !found {
				g.deny(w, r, Refusal{Namespace: ns, Key: key, Reason: entitlement.ReasonUnregistered})
				return
			}
			flagOn := g.resolveFlag(r.Context(), f.Flag)
			invited := g.resolveInvite(r.Context(), key, f.InviteOnly)
			switch reason := g.reg.Decide(ns, plan, key, flagOn, invited); reason {
			case entitlement.ReasonNone:
			case entitlement.ReasonPlanGated:
				g.deny(w, r, g.planGated(ns, key, key))
				return
			default:
				g.deny(w, r, Refusal{Namespace: ns, Key: key, Reason: reason})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireFeatureParam returns middleware that admits a request only when the feature is Available to the
// caller's plan, as RequireFeature, and the plan's row lists value for the key's parameter param
// (Registry.AllowedParam). The refusal names the key with its value, key(value), the spelling a plan
// row's parameter takes. An unresolved plan, an unregistered feature or an undeclared value denies.
func (g *Gate) RequireFeatureParam(ns entitlement.Namespace, key, param, value string) func(http.Handler) http.Handler {
	named := key + "(" + value + ")"
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			plan, ok := g.extract(r.Context())
			if !ok {
				g.deny(w, r, Refusal{Namespace: ns, Key: named, Reason: entitlement.ReasonPlanUnresolved})
				return
			}
			f, found := g.reg.Get(ns, key)
			if !found {
				g.deny(w, r, Refusal{Namespace: ns, Key: named, Reason: entitlement.ReasonUnregistered})
				return
			}
			flagOn := g.resolveFlag(r.Context(), f.Flag)
			invited := g.resolveInvite(r.Context(), key, f.InviteOnly)
			reason := g.reg.Decide(ns, plan, key, flagOn, invited)
			if reason == entitlement.ReasonNone && !g.reg.AllowedParam(ns, plan, key, param, value) {
				reason = entitlement.ReasonPlanGated // the plan's row lacks this value of the parameter
			}
			switch reason {
			case entitlement.ReasonNone:
			case entitlement.ReasonPlanGated:
				g.deny(w, r, g.planGated(ns, named, key))
				return
			default:
				g.deny(w, r, Refusal{Namespace: ns, Key: named, Reason: reason})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireSet returns middleware gating a whole surface by a FeatureSet: the set's release flag, each
// member's release flag, the caller's invite and the plan's membership must all admit
// (Registry.DecideSetFlags, every flag through the Gate's FlagResolver). An unresolved plan or
// unregistered set denies.
func (g *Gate) RequireSet(ns entitlement.Namespace, setKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			plan, ok := g.extract(r.Context())
			if !ok {
				g.deny(w, r, Refusal{Namespace: ns, Key: setKey, Reason: entitlement.ReasonPlanUnresolved})
				return
			}
			set, found := g.reg.GetSet(ns, setKey)
			if !found {
				g.deny(w, r, Refusal{Namespace: ns, Key: setKey, Reason: entitlement.ReasonUnregistered})
				return
			}
			flagOn := func(flag string) bool { return g.resolveFlag(r.Context(), flag) }
			invited := g.resolveInvite(r.Context(), setKey, set.InviteOnly)
			if reason := g.reg.DecideSetFlags(ns, plan, setKey, flagOn, invited); reason != entitlement.ReasonNone {
				g.deny(w, r, Refusal{Namespace: ns, Key: setKey, Reason: reason})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
