package router_test

import (
	"strings"
	"testing"

	"webtyp.com/json"
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/router/mock"
)

func TestDescribeOperationExposesDescription(t *testing.T) {
	r := &mock.Router{}

	r.Operation("list_hours", func(ctx router.Context) {}).
		Requires("business_hours", model.Read).
		Describe("Horario de atención")

	routes := r.Routes()
	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}

	if routes[0].Description != "Horario de atención" {
		t.Errorf("expected Description %q, got %q", "Horario de atención", routes[0].Description)
	}
}

func TestDescribeOrderDoesNotMatter(t *testing.T) {
	r1 := &mock.Router{}
	r1.Operation("op1", func(ctx router.Context) {}).
		Requires("res", model.Read).
		Describe("Horario de atención")

	r2 := &mock.Router{}
	r2.Operation("op2", func(ctx router.Context) {}).
		Describe("Horario de atención").
		Requires("res", model.Read)

	info1 := r1.Routes()[0]
	info2 := r2.Routes()[0]

	if info1.Description != info2.Description {
		t.Errorf("expected same Description regardless of call order: %q vs %q", info1.Description, info2.Description)
	}
	if info1.Resource != info2.Resource || info1.Action != info2.Action || info1.Access != info2.Access {
		t.Errorf("expected same RBAC fields regardless of call order")
	}
}

func TestRouteWithoutDescribeHasEmptyDescription(t *testing.T) {
	r := &mock.Router{}
	r.Operation("op", func(ctx router.Context) {}).Public()

	routes := r.Routes()
	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}

	if routes[0].Description != "" {
		t.Errorf("expected empty Description, got %q", routes[0].Description)
	}
}

func TestEncodeFieldsIncludesDescriptionWhenNonEmpty(t *testing.T) {
	infoWithDesc := router.RouteInfo{
		Method:      "GET",
		Path:        "/test",
		Access:      model.AccessPublic,
		Description: "Horario de atención",
	}

	var outWithDesc string
	if err := json.Encode(infoWithDesc, &outWithDesc); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(outWithDesc, `"description":"Horario de atención"`) {
		t.Errorf("expected description key in JSON output: %s", outWithDesc)
	}

	infoWithoutDesc := router.RouteInfo{
		Method: "GET",
		Path:   "/test",
		Access: model.AccessPublic,
	}

	var outWithoutDesc string
	if err := json.Encode(infoWithoutDesc, &outWithoutDesc); err != nil {
		t.Fatal(err)
	}

	if strings.Contains(outWithoutDesc, `"description"`) {
		t.Errorf("description key should be omitted when empty: %s", outWithoutDesc)
	}
}
