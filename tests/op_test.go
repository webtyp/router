package router_test

import (
	"strings"
	"testing"

	"webtyp.com/model"
	"webtyp.com/router"
)

func TestOperation_Valid(t *testing.T) {
	valid := router.Operation{Module: "m", Entity: "e", Action: model.Create}
	if !valid.Valid() {
		t.Fatalf("expected %+v to be valid", valid)
	}

	invalidCases := []router.Operation{
		{Module: "", Entity: "e", Action: model.Create},
		{Module: "m", Entity: "", Action: model.Create},
		{Module: "m", Entity: "e", Action: 0},
		{Module: "m", Entity: "e", Action: model.Create | model.Read},
	}
	for _, tc := range invalidCases {
		if tc.Valid() {
			t.Fatalf("expected %+v to be invalid", tc)
		}
	}
}

func TestOperation_StringRendersModuleEntityVerb(t *testing.T) {
	op := router.Operation{Module: "m", Entity: "e", Action: model.Create}
	if got := op.String(); got != "m.e.create" {
		t.Fatalf("got %q, want %q", got, "m.e.create")
	}

	invalidOp := router.Operation{Module: "m", Entity: "e", Action: model.Create | model.Read}
	if got := invalidOp.String(); got != "m.e.?" {
		t.Fatalf("got %q, want %q", got, "m.e.?")
	}
}

func TestParseOperation_RoundTrips(t *testing.T) {
	for _, action := range []model.Action{model.Create, model.Read, model.Update, model.Delete} {
		op := router.Operation{Module: "m", Entity: "e", Action: action}
		parsed, err := router.ParseOperation(op.String())
		if err != nil {
			t.Fatalf("ParseOperation(%q) error: %v", op.String(), err)
		}
		if parsed != op {
			t.Fatalf("ParseOperation(%q) = %+v, want %+v", op.String(), parsed, op)
		}
	}
}

func TestParseOperation_Rejects(t *testing.T) {
	cases := []string{"", "m.e", "m.e.create.x", "m.e.drop", "..create", "m..create"}
	for _, s := range cases {
		_, err := router.ParseOperation(s)
		if err == nil {
			t.Fatalf("ParseOperation(%q) expected error, got nil", s)
		}
		if !strings.Contains(err.Error(), s) {
			t.Fatalf("ParseOperation(%q) error %q does not contain input %q", s, err.Error(), s)
		}
	}
}

func TestOperation_IsComparable(t *testing.T) {
	a := router.Operation{Module: "m", Entity: "e", Action: model.Create}
	b := router.Operation{Module: "m", Entity: "e", Action: model.Create}
	if a != b {
		t.Fatalf("expected %+v == %+v", a, b)
	}

	c := router.Operation{Module: "m", Entity: "e", Action: model.Update}
	if a == c {
		t.Fatalf("expected %+v != %+v", a, c)
	}
}
