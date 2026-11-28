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

// TestNilRegistryRefuses: a gate built over a registry that was never
// constructed must answer 403 on both middlewares, never panic in the chain.
func TestNilRegistryRefuses(t *testing.T) {
	g := NewGate(nil, entitlement.ExtractPlan)
	pro := entitlement.WithPlan(context.Background(), "pro")
	if code := serve(t, g.RequireFeature("app", "recording"), pro); code != http.StatusForbidden {
		t.Fatalf("RequireFeature over a nil registry must be 403; got %d", code)
	}
	if code := serve(t, g.RequireSet("app", "open_surface"), pro); code != http.StatusForbidden {
		t.Fatalf("RequireSet over a nil registry must be 403; got %d", code)
	}
}

// TestNilExtractorRefuses: with no plan extractor there is no resolved plan,
// which every gate treats as deny.
func TestNilExtractorRefuses(t *testing.T) {
	g := NewGate(setReg(), nil)
	pro := entitlement.WithPlan(context.Background(), "pro")
	if code := serve(t, g.RequireFeature("app", "recording"), pro); code != http.StatusForbidden {
		t.Fatalf("RequireFeature with a nil extractor must be 403; got %d", code)
	}
	if code := serve(t, g.RequireSet("app", "open_surface"), pro); code != http.StatusForbidden {
		t.Fatalf("RequireSet with a nil extractor must be 403; got %d", code)
	}
}

// TestNilDenierRendersTheDefault: WithDenier(nil) leaves the default 403 in
// place rather than a nil renderer the first denial would call.
func TestNilDenierRendersTheDefault(t *testing.T) {
	g := NewGate(setReg(), entitlement.ExtractPlan, WithDenier(nil))
	if code := serve(t, g.RequireFeature("app", "recording"), context.Background()); code != http.StatusForbidden {
		t.Fatalf("a nil denier must fall back to the default 403; got %d", code)
	}
}
