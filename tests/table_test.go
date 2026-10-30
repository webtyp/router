package router_test

import (
	"bytes"
	"testing"

	"webtyp.com/json"
	"webtyp.com/model"
	"webtyp.com/router"
)

func TestTable_NoPolicy(t *testing.T) {
	routes := []router.RouteInfo{
		{
			Method:      "POST",
			Path:        "/api/access/list_roles",
			Resource:    "access_role",
			Action:      model.Read,
			MFA:         true,
			Description: "list roles",
			Args:        &dummyArgs{},
			Access:      model.AccessGuarded,
		},
	}

	table := router.Table(routes, nil)

	if len(table.Routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(table.Routes))
	}

	rec := table.Routes[0]
	if rec.Method != "POST" || rec.Path != "/api/access/list_roles" || rec.Resource != "access_role" ||
	   rec.Action != model.Read.String() || rec.Access != model.AccessGuarded.String() || !rec.MFA ||
	   rec.Description != "list roles" || rec.PolicyKnown != false || len(rec.Roles) != 0 ||
	   !rec.HasArgs || len(rec.Args) != 1 || rec.Args[0].Name != "name" || rec.Args[0].Kind != "" || !rec.Args[0].Required {
		t.Errorf("record mismatch: %+v", rec)
	}
}

func TestTable_WithPolicy(t *testing.T) {
	routes := []router.RouteInfo{
		{
			Method:      "POST",
			Path:        "/api/access/list_roles",
			Resource:    "access_role",
			Action:      model.Read,
			MFA:         true,
			Description: "list roles",
			Args:        &dummyArgs{},
			Access:      model.AccessGuarded,
		},
	}

	policy := dummyPolicy{
		grants: []model.RoleGrant{
			{Role: "admin", Grant: model.Grant{Resource: "access_role", Actions: model.Read}},
		},
	}

	table := router.Table(routes, policy)

	rec := table.Routes[0]
	if !rec.PolicyKnown {
		t.Errorf("expected PolicyKnown=true")
	}
	if len(rec.Roles) != 1 || rec.Roles[0] != "admin" {
		t.Errorf("expected Roles=[\"admin\"], got %v", rec.Roles)
	}
}

func TestTable_PublicWithoutArgs(t *testing.T) {
	routes := []router.RouteInfo{
		{
			Method: "GET",
			Path:   "/health",
			Access: model.AccessPublic,
		},
	}

	table := router.Table(routes, nil)

	rec := table.Routes[0]
	if rec.Access != "public" {
		t.Errorf("expected Access=\"public\", got %s", rec.Access)
	}
	if rec.HasArgs {
		t.Errorf("expected HasArgs=false")
	}
	if rec.Args != nil {
		t.Errorf("expected Args=nil, got %v", rec.Args)
	}
}

func TestTable_WireRoundTrip(t *testing.T) {
	routes := []router.RouteInfo{
		{
			Method:      "POST",
			Path:        "/api/access/list_roles",
			Resource:    "access_role",
			Action:      model.Read,
			MFA:         true,
			Description: "list roles",
			Args:        &dummyArgs{},
			Access:      model.AccessGuarded,
		},
		{
			Method: "GET",
			Path:   "/health",
			Access: model.AccessPublic,
		},
	}

	policy := dummyPolicy{
		grants: []model.RoleGrant{
			{Role: "admin", Grant: model.Grant{Resource: "access_role", Actions: model.Read}},
		},
	}

	table := router.Table(routes, policy)

	var buf bytes.Buffer
	err := json.Encode(&table, &buf)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	var decodedTable router.RouteTable
	err = json.Decode(buf.Bytes(), &decodedTable)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if len(decodedTable.Routes) != len(table.Routes) {
		t.Fatalf("routes length mismatch: expected %d, got %d", len(table.Routes), len(decodedTable.Routes))
	}

	for i := range table.Routes {
		orig := table.Routes[i]
		dec := decodedTable.Routes[i]

		if dec.Method != orig.Method || dec.Path != orig.Path || dec.Resource != orig.Resource ||
		   dec.Action != orig.Action || dec.Access != orig.Access || dec.MFA != orig.MFA ||
		   dec.Description != orig.Description || dec.PolicyKnown != orig.PolicyKnown ||
		   dec.HasArgs != orig.HasArgs {
			t.Errorf("route %d mismatch: orig=%+v, dec=%+v", i, orig, dec)
		}

		if len(dec.Roles) != len(orig.Roles) {
			t.Errorf("route %d roles length mismatch", i)
		}
		for j := range dec.Roles {
			if dec.Roles[j] != orig.Roles[j] {
				t.Errorf("route %d role %d mismatch", i, j)
			}
		}

		if len(dec.Args) != len(orig.Args) {
			t.Errorf("route %d args length mismatch", i)
		}
		for j := range dec.Args {
			if dec.Args[j] != orig.Args[j] {
				t.Errorf("route %d arg %d mismatch: orig=%+v, dec=%+v", i, j, orig.Args[j], dec.Args[j])
			}
		}
	}
}
