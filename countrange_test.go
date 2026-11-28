/*
 * Copyright (c) 2026 Brandon Miller-Mumford.
 * Licensed under the MIT License; see LICENSE.
 */

package entitlement

import (
	"errors"
	"testing"
)

// Validate refuses a count below -1, in the key's Limit or a parameter's: -1 is unlimited and 0 and up
// a count, and nothing lower is either.
func TestValidateRefusesACountBelowMinusOne(t *testing.T) {
	ok := []Feature{
		{Key: "k", AllowedPlans: []PlanID{"pro"}, Limit: map[PlanID]int64{"pro": -1}},
		{Key: "k", AllowedPlans: []PlanID{"pro"}, Limit: map[PlanID]int64{"pro": 0}},
		{Key: "k", AllowedPlans: []PlanID{"pro"}, Params: map[string]Param{"seats": {Limit: map[PlanID]int64{"pro": -1}}}},
	}
	for _, f := range ok {
		if err := f.Validate(); err != nil {
			t.Errorf("Validate(%+v) = %v", f, err)
		}
	}
	bad := []Feature{
		{Key: "k", AllowedPlans: []PlanID{"pro"}, Limit: map[PlanID]int64{"pro": -5}},
		{Key: "k", AllowedPlans: []PlanID{"pro"}, Params: map[string]Param{"seats": {Limit: map[PlanID]int64{"pro": -2}}}},
	}
	for _, f := range bad {
		if err := f.Validate(); !errors.Is(err, ErrInvalidParam) {
			t.Errorf("Validate(%+v) = %v, want ErrInvalidParam", f, err)
		}
	}
}
