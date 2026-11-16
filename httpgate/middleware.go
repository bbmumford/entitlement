// Package httpgate is the stdlib net/http adapter for the entitlement gate: middleware that fails closed
// when the caller's plan does not include the requested feature. It imports only net/http, context, and
// the entitlement leaf, so any consumer wraps its routes identically. The plan (and the optional
// flag/invite) is resolved through INJECTED seams, not a hard import of a consumer symbol, keeping the
// adapter free of any product dependency.
package httpgate

import (
	"context"
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

// InviteResolver reports whether the caller passes a surface's invite gate. A nil resolver is treated as
// "invited", so a plan-locked-only surface omits it.
type InviteResolver func(ctx context.Context, featureKey string) bool

// Denier renders the 403 body. A consumer may override it (WithDenier) to match its own error envelope;
// the default writes a minimal 403.
type Denier func(w http.ResponseWriter, r *http.Request, ns entitlement.Namespace, key string)

func defaultDenier(w http.ResponseWriter, _ *http.Request, _ entitlement.Namespace, _ string) {
	http.Error(w, "feature not included in your plan", http.StatusForbidden)
}

// Gate wires the entitlement registry to the injected request seams and produces middleware. Build one
// at the composition root and reuse its RequireFeature/RequireSet across routes.
type Gate struct {
	reg     *entitlement.Registry
	extract PlanExtractor
	flag    FlagResolver
	invite  InviteResolver
	denier  Denier
}

// NewGate builds a Gate. reg and extract are required, and a missing one refuses rather than panics:
// a nil registry reports every feature and set unregistered, so each request is denied; a nil extractor
// is replaced by one that never resolves a plan, so each request is denied. That is safe and useless,
// which is what an unwired gate should be. flag/invite/denier are optional: a nil flag resolver darks
// any FLAGGED feature (fail-closed); a nil invite resolver treats every surface as invited; a nil denier,
// including one passed through WithDenier, uses the default 403.
func NewGate(reg *entitlement.Registry, extract PlanExtractor, opts ...Option) *Gate {
	g := &Gate{reg: reg, extract: extract, denier: defaultDenier}
	for _, o := range opts {
		o(g)
	}
	if g.extract == nil {
		g.extract = unresolvedPlan
	}
	if g.denier == nil {
		g.denier = defaultDenier
	}
	return g
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

// WithDenier overrides the 403 renderer.
func WithDenier(d Denier) Option { return func(g *Gate) { g.denier = d } }

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

func (g *Gate) resolveInvite(ctx context.Context, key string) bool {
	if g.invite == nil {
		return true
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
				g.denier(w, r, ns, key)
				return
			}
			f, found := g.reg.Get(ns, key)
			if !found {
				g.denier(w, r, ns, key)
				return
			}
			flagOn := g.resolveFlag(r.Context(), f.Flag)
			invited := g.resolveInvite(r.Context(), key)
			if !g.reg.Available(ns, plan, key, flagOn, invited) {
				g.denier(w, r, ns, key)
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
				g.denier(w, r, ns, named)
				return
			}
			f, found := g.reg.Get(ns, key)
			if !found {
				g.denier(w, r, ns, named)
				return
			}
			flagOn := g.resolveFlag(r.Context(), f.Flag)
			invited := g.resolveInvite(r.Context(), key)
			if !g.reg.Available(ns, plan, key, flagOn, invited) || !g.reg.AllowedParam(ns, plan, key, param, value) {
				g.denier(w, r, ns, named)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireSet returns middleware gating a whole surface by a FeatureSet: the set's release flag, the
// caller's invite and the plan's membership must all admit. An unresolved plan or unregistered set denies.
func (g *Gate) RequireSet(ns entitlement.Namespace, setKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			plan, ok := g.extract(r.Context())
			if !ok {
				g.denier(w, r, ns, setKey)
				return
			}
			set, found := g.reg.GetSet(ns, setKey)
			if !found {
				g.denier(w, r, ns, setKey)
				return
			}
			flagOn := g.resolveFlag(r.Context(), set.Flag)
			invited := g.resolveInvite(r.Context(), setKey)
			if !g.reg.SetAvailable(ns, plan, setKey, flagOn, invited) {
				g.denier(w, r, ns, setKey)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
