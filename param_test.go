package entitlement

import (
	"errors"
	"testing"
)

func paramReg(t *testing.T) *Registry {
	t.Helper()
	f := Feature{Key: "assistant", Namespace: "app", AllowedPlans: []PlanID{"free", "pro"},
		Params: map[string]Param{
			"action": {Values: []string{"refer", "technician"}, Plans: map[PlanID][]string{"pro": {"refer", "technician"}}},
			"seats":  {Limit: map[PlanID]int64{"free": 2, "pro": -1}},
		}}
	if err := f.Validate(); err != nil {
		t.Fatalf("a well-formed parameter declaration is refused: %v", err)
	}
	r := NewRegistry()
	r.Register(f)
	return r
}

func TestAllowedParam(t *testing.T) {
	r := paramReg(t)
	for _, c := range []struct {
		plan         PlanID
		param, value string
		want         bool
	}{
		{"pro", "action", "refer", true},
		{"pro", "action", "technician", true},
		{"free", "action", "refer", false},   // the plan holds the key but its row lists no value
		{"pro", "action", "personas", false}, // a value the parameter does not declare
		{"pro", "mode", "refer", false},      // a parameter the key does not declare
		{"pro", "action", "", false},         // no value asks for nothing
		{"apex", "action", "refer", false},   // a plan that does not hold the key
		{"pro", "seats", "refer", false},     // a count is not a set
	} {
		if got := r.AllowedParam("app", c.plan, "assistant", c.param, c.value); got != c.want {
			t.Errorf("AllowedParam(%s, %s, %s) = %v, want %v", c.plan, c.param, c.value, got, c.want)
		}
	}
	if !r.Allowed("app", "free", "assistant") {
		t.Error("the key alone must admit every plan holding it, whatever its parameters")
	}
	if r.AllowedParam("other", "pro", "assistant", "action", "refer") || (*Registry)(nil).AllowedParam("app", "pro", "assistant", "action", "refer") {
		t.Error("an unknown namespace or a nil registry must deny")
	}
}

func TestParamLimit(t *testing.T) {
	r := paramReg(t)
	if v, ok := r.ParamLimit("app", "free", "assistant", "seats"); !ok || v != 2 {
		t.Errorf("free seats = %d, %v; want 2", v, ok)
	}
	if v, ok := r.ParamLimit("app", "pro", "assistant", "seats"); !ok || v != -1 {
		t.Errorf("pro seats = %d, %v; want unlimited", v, ok)
	}
	for _, c := range []struct {
		plan  PlanID
		param string
	}{{"apex", "seats"}, {"pro", "action"}, {"pro", "missing"}} {
		if _, ok := r.ParamLimit("app", c.plan, "assistant", c.param); ok {
			t.Errorf("ParamLimit(%s, %s) must give no allowance", c.plan, c.param)
		}
	}
}

func TestValidateRefusesWhatAGateCouldNotAnswer(t *testing.T) {
	base := func(p Param) Feature {
		return Feature{Key: "k", Namespace: "app", AllowedPlans: []PlanID{"pro"}, Params: map[string]Param{"p": p}}
	}
	for name, f := range map[string]Feature{
		"no name":               {Key: "k", AllowedPlans: []PlanID{"pro"}, Params: map[string]Param{"": {Limit: map[PlanID]int64{"pro": 1}}}},
		"neither set nor count": base(Param{}),
		"both set and count":    base(Param{Values: []string{"a"}, Limit: map[PlanID]int64{"pro": 1}}),
		"undeclared value":      base(Param{Values: []string{"a"}, Plans: map[PlanID][]string{"pro": {"b"}}}),
		"empty value":           base(Param{Values: []string{""}}),
		"set widens the key":    base(Param{Values: []string{"a"}, Plans: map[PlanID][]string{"free": {"a"}}}),
		"count widens the key":  base(Param{Limit: map[PlanID]int64{"free": 1}}),
	} {
		if err := f.Validate(); !errors.Is(err, ErrInvalidParam) {
			t.Errorf("%s: Validate = %v, want ErrInvalidParam", name, err)
		}
	}
	if err := (Feature{Key: "k", AllowedPlans: []PlanID{"pro"}}).Validate(); err != nil {
		t.Errorf("a key with no parameters is valid: %v", err)
	}
}
