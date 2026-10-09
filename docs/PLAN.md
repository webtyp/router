---
PLAN: "feat: RouteTable — the decodable reading shape of /_routes, owned next to its encoder"
TAG: v0.4.0
EXECUTOR: jules
REVIEWER: none
---

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.

# Plan — `router`: read back what `MountIntrospection` writes

## 0. Context (read first)

`router.MountIntrospection(r, router.IntrospectionPath, policy)` (`introspection.go`) serves
`GET /_routes`: a JSON object `{"routes": [...]}` where every entry has `method`, `path`,
`resource`, `action` (CRUD letters, e.g. `"ru"`), `access` (`"public"`, `"authenticated"`,
`"guarded"`), optional `description`, `policy_known` (bool), `roles` (array of strings) and, only
when the route declared `Accepts`, `args`: an array of `{name, kind, required}`.

That shape is **encoded** here (`routesResponse`, `routeView`, `argField`, `RouteInfo.EncodeFields`)
but nothing can **decode** it. An API explorer (a browser screen, next plan in
`webtyp/layout/apiexplorer`) and any tool reading `/_routes` would each have to re-declare the
shape — a contract missing at a boundary, which belongs to the owner of the encoder: this package.

## Design gate

### 1. Prior art
- **Kubernetes API discovery** (`/apis`): the server's discovery document has typed client-side
  structs in the same API package (`metav1.APIResourceList`).
- **gRPC server reflection**: the reflection protocol and its client types ship together.
- **Spring Boot Actuator `/actuator/mappings`**: the response DTOs are public types of the
  actuator module, used by both the endpoint and Spring Boot Admin.

All three keep the reader next to the writer so the two cannot drift.

### 2. Novice-name test
`router.RouteTable` — "the route table". `router.RouteRecord` — "one route as read back".
`rec.Orphan()` — "a guarded route nobody can call". `router.ArgRecord` — "one argument it takes".

### 3. Complexity ledger
```
Concepts the developer must learn   +2 (RouteTable/RouteRecord, ArgRecord)
Files they must touch to do X        0
Lines at the call site               −N for every consumer that would have redeclared the shape
Ways to do the same thing            0 (nothing decodes /_routes today)
```

### 4. Where it belongs
Here, beside the encoder: one wire format, one owner. No new dependency (only `webtyp.com/model`).

### 5. What this deletes
Nothing (new capability). Rule for the future: no other package declares the `/_routes` shape.

## 1. Target API (new file `introspection_read.go`, no build tag)

```go
// RouteRecord is one entry of the /_routes table as a client reads it back.
type RouteRecord struct {
    Method      string
    Path        string
    Resource    string
    Action      string // CRUD letters as served: "r", "ru", "crud"; "" when none
    Access      string // "public" | "authenticated" | "guarded"
    Description string
    PolicyKnown bool
    Roles       []string
    Args        []ArgRecord
    HasArgs     bool // false when the entry had no "args" key — different from an empty list
}

// ArgRecord is one field of a route's declared body schema.
type ArgRecord struct {
    Name     string
    Kind     string
    Required bool
}

// Orphan reports a guarded route whose permission NO role holds: it answers 403 to everyone.
// False when PolicyKnown is false: "the server did not say" is not "nobody has it".
func (r RouteRecord) Orphan() bool

// RouteTable is the decoded /_routes response.
type RouteTable struct {
    Routes []RouteRecord
}
```

`RouteRecord`, `ArgRecord` and `RouteTable` implement `model.Decodable` (`DecodeFields(r
model.FieldReader)`, `IsNil() bool`) reading **exactly** the keys the encoder writes, with the
same key strings — move those key names into unexported constants shared by the encoder and the
decoder (`keyRoutes = "routes"`, `keyMethod = "method"`, …) so the two cannot drift. Use
`r.Array(...)` + `ArrayReader.Object(i, &rec)` / `ArrayReader.String(i)`.

Access strings must come from the same place the encoder gets them (`model.Access.String()`),
compared against constants, never retyped literals.

## 2. Stages

| Stage | Files | Content |
|---|---|---|
| 1 | `introspection.go` | key-name constants; the encoder uses them (behaviour unchanged) |
| 2 | `introspection_read.go` | §1 |
| 3 | `tests/introspection_read_test.go` | §3 |
| 4 | `README.md`, `docs/ARCHITECTURE.md` | "I want to read `/_routes` → `router.RouteTable`"; one example |

## 3. Tests (`tests/`, external package; `gotest`)

Round trip through the **real** encoder: build a router with the package's own test helpers or
`router/mock` (read the existing tests for how `MountIntrospection` is tested), register a public
route, an authenticated route, a guarded route with `Accepts` and `Describe`, and a guarded route
whose permission no role holds; serve `/_routes`; decode the body with `webtyp.com/json` into
`router.RouteTable`.

1. Every field round-trips (method, path, resource, action letters, access word, description, roles).
2. A route without `Accepts` → `HasArgs == false`, `Args == nil`; with `Accepts` → `HasArgs` and its fields.
3. `Orphan()`: guarded + policy known + no roles → true; same with `policy == nil` → false; public → false.
4. Encoder output unchanged: the existing introspection tests still pass untouched.

## 4. Code rules (non-negotiable)
- Root package compiles to TinyGo WASM: `webtyp.com/fmt`; no `errors`, `strconv`, `strings`,
  `encoding/json`, no `map`.
- No exported symbol beyond §1. Tests in `tests/`; never export for a test.

## 5. Acceptance criteria
- `gotest ./...` green (stdlib and wasm).
- `grep -n "\"routes\"\|\"method\"\|\"policy_known\"" introspection.go introspection_read.go` →
  each key string appears once, in its constant.
