// Package entitlement is the scope-agnostic feature-entitlement gate. It answers one authorization
// question — "does this plan include this feature?" — orthogonal to any RBAC/permission axis. It is a
// pure in-memory registry seeded at the composition root and read on the hot path; it has no persistence
// and pulls no dependency. It interprets no axis: a feature's Namespace and a PlanID are both
// opaque strings the consumer assigns — a plan may use a feature iff it is in the feature's AllowedPlans
// set (set-membership IS the validity, so an unknown plan is simply absent and denied). Every gate FAILS
// CLOSED: an unregistered feature, an unknown namespace, or an unresolved plan denies.
package entitlement

import "sync"

// Namespace is the opaque scope a feature belongs to (a consumer assigns it — e.g. a product/surface
// name). The registry keys features by (Namespace, Key) so the same key under two namespaces never
// collides; the gate never interprets the namespace's meaning.
type Namespace string

// PlanID is an opaque plan identifier the consumer assigns. Entitlement is set-membership over these
// strings; there is no baked plan vocabulary, so a plan the registry never lists grants nothing.
type PlanID string

// Meta is the display catalog for a Feature — strings an upgrade UI renders. It carries no authorization
// weight (Allowed reads AllowedPlans only).
type Meta struct {
	Name       string `json:"name,omitempty"`
	Desc       string `json:"desc,omitempty"`
	Icon       string `json:"icon,omitempty"`
	UpgradeURL string `json:"upgrade_url,omitempty"`
}

// Feature is one canonical entitlement entry.
//   - AllowedPlans is the BOOLEAN authority: a plan may use the feature iff it is in this set.
//   - Limit is the OPTIONAL numeric entitlement: a per-plan cap, -1 = unlimited, nil = no numeric axis.
//   - Flag is the OPTIONAL release-flag key: empty = always on; non-empty = the surface stays dark until
//     the caller's flag resolver reports it on (Available ANDs it in).
//   - Params are the OPTIONAL named parameters the key carries: each lists, per plan, what the plan may
//     do within the key (AllowedParam, ParamLimit). A gate that asks for the key alone reads none of them.
//   - InviteOnly marks a surface that is plan-locked AND invite-only: a gate with no invite resolver
//     admits no one to it, where an unmarked key is not invite-gated at all.
type Feature struct {
	Key          string           `json:"key"`
	Namespace    Namespace        `json:"namespace"`
	AllowedPlans []PlanID         `json:"allowed_plans"`
	Limit        map[PlanID]int64 `json:"limit,omitempty"`
	Flag         string           `json:"flag,omitempty"`
	Display      Meta             `json:"display,omitempty"`
	Params       map[string]Param `json:"params,omitempty"`
	InviteOnly   bool             `json:"inviteOnly,omitempty"`
}

// Param is one named parameter of a feature key. A parameter is a set or a count, never both:
//   - Values declares the values the parameter admits at all, and Plans lists, per plan, the values that
//     plan allows, each one of Values; a plan absent from Plans allows none.
//   - Limit is a count, per plan, in Feature.Limit's shape: -1 = unlimited, a plan absent = no allowance.
type Param struct {
	Values []string            `json:"values,omitempty"`
	Plans  map[PlanID][]string `json:"plans,omitempty"`
	Limit  map[PlanID]int64    `json:"limit,omitempty"`
}

// FeatureSet is a named bundle of feature keys a surface requires as a unit. RequireAll = satisfied only
// when EVERY member is allowed; otherwise ANY allowed member satisfies it. Flag is the OPTIONAL release
// flag for the whole surface, with the same rule as Feature.Flag: empty = always on; non-empty = dark
// until the caller's flag resolver reports it on. InviteOnly marks the surface invite-only, with
// Feature.InviteOnly's rule.
type FeatureSet struct {
	Key        string    `json:"key"`
	Namespace  Namespace `json:"namespace"`
	Members    []string  `json:"members"`
	RequireAll bool      `json:"require_all"`
	Flag       string    `json:"flag,omitempty"`
	InviteOnly bool      `json:"inviteOnly,omitempty"`
}

// featureKey namespaces the registry by (Namespace, Key).
type featureKey struct {
	ns  Namespace
	key string
}

// Registry is the canonical feature + feature-set registry. One instance is built at the composition
// root, seeded by the owner, then read on the hot path. Concurrency-safe: seeding takes the write lock,
// gate reads take the read lock, so a late Register never races a concurrent Allowed.
//
// A NIL *Registry is an empty one that cannot be seeded: every read reports nothing and every gate in
// gate.go denies, because each resolves through Get or GetSet. The httpgate middleware hands the
// registry it was given straight to those reads on every request, so a composition root that never
// built one must produce a refusal there, not a panic inside the handler chain. A zero
// Registry value is usable as well: its maps are allocated on the first Register.
type Registry struct {
	mu       sync.RWMutex
	features map[featureKey]Feature
	sets     map[featureKey]FeatureSet
	plans    map[Namespace]PlanValidator
}

// NewRegistry returns an empty registry ready to seed.
func NewRegistry() *Registry {
	return &Registry{
		features: map[featureKey]Feature{},
		sets:     map[featureKey]FeatureSet{},
	}
}

// Register adds or replaces a feature. Seeding is the owner's job; call it at boot before serving. On a
// nil registry it stores nothing, so the feature stays unregistered and every gate over it denies.
func (r *Registry) Register(f Feature) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.features == nil {
		r.features = map[featureKey]Feature{}
	}
	r.features[featureKey{f.Namespace, f.Key}] = f.clone()
}

// RegisterSet adds or replaces a feature set. On a nil registry it stores nothing, as Register.
func (r *Registry) RegisterSet(s FeatureSet) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.sets == nil {
		r.sets = map[featureKey]FeatureSet{}
	}
	r.sets[featureKey{s.Namespace, s.Key}] = s.clone()
}

// Get returns a copy of a registered feature and whether it exists. A nil registry holds no feature.
// The copy shares no slice or map with the registry, so a caller that edits AllowedPlans, Limit or
// Params on it changes nothing the gates read.
func (r *Registry) Get(ns Namespace, key string) (Feature, bool) {
	f, ok := r.get(ns, key)
	return f.clone(), ok
}

// GetSet returns a copy of a registered feature set and whether it exists, sharing no slice with the
// registry, as Get. A nil registry holds no set.
func (r *Registry) GetSet(ns Namespace, key string) (FeatureSet, bool) {
	s, ok := r.getSet(ns, key)
	return s.clone(), ok
}

// get is the stored feature itself, for the gates in this package, which only read it: Register
// stores a copy and replaces an entry whole, so nothing outside the registry holds its slices and
// maps, and reading them after the lock is released races no write.
func (r *Registry) get(ns Namespace, key string) (Feature, bool) {
	if r == nil {
		return Feature{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.features[featureKey{ns, key}]
	return f, ok
}

// getSet is the stored feature set itself, read as get is.
func (r *Registry) getSet(ns Namespace, key string) (FeatureSet, bool) {
	if r == nil {
		return FeatureSet{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.sets[featureKey{ns, key}]
	return s, ok
}

// clone is f with every slice and map copied.
func (f Feature) clone() Feature {
	f.AllowedPlans = append([]PlanID(nil), f.AllowedPlans...)
	f.Limit = cloneCounts(f.Limit)
	if f.Params != nil {
		params := make(map[string]Param, len(f.Params))
		for name, p := range f.Params {
			p.Values = append([]string(nil), p.Values...)
			if p.Plans != nil {
				plans := make(map[PlanID][]string, len(p.Plans))
				for plan, vs := range p.Plans {
					plans[plan] = append([]string(nil), vs...)
				}
				p.Plans = plans
			}
			p.Limit = cloneCounts(p.Limit)
			params[name] = p
		}
		f.Params = params
	}
	return f
}

// clone is s with its member list copied.
func (s FeatureSet) clone() FeatureSet {
	s.Members = append([]string(nil), s.Members...)
	return s
}

func cloneCounts(m map[PlanID]int64) map[PlanID]int64 {
	if m == nil {
		return nil
	}
	out := make(map[PlanID]int64, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// Features returns every registered feature (order unspecified) — the read side for a caller that needs
// to enumerate or count the seeded catalog. A nil registry returns none.
func (r *Registry) Features() []Feature {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Feature, 0, len(r.features))
	for _, f := range r.features {
		out = append(out, f.clone())
	}
	return out
}
