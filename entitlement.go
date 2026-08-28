// Package entitlement is the scope-agnostic feature-entitlement gate (extracted from the HSTLES
// entitlement domain). It answers one authorization question — "does this plan include this feature?" —
// orthogonal to any RBAC/permission axis. It is a pure in-memory registry seeded at the composition root
// and read on the hot path; it has no persistence and pulls no dependency. It interprets no axis: a
// feature's Namespace (was a baked io/ai/platform Product) and a PlanID (was a baked plan enum) are both
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
type Feature struct {
	Key          string           `json:"key"`
	Namespace    Namespace        `json:"namespace"`
	AllowedPlans []PlanID         `json:"allowed_plans"`
	Limit        map[PlanID]int64 `json:"limit,omitempty"`
	Flag         string           `json:"flag,omitempty"`
	Display      Meta             `json:"display,omitempty"`
}

// FeatureSet is a named bundle of feature keys a surface requires as a unit. RequireAll = satisfied only
// when EVERY member is allowed; otherwise ANY allowed member satisfies it.
type FeatureSet struct {
	Key        string    `json:"key"`
	Namespace  Namespace `json:"namespace"`
	Members    []string  `json:"members"`
	RequireAll bool      `json:"require_all"`
}

// featureKey namespaces the registry by (Namespace, Key).
type featureKey struct {
	ns  Namespace
	key string
}

// Registry is the canonical feature + feature-set registry. One instance is built at the composition
// root, seeded by the owner, then read on the hot path. Concurrency-safe: seeding takes the write lock,
// gate reads take the read lock, so a late Register never races a concurrent Allowed.
type Registry struct {
	mu       sync.RWMutex
	features map[featureKey]Feature
	sets     map[featureKey]FeatureSet
}

// NewRegistry returns an empty registry ready to seed.
func NewRegistry() *Registry {
	return &Registry{
		features: map[featureKey]Feature{},
		sets:     map[featureKey]FeatureSet{},
	}
}

// Register adds or replaces a feature. Seeding is the owner's job; call it at boot before serving.
func (r *Registry) Register(f Feature) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.features[featureKey{f.Namespace, f.Key}] = f
}

// RegisterSet adds or replaces a feature set.
func (r *Registry) RegisterSet(s FeatureSet) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sets[featureKey{s.Namespace, s.Key}] = s
}

// Get returns a registered feature and whether it exists.
func (r *Registry) Get(ns Namespace, key string) (Feature, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.features[featureKey{ns, key}]
	return f, ok
}

// GetSet returns a registered feature set and whether it exists.
func (r *Registry) GetSet(ns Namespace, key string) (FeatureSet, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.sets[featureKey{ns, key}]
	return s, ok
}
