package router_test

import (
	"testing"

	"webtyp.com/json"
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/router/mock"
)

func TestIntrospectionRead(t *testing.T) {
	r := &mock.Router{}

	// Register routes to test various scenarios
	// 1. Public route
	r.Get("/public", func(ctx router.Context) {}).Public().Describe("Public route description")

	// 2. Authenticated route
	r.Post("/authenticated", func(ctx router.Context) {}).Authenticated()

	// 3. Guarded route with Accepts
	r.Put("/guarded", func(ctx router.Context) {}).
		Requires("test", model.Update).
		Accepts(&dummyArgs{Name: "test_arg"})

	// 4. Guarded route (orphan)
	r.Delete("/orphan", func(ctx router.Context) {}).
		Requires("other", model.Delete)

	policy := dummyPolicy{
		grants: []model.RoleGrant{
			{Role: "admin", Grant: model.Grant{Resource: "test", Actions: model.Update}},
		},
	}

	router.MountIntrospection(r, router.IntrospectionPath, policy).Public()

	ctx := &mock.Context{InMethod: "GET", InPath: router.IntrospectionPath}
	r.Invoke("GET", router.IntrospectionPath, ctx)

	if ctx.Status != 200 {
		t.Fatalf("expected status 200, got %d", ctx.Status)
	}

	body := ctx.ResponseBody()

	var table router.RouteTable
	if err := json.Decode(body, &table); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(table.Routes) != 5 {
		t.Fatalf("expected 5 routes, got %d", len(table.Routes))
	}

	// Route 0: /public
	if table.Routes[0].Path != "/public" {
		t.Errorf("expected /public, got %s", table.Routes[0].Path)
	}
	if table.Routes[0].Method != "GET" {
		t.Errorf("expected GET, got %s", table.Routes[0].Method)
	}
	if table.Routes[0].Access != "public" {
		t.Errorf("expected access public, got %s", table.Routes[0].Access)
	}
	if table.Routes[0].Description != "Public route description" {
		t.Errorf("expected description, got %s", table.Routes[0].Description)
	}
	if table.Routes[0].HasArgs {
		t.Errorf("expected HasArgs false for /public")
	}
	if table.Routes[0].Orphan() {
		t.Errorf("expected Orphan false for /public")
	}

	// Route 1: /authenticated
	if table.Routes[1].Path != "/authenticated" {
		t.Errorf("expected /authenticated, got %s", table.Routes[1].Path)
	}
	if table.Routes[1].Method != "POST" {
		t.Errorf("expected POST, got %s", table.Routes[1].Method)
	}
	if table.Routes[1].Access != "authenticated" {
		t.Errorf("expected access authenticated, got %s", table.Routes[1].Access)
	}

	// Route 2: /guarded
	if table.Routes[2].Path != "/guarded" {
		t.Errorf("expected /guarded, got %s", table.Routes[2].Path)
	}
	if table.Routes[2].Method != "PUT" {
		t.Errorf("expected PUT, got %s", table.Routes[2].Method)
	}
	if table.Routes[2].Access != "guarded" {
		t.Errorf("expected access guarded, got %s", table.Routes[2].Access)
	}
	if table.Routes[2].Resource != "test" {
		t.Errorf("expected resource test, got %s", table.Routes[2].Resource)
	}
	if table.Routes[2].Action != "u" {
		t.Errorf("expected action u, got %s", table.Routes[2].Action)
	}
	if len(table.Routes[2].Roles) != 1 || table.Routes[2].Roles[0] != "admin" {
		t.Errorf("expected role admin, got %v", table.Routes[2].Roles)
	}
	if !table.Routes[2].HasArgs {
		t.Errorf("expected HasArgs true for /guarded")
	}
	if len(table.Routes[2].Args) != 1 || table.Routes[2].Args[0].Name != "name" || !table.Routes[2].Args[0].Required {
		t.Errorf("expected arg name required, got %v", table.Routes[2].Args)
	}
	if table.Routes[2].Orphan() {
		t.Errorf("expected Orphan false for /guarded")
	}

	// Route 3: /orphan
	if table.Routes[3].Path != "/orphan" {
		t.Errorf("expected /orphan, got %s", table.Routes[3].Path)
	}
	if table.Routes[3].Access != "guarded" {
		t.Errorf("expected access guarded, got %s", table.Routes[3].Access)
	}
	if len(table.Routes[3].Roles) != 0 {
		t.Errorf("expected no roles for /orphan, got %v", table.Routes[3].Roles)
	}
	if !table.Routes[3].Orphan() {
		t.Errorf("expected Orphan true for /orphan")
	}
}
