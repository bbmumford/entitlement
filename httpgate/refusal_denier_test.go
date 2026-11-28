/*
 * Copyright (c) 2026 Brandon Miller-Mumford.
 * Licensed under the MIT License; see LICENSE.
 */

package httpgate

import (
	"context"
	"net/http"
	"testing"

	"github.com/bbmumford/entitlement"
)

// A RefusalDenier is told why each refusal was made, on every gate shape, and
// is preferred to a Denier; a Denier alone still renders every refusal.
func TestRefusalDenierIsToldTheReason(t *testing.T) {
	reg := refusalReg()
	reg.Register(entitlement.Feature{Key: "assistant", Namespace: "app", AllowedPlans: []entitlement.PlanID{"pro"},
		Params: map[string]entitlement.Param{"action": {Values: []string{"draft", "send"},
			Plans: map[entitlement.PlanID][]string{"pro": {"draft"}}}}})
	reg.RegisterSet(entitlement.FeatureSet{Key: "suite", Namespace: "app", Members: []string{"recording"}, Flag: "suite_flag"})
	var got []Refusal
	record := WithRefusalDenier(func(w http.ResponseWriter, _ *http.Request, refusal Refusal) {
		got = append(got, refusal)
		w.WriteHeader(http.StatusTeapot)
	})
	legacyCalls := 0
	legacy := WithDenier(func(w http.ResponseWriter, _ *http.Request, _ entitlement.Namespace, _ string) {
		legacyCalls++
		w.WriteHeader(http.StatusPaymentRequired)
	})
	g := NewGate(reg, entitlement.ExtractPlan, legacy, record)
	pro := entitlement.WithPlan(context.Background(), "pro")
	starter := entitlement.WithPlan(context.Background(), "starter")

	answer(t, g.RequireFeature("app", "ghost"), pro)
	answer(t, g.RequireFeature("app", "beta"), pro)
	answer(t, g.RequireFeature("app", "recording"), starter)
	answer(t, g.RequireFeature("app", "recording"), context.Background())
	answer(t, g.RequireFeatureParam("app", "assistant", "action", "send"), pro)
	answer(t, g.RequireSet("app", "suite"), pro)
	want := []Refusal{
		{Namespace: "app", Key: "ghost", Reason: entitlement.ReasonUnregistered},
		{Namespace: "app", Key: "beta", Reason: entitlement.ReasonDark},
		{Namespace: "app", Key: "recording", Reason: entitlement.ReasonPlanGated, UpgradeURL: "/upgrade/recording"},
		{Namespace: "app", Key: "recording", Reason: entitlement.ReasonPlanUnresolved},
		{Namespace: "app", Key: "assistant(send)", Reason: entitlement.ReasonPlanGated},
		{Namespace: "app", Key: "suite", Reason: entitlement.ReasonDark},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d refusals %+v, want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("refusal %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
	if legacyCalls != 0 {
		t.Fatalf("the Denier rendered %d refusals while a RefusalDenier was installed", legacyCalls)
	}
	if code, _ := answer(t, g.RequireFeature("app", "recording"), pro); code != http.StatusOK {
		t.Fatalf("an entitled plan was refused: %d", code)
	}

	only := NewGate(reg, entitlement.ExtractPlan, legacy)
	if code, _ := answer(t, only.RequireFeature("app", "beta"), pro); code != http.StatusPaymentRequired || legacyCalls != 1 {
		t.Fatalf("a Denier alone: got %d after %d calls, want it to render", code, legacyCalls)
	}
}
