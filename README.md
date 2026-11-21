# entitlement

A plan-based feature gate. It answers three questions: does this plan include this feature, what is this plan's numeric cap for it, and is this staged surface available (release flag, plan and invite all passing). It is an in-memory registry seeded at boot, with no storage and no vocabulary of its own: namespaces and plan ids are opaque strings each product supplies, and every question fails closed (an unregistered feature is never allowed).

`entitlement/httpgate` turns the registry into `net/http` middleware, with the plan, the release flag and the invite check supplied as functions.

## Use

```sh
go get github.com/bbmumford/entitlement
```

```go
const ns entitlement.Namespace = "app"

reg := entitlement.NewRegistry()
reg.Register(entitlement.Feature{
	Key:          "exports",
	Namespace:    ns,
	AllowedPlans: []entitlement.PlanID{"pro", "team"},
	Limit:        map[entitlement.PlanID]int64{"pro": 10, "team": 100},
})

ok := reg.Allowed(ns, "pro", "exports")     // true
max, _ := reg.Limit(ns, "pro", "exports")   // 10
open := reg.Available(ns, "pro", "exports", flagOn, invited)
```

`DeclarePlans` installs a namespace's plan validator, so `CheckPlans` refuses a feature whose namespace no product declared or that names a plan the validator refuses. Feature sets (`RegisterSet`, `SetAvailable`) and per-parameter rules (`AllowedParam`, `ParamLimit`) follow the same rules.

```go
gate := httpgate.NewGate(reg, entitlement.ExtractPlan, httpgate.WithFlagResolver(flags))
mux.Handle("/exports", gate.RequireFeature(ns, "exports")(exportsHandler))
```

`entitlement.WithPlan` puts the caller's plan on a context for `ExtractPlan` to read.

## Consumers

ORBTR library, ai cortex, xyz and the api.orbtr.ai endpoint, and io constellation.
