---
PLAN: "feat: router.Table — the /_routes table built in process, one builder for server and browser"
EXECUTOR: jules
REVIEWER: none
STATUS: running
SESSION: 12372236154124309452
---

> Read AGENTS.md at the repo root before any change.

# `router.Table`

## Context

The API explorer screen shows the route table an app serves at `/_routes` (`MountIntrospection`,
`introspection.go:56`). A module's demo runs entirely in the browser, with no server, so the explorer finds nothing.
Decided: the demo shows the table built **in process** from its own operations. For that, the table must be buildable
without HTTP. Today the builder is fused with the HTTP handler (`routesResponse`/`routeView`/`argField` write JSON
directly) and the client type (`RouteTable`/`RouteRecord`/`ArgRecord`) can only be decoded. This plan makes **one**
builder, `router.Table`, used by `MountIntrospection` and by any page without a server, and gives the client types
their encoder so the wire stays identical.

## Tools

```bash
go install webtyp.com/devflow/cmd/gotest@latest
```

Environment: `GOPRIVATE=webtyp.com/*,github.com/veltylabs/*`, `GOFLAGS=-mod=mod`.

## Ecosystem search

Searched `RouteTable`, `routeView`, `Table(` over `webtyp/`: the table is only produced inside `MountIntrospection`;
`layout/apiexplorer` decodes `router.RouteTable`. **Extend** `router`; the three private wire types are **deleted**.

## Design gate

1. **Prior art.** Rails `rails routes` and Django `show_urls` list routes from the in-process router, not over HTTP;
   Spring Actuator's `mappings` serves the same in-process list over HTTP (unverified: pages not read in this session).
   Same split: one in-process builder, HTTP is one way to show it.
2. **Novice name.** `router.Table(routes, policy)` — "the route table".
3. **Complexity ledger.** Concepts +0 (`RouteTable` exists) · ways to build the table −1+1 (the private writer dies) ·
   types −3 (`routesResponse`, `routeView`, `argField`).
4. **Where it belongs.** `router`, owner of `RouteInfo` and of the table.
5. **Deletes.** `routesResponse`, `routeView`, `argField` and their methods.

## APIs used

| Symbol | Exact signature | File |
|---|---|---|
| `RouteInfo` | fields `Method, Path string; Resource model.Resource; Action model.Action; Access model.Access; MFA bool; Description, Dir string; Args model.Fielder` | `route.go:53` |
| `model.RolesFor` | `func RolesFor(p PolicyDescriber, r Resource, a Action) []RoleCode` | `webtyp.com/model` (`rbac.go:297`) |
| `model.Action.String`, `model.Access.String` | `func (a Action) String() string`, `func (a Access) String() string` | `webtyp.com/model` |
| keys | `keyRoutes … keyRequired` | `introspection.go:8-21` |

If a signature differs, stop and name the symbol in the PR; never adapt it by guessing.

## Stage 1 — tests first: `tests/table_test.go` (new, `package router_test`)

Reuse `dummyPolicy` and `dummyArgs` from `tests/introspection_test.go`. Red after this stage (`router.Table` and the
encoders do not exist).

| Test | Setup | Call | Expected |
|---|---|---|---|
| `TestTable_NoPolicy` | one `RouteInfo{Method: "POST", Path: "/api/access/list_roles", Resource: "access_role", Action: model.Read, MFA: true, Description: "list roles", Args: &dummyArgs{}}` | `router.Table(routes, nil)` | one record: same method/path; `Resource "access_role"`; `Action == model.Read.String()`; `Access == model.AccessGuarded.String()`; `MFA`; `Description`; `PolicyKnown == false`; `Roles` empty; `HasArgs == true`; `Args == [{Name: "name", Kind: "", Required: true}]` |
| `TestTable_WithPolicy` | `dummyPolicy` granting role `"admin"` read on `"access_role"` | `router.Table(routes, policy)` | `PolicyKnown == true`, `Roles == ["admin"]` |
| `TestTable_PublicWithoutArgs` | `RouteInfo{Method: "GET", Path: "/health", Access: model.AccessPublic}` | `router.Table` | `Access == "public"`, `HasArgs == false`, `Args == nil` |
| `TestTable_WireRoundTrip` | the table of `TestTable_WithPolicy` plus the public route | `json.Encode(&t)` then `json.Decode` into a new `router.RouteTable` (`webtyp.com/json`, already a test dependency or add it) | the decoded table equals the original field by field, including `HasArgs` |

The existing `TestMountIntrospection` (`tests/introspection_test.go:29`) is the proof the served JSON did not change:
it must keep passing **unchanged**. Commit after this stage.

## Stage 2 — code

### `introspection_read.go` — encoders next to the decoders

```go
func (t *RouteTable) EncodeFields(w model.FieldWriter) {
	arr := w.Array(keyRoutes, len(t.Routes))
	for i := range t.Routes {
		arr.Object(&t.Routes[i])
	}
	arr.Close()
}

func (r *RouteRecord) EncodeFields(w model.FieldWriter) {
	w.String(keyMethod, r.Method)
	w.String(keyPath, r.Path)
	w.String(keyResource, r.Resource)
	w.String(keyAction, r.Action)
	w.String(keyAccess, r.Access)
	if r.MFA {
		w.Bool(keyMFA, true)
	}
	if r.Description != "" {
		w.String(keyDescription, r.Description)
	}
	w.Bool(keyPolicyKnown, r.PolicyKnown)
	roles := w.Array(keyRoles, len(r.Roles))
	for _, s := range r.Roles {
		roles.String(s)
	}
	roles.Close()
	// args are omitted entirely when the route declared none: an empty list
	// would claim the route takes an empty body, which is a different statement.
	if r.HasArgs {
		args := w.Array(keyArgs, len(r.Args))
		for i := range r.Args {
			args.Object(&r.Args[i])
		}
		args.Close()
	}
}

func (a *ArgRecord) EncodeFields(w model.FieldWriter) {
	w.String(keyName, a.Name)
	w.String(keyKind, a.Kind)
	w.Bool(keyRequired, a.Required)
}
```

### `introspection.go`

Add:

```go
// Table is the route table a client reads from IntrospectionPath, built in
// process: what MountIntrospection serves, and what a page with no server
// (a module's demo) shows from its own operations. A nil policy leaves every
// route's roles unknown (PolicyKnown false), which is not "nobody holds it".
//
// Example: t := router.Table(r.Routes(), policy)
func Table(routes []RouteInfo, policy model.PolicyDescriber) RouteTable {
	t := RouteTable{Routes: make([]RouteRecord, 0, len(routes))}
	for _, info := range routes {
		rec := RouteRecord{
			Method:      info.Method,
			Path:        info.Path,
			Resource:    string(info.Resource),
			Action:      info.Action.String(),
			Access:      info.Access.String(),
			MFA:         info.MFA,
			Description: info.Description,
			PolicyKnown: policy != nil,
		}
		if policy != nil {
			for _, role := range model.RolesFor(policy, info.Resource, info.Action) {
				rec.Roles = append(rec.Roles, string(role))
			}
		}
		if info.Args != nil {
			rec.HasArgs = true
			fields := info.Args.Schema()
			rec.Args = make([]ArgRecord, 0, len(fields))
			for i := range fields {
				kind := ""
				if fields[i].Type != nil {
					kind = fields[i].Type.Name()
				}
				rec.Args = append(rec.Args, ArgRecord{Name: fields[i].Name, Kind: kind, Required: fields[i].NotNull})
			}
		}
		t.Routes = append(t.Routes, rec)
	}
	return t
}
```

Replace the body of the handler in `MountIntrospection` with:

```go
		t := Table(r.Routes(), policy)
		ctx.WriteStatus(200)
		if err := ctx.Encode(&t); err != nil {
			ctx.WriteStatus(500)
			ctx.Write([]byte("encoding routes failed"))
		}
```

Delete `routesResponse`, `routeView`, `argField` and all their methods. `grep -n "routeView\|routesResponse\|argField" *.go` → empty.

Run `gotest`: the new tests and the unchanged `TestMountIntrospection` pass. Commit after this stage.

## Stage 3 — docs

`README.md` (or `docs/` where introspection is described): one paragraph — `router.Table(routes, policy)` builds the
table that `MountIntrospection` serves; a page with no server builds the same table from its own operations. Commit.

## Delivery check

1. `gotest` passes, with `tests/introspection_test.go` unchanged. A PR that does not compile, or whose tests could not run, is a failed delivery.
2. `grep -n "routeView\|routesResponse\|argField" *.go` → empty.
3. `git diff --stat main` lists only `tests/table_test.go`, `introspection.go`, `introspection_read.go`, the doc touched (and `go.mod`/`go.sum` if `webtyp.com/json` was added for the test).

## Delivery rules

- Never pause to ask permission or report progress: when the delivery check passes, commit, push and open the PR. Stuck
  for 30 minutes: open the PR anyway, listing what fails.
- Commit after every stage. Never run `git reset`, `git restore`, `git checkout -- <file>` or `git stash` on your own work.
- Never edit `tests/introspection_test.go`: it is the proof the wire did not change.

## Stages

| # | Files | Result |
|---|---|---|
| 1 | `tests/table_test.go` | Builder and round-trip tests, red |
| 2 | `introspection.go`, `introspection_read.go` | One builder; private writers deleted; green |
| 3 | `README.md` / docs | Documented |
