---
PLAN: "feat!: Router.Mount, Operation rename, and routescan"
EXECUTOR: jules
REVIEWER: none
---

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.
> Part of https://github.com/webtyp/docs — ROUTES_SINGLE_SOURCE_MASTER_PLAN.md.
> This is **phase 1 and a gate**: `goflare` and `sitec` depend on what it ships.
> **Queued after this plan:** `docs/PLAN_SECURITY.md` (the `security` subpackage).

## Prerequisite — install the test runner

External agents run in isolated environments where `gotest` is not installed.
Run this **before anything else**; the acceptance criteria depend on it:

```bash
go install webtyp.com/devflow/cmd/gotest@latest
```

Then use `gotest` for the whole suite and `gotest -run TestName` for one test.
Never call `go test` directly: `gotest` handles `-vet`, `-race`, `-cover`, the
WASM suite and the README badges.

# Plan — `router/routescan`

## Context (the executing agent has none — read this fully)

`webtyp.com/router` is the transport-neutral routing contract of the WebTyp
framework. It is imported by code compiled to WASM, so the package root avoids
heavy standard-library dependencies.

An application declares every route it answers in one file, `routes/routes.go`:

```go
package routes

import (
    "webtyp.com/orm"
    "webtyp.com/router"
    "example.com/app/modules/contact"
)

func Register(r router.Router, db *orm.DB) {
    r.Post("/api/contacto", contact.Handle(db)).Public()
    r.Get("/api/contacto", contact.HandleList(db)).Public()
}
```

Build tools — `goflare` when it deploys, `sitec` when it compiles — need the
**method and path** of every route **without running the application**. Today
they cannot, so `goflare` hardcodes a guess (`WorkerFirstRoutes =
["/api/*", "/oauth/*"]`) and any route outside it is silently shadowed by the
static asset layer.

This plan adds two things: one method on `Router`, and the reader.
`Route`, `RouteInfo`, `APIModule`, `OpModule` and `OpRegistry` are untouched.

### Why `Mount` is needed

A reusable module's routes cannot be listed in the application's file. In
`webtyp.com/auth/authority` the paths come from another package's constants
(`auth.PathLogout`) and **which** routes exist is decided at runtime by the
authenticators the application enabled. No parser can see them, and copying them
into every application would fork the module's contract.

`Mount` lets the application declare the **prefix** a module owns. That is all
build tooling needs: `run_worker_first` takes prefixes, not leaves.

### Anti-footgun

This new subpackage is **build tooling**, not WASM code. It uses `go/ast`,
`go/parser` and `go/token` from the standard library, which is correct and
deliberate. Do **not** "fix" those imports to `webtyp.com/fmt`, and do not move
this code into the package root — the root must stay WASM-safe.

## Design gate

Required by skill **api-design** because this plan adds a method to a public
interface.

### 1. Prior art

| Framework | How a module's routes are mounted under a prefix |
|---|---|
| chi (Go) | `r.Mount("/admin", adminRouter)` — same name, same shape |
| Django | `path("api/auth/", include("auth.urls"))` |
| Laravel | `Route::prefix("api/auth")->group(...)` |
| Express | `app.use("/api/auth", authRouter)` |
| Phoenix | `scope "/api/auth" do ... end` |

Every mature central-manifest framework has this primitive. We are not inventing
one; we are adopting the one a Go developer already knows from chi. The
alternative family — Spring/NestJS-style discovery by scanning at runtime — is
what this ecosystem has today, and it is what makes the routes unreadable to
build tooling.

### 2. Novice-name test — including what this plan renames

`Op` is not a universal Go abbreviation. A reader cannot deduce that
`OpRegistry` means "registry of transport-neutral named operations"; they have to
open the file. The full word costs six characters and removes the lookup.

| Today | Renamed to |
|---|---|
| `OpRegistry` | `OperationRegistry` |
| `OpModule` | `OperationModule` |
| `Op(name, h)` | `Operation(name, h)` |
| `MountOps(reg)` | `MountOperations(reg)` |

Done now because there are no external users: the blast radius is `router`
(3 files), `mcp` (2) and `auth` (4). Every month this waits, that number only
grows, and a bad name published in a tag is permanent.

### 2b. Novice-name test — the new method

`Mount(prefix string, fn func(Router))` reads as "mount these routes under this
prefix". `Mount` is chi's word for exactly this operation, so a Go developer
pays no lookup. Rejected: `Attach` (invented), `Group` (gin's, but gin's returns
a group object — a different shape, so reusing the name would mislead), `Use`
(already taken by middleware on this interface).

### 3. Complexity ledger

```
Concepts to learn                 −2   (APIModule and the type assertion go away for applications)
Lookups to read the contract      −1   (Op* → Operation*: no abbreviation to decode)
Files touched to add a route      −2   (3 → 1)
Call-site lines, app route        −4   (an interface method → one line)
Call-site lines, reusable module  +1   (automatic → one explicit line)
Ways to declare a route            0   (MountAPI survives for domain modules outside applications
                                        until phase 3 completes, then it is removed — see master plan)
```

The one worsening row is stated deliberately: mounting `auth` or `mcp` costs one
written line it did not cost before. That is the price of the surface being
readable at build time and by a human.

### 4. Where it belongs

`Router` is the routing contract; a prefix under which routes are registered is
routing. It is not a second concern and needs no new package. `routescan` **is**
a second concern — reading source at build time — so it is a separate
subpackage with no dependency on the parent.

### 5. What it deletes

Within this repository: nothing yet — `Mount` is additive so the ecosystem can
migrate. The deletions are scheduled and tracked: `APIModule` and the
`if api, ok := m.(router.APIModule); ok` loop are removed once every application
has migrated (phase 3 of ROUTES_SINGLE_SOURCE_MASTER_PLAN.md). This plan does
not leave a permanent second path; it opens a migration whose end state is one.

## Stage 0 — rename `Op*` to `Operation*`

Pure rename, no behaviour change. In `router.go` and every file in this
repository:

| Old | New |
|---|---|
| `OpRegistry` | `OperationRegistry` |
| `OpModule` | `OperationModule` |
| `Op(name string, h HandlerFunc) Route` | `Operation(name string, h HandlerFunc) Route` |
| `MountOps(reg OpRegistry)` | `MountOperations(reg OperationRegistry)` |

Update the doc comments to match: they currently read "harvests each Op as a
tool" and similar. No aliases, no deprecated wrappers — the old names are gone.

Consumers `webtyp.com/mcp` and `webtyp.com/auth` break until they follow; that is
phase 1b of the master plan and deliberate. They are touched once for this and
for `Mount` together, rather than twice.

**Acceptance:** `grep -rn "OpRegistry\|OpModule\|MountOps\|\.Op(" --include='*.go' .` → empty.

## Stage 0b — `Router.Mount`

Add one method to the `Router` interface in `router.go`, immediately after
`PublicDir`:

```go
    // Mount registers a module's routes under a prefix. Every path the callback
    // registers is relative to that prefix. The prefix is what build tooling
    // reads; what the module registers beneath it stays the module's business.
    //
    // The prefix must begin with "/" and must not end with "/".
    Mount(prefix string, fn func(Router))
```

Implement it in **both** implementations that live in this repository:

- `router.go` — the base implementation: it calls `fn` with a router that
  prepends `prefix` to every path registered through it, and records the
  resulting absolute paths in `Routes()` exactly as if they had been registered
  directly. Introduce an unexported `prefixed` type wrapping the parent router;
  do not duplicate the registration logic.
- `mock/router.go` — the test double: same prefixing behaviour.

A prefix that does not begin with `/`, or that ends with `/`, is a programming
error caught at registration: panic with the message
`router: Mount prefix must begin with "/" and must not end with "/"`. This is
startup-time wiring, not request handling — a loud failure is correct here.

Nested `Mount` composes: the prefixes concatenate.

Tests in `mount_test.go`: a mounted route appears in `Routes()` with the joined
path; nested mounts join in order; `.Public()` and `.Requires(...)` on a mounted
route behave identically to a direct one; both invalid prefixes panic.

**Out of scope:** `server/httpd/adapter.go` and `cloudflare/edge/edge.go` are in
other repositories and implement this interface. They break until they add the
method — that is phase 1b of the master plan, not this plan's problem.

## What to build

Create the subpackage **`routescan`** at `routescan/` (its own directory inside
this repository, package name `routescan`). It has no dependency on the parent
`router` package.

### API

```go
// Decl is one route declared in routes/routes.go.
type Decl struct {
    Method string // "GET", "POST", "PUT", "DELETE", "OPTIONS", "STREAM", "SOCKET", or the literal passed to Handle
    Path   string // exactly as written: "/api/contacto"
    Line   int    // 1-based line in routes/routes.go, for error messages
}

// DefaultFile is the path, relative to the project root, that Scan reads.
const DefaultFile = "routes/routes.go"

// Scan parses <rootDir>/routes/routes.go and returns every route declared in it,
// in source order.
//
// It returns an empty slice and a nil error when the file does not exist: a
// project without routes is legal, not an error.
func Scan(rootDir string) ([]Decl, error)
```

### Recognised calls

Inside the body of any function in that file whose **first parameter type is
`router.Router`**, a call is a route declaration when the receiver is that
parameter's identifier and the method is one of:

| Method call | `Decl.Method` |
|---|---|
| `r.Get(p, h)` | `GET` |
| `r.Post(p, h)` | `POST` |
| `r.Put(p, h)` | `PUT` |
| `r.Delete(p, h)` | `DELETE` |
| `r.Options(p, h)` | `OPTIONS` |
| `r.Stream(p, h)` | `STREAM` |
| `r.Socket(p, h)` | `SOCKET` |
| `r.PublicAsset(p, h)` | `GET` |
| `r.PublicDir(prefix, dir)` | `GET`, `Path` = prefix |
| `r.Handle(m, p, h)` | the literal `m` |
| `r.Mount(prefix, fn)` | `MOUNT`, `Path` = `prefix + "*"` |

Trailing chained calls (`.Public()`, `.Requires(...)`, `.Authenticated()`,
`.Accepts(...)`) are ignored — they carry no path. Resolve the method name
through the call expression, never by matching the text `r.` : the parameter may
be named anything.

Identify `router.Router` through the file's **import block**, not by literal
selector text — the import may be aliased.

### The path rule

The path argument MUST be a string literal, or an identifier bound to a
`const` declared in the same file. Anything else — a variable, a function call,
a concatenation, an imported constant — returns an error whose message is
exactly:

```
routes/routes.go:<line>: route path must be a string literal or a const declared in this file
```

This rule is the point of the whole design: a path a build tool cannot read must
not build. Do not add a fallback that skips such a route.

### Errors

- File missing → `nil, nil` (see above).
- File present but unparseable → wrap the `go/parser` error, prefixed
  `routescan: `.
- No function in the file takes `router.Router` as its first parameter →
  error exactly: `routes/routes.go: no function takes router.Router as its first parameter`.

## Constraints (mandatory)

### No hardcoded strings — typed constants only

Every method name, error message and file path is a named constant in the
package. String literals are forbidden in logic.

```go
const DefaultFile = "routes/routes.go"
const ErrPathNotLiteral = "route path must be a string literal or a const declared in this file"
```

The method mapping is a single package-level `var methodOf = map[string]string{…}`
— never a `switch` with repeated literals.

### Minimal surface

Export exactly `Decl`, `Scan`, `DefaultFile` and the error-message constants.
Every parsing helper stays unexported.

## Tests — `routescan_test.go`

Table-driven, using `t.TempDir()` to write a `routes/routes.go` per case. Cover:

1. Every method in the table above, asserting `Method`, `Path` and `Line`.
2. `Handle("PATCH", "/x", h)` → `Method: "PATCH"`.
3. A chained `.Public().Requires(...)` — parsed identically, chain ignored.
4. A parameter named something other than `r` (e.g. `mux`) — still recognised.
5. An aliased import (`rt "webtyp.com/router"`) — still recognised.
6. A path that is a `const` in the file — resolved to its value.
7. A path that is a variable → the verbatim error, with the right line number.
8. A path built by concatenation → the verbatim error.
9. Missing file → empty slice, nil error.
10. A file with no `router.Router` function → the verbatim error.
11. A helper function in the same file that does **not** take `router.Router` —
    its calls must be ignored.
12. `r.Mount("/api/auth", auth.Routes)` → `Decl{Method: "MOUNT", Path: "/api/auth*"}`.

## Acceptance criteria

0. `grep -rn "OpRegistry\|OpModule\|MountOps" --include='*.go' .` → empty.
1. `go build ./... && go vet ./...` → clean.
2. `go test ./...` → passes.
3. `grep -rn "go/ast\|go/parser" --include='*.go' . | grep -v routescan/` → empty
   (the parser must not leak into the WASM-safe package root).
4. `grep -c "func " routescan/routescan.go` — every exported symbol is one of
   `Decl`, `Scan`, `DefaultFile`; nothing else is exported.

## Stages

| # | Stage | File(s) | Gate |
|---|---|---|---|
| 0 | rename `Op*` → `Operation*` | `router.go`, `mock/router.go`, `conformance/conformance.go` | grep empty |
| 0b | `Router.Mount` + `prefixed` | `router.go`, `mock/router.go`, `mount_test.go` | mount tests pass |
| 1 | `Decl`, constants, method map | `routescan/routescan.go` | compiles |
| 2 | `Scan` + literal/const resolution | `routescan/routescan.go` | cases 1–6, 9 |
| 3 | Error paths | `routescan/routescan.go` | cases 7, 8, 10, 11 |
| 4 | Isolation check | — | criterion 3 |

Sequential.
