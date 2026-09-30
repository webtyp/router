---
PLAN: "feat!: Route.Describe — every route and operation can say what it does, in words a person or a model can read"
TAG: v0.3.0
EXECUTOR: jules
REVIEWER: none
STATUS: running
SESSION: 10379306715931490546
---

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.

# Plan — `webtyp.com/router`: routes and operations carry a description

## 0. Context

`router` is the ecosystem's routing contract. A domain module registers HTTP routes
(`Router.Get/Post/...`) or transport-neutral **operations** (`OperationRegistry.Operation(name,
handler)`). Each registration returns a `Route`, and the module chains the route's declarations
on it: `Requires(resource, action)` (RBAC), `Authenticated()`, `Public()`,
`Accepts(args)` (the argument schema). A transport reads them back as `RouteInfo`. For example,
`webtyp.com/mcp` turns every operation into an MCP tool for AI agents, whose `tools/list` a model
reads to choose which tool to call.

**The defect.** Nothing lets a module say **what an operation does**. So `tools/list` gives a
model only names such as `business_calendar.list_business_hours`, and a small model cannot pick
tools reliably from names. The clinic assistant that uses these tools (Jose) needs descriptions
like `"Horario de atención del consultorio para cada día de la semana: día (0 domingo … 6
sábado), minuto de apertura y de cierre desde la medianoche, y si abre."`.

**The fix:** one more declaration on `Route`, and one more field on `RouteInfo`:

```go
// Describe says what the route does, in one or two sentences a person or a language model
// reads to decide whether to call it: what it returns or changes, and the units of its data.
Describe(text string) Route
```

```go
// Description is what Route.Describe declared; "" when the route declared nothing.
Description string
```

This repository changes the contract and its implementations inside this repository (`mock`,
`loopback` and anything else in this repository that implements `router.Route`). The other
implementations (`webtyp.com/server/httpd`, `webtyp.com/cloudflare/edge`, `webtyp.com/mcp`) are
adapted in their own repositories after this version is published. You do not have them; do not
look for them.

## Development rules (inline)

- Library code compiles for the browser: `GOOS=js GOARCH=wasm go build ./...`. Never import `fmt`,
  `errors`, `strings`, `strconv` (use `webtyp.com/fmt`), `encoding/json` (use `webtyp.com/json`),
  or `map[K]V` in non-test files.
- Follow the file's existing style: `RouteInfo` fields have a comment saying what they are.
  `EncodeFields` declares the wire shape by hand, with no reflection.
- Tests live in `tests/` (package `tests`) and use only the exported API. This repository
  already follows that rule. Keep it.
- No `TODO`, no commented-out code. `gotest` green at the end.

## Design gate (api-design — five answers)

1. **Prior art.** OpenAPI puts `summary` and `description` on every operation. MCP's `Tool` has a
   `description` that the model reads. gRPC and protobuf use the comment on each `rpc`. Cobra has
   `Short` on every command. All of them keep the description **next to the declaration of the
   operation**, as data a tool can read. Here the declaration is the chain on `Route`, so the
   description goes there too.
2. **Novice-name test.** `reg.Operation("list_business_hours", h).Requires("business_hours",
   model.Read).Describe("Horario de atención…")` reads as the sentence it means.
   `RouteInfo.Description` is the word every ecosystem above uses.
3. **Complexity ledger.** Concepts +1 (`Describe`). Files to touch: the module's existing
   `MountOperations`. Lines at the call site +1 per operation. Ways to do the same thing: +0.
4. **Where it belongs.** `router` owns the declaration surface (`Route`) and its read-back
   (`RouteInfo`). Every transport reads it from there, and none invents its own field.
5. **What it deletes.** Nothing. It is a new capability. It also makes `mcp`'s empty tool
   descriptions for harvested operations fixable, in `mcp`'s own follow-up.

## Stage 1 — the contract (`route.go`)

- Add `Describe(text string) Route` to the `Route` interface, after `Accepts`, with the doc
  comment of §0.
- Add `Description string` to `RouteInfo`, after `Access`, with the doc comment of §0.
- `RouteInfo.EncodeFields`: write `"description"` **only when non-empty**, after `"access"` (follow
  how the method already writes optional fields). Keep the decode side in step if `RouteInfo`
  has one.

## Stage 2 — the implementations in this repository

- `mock/route.go`: `func (r *Route) Describe(text string) router.Route { r.info.Description = text; return r }`.
- `loopback/loopback.go`: `noopRoute.Describe` returns the route unchanged, like its other
  methods.
- Run `grep -rn "router.Route = \|) Accepts(" --include=*.go .` and implement `Describe` on every
  other type in this repository that implements `router.Route`, recording the text into its
  `RouteInfo` wherever that type keeps one.
- `introspection.go`: the routes view includes `description` when non-empty (the same key as
  `EncodeFields`).
- `conformance/`: if the suite checks what `Route` declarations are read back, add `Describe`:
  a route that declared `Describe("x")` reads back `Description == "x"`, and one that did not
  reads back `""`.

## Stage 3 — tests (`tests/`)

- `tests/describe_test.go`:
  - an operation registered with `.Requires(...).Describe("Horario de atención")` exposes that
    text in its `RouteInfo`, through the mock registry's read-back;
  - order does not matter: `.Describe(...)` before `.Requires(...)` gives the same `RouteInfo`;
  - a route without `Describe` has `Description == ""`;
  - `EncodeFields` output contains `"description":"Horario de atención"` for the first and no
    `"description"` key for the last. Compare the encoded JSON the way `route_encode_test.go`
    already does.
- Every existing test stays green.

## Stage 4 — docs

- `README.md`: in the `Route` bullet, add `Describe(text)`, "what the route does, read by people
  and by AI agents through mcp's tools/list". Add `.Describe("…")` to the operation example that
  shows `Accepts(&CatalogItem{})`.
- `docs/INTROSPECTION.md`: document the optional `description` key next to `args`.

## Stages

| Stage | Files | Acceptance |
|---|---|---|
| 1 | `route.go` | compiles; `RouteInfo.Description` exists |
| 2 | `mock/route.go`, `loopback/loopback.go`, `introspection.go`, `conformance/` | every `router.Route` in this repo implements `Describe` |
| 3 | `tests/describe_test.go` | new tests pass |
| 4 | `README.md`, `docs/INTROSPECTION.md` | both mention `Describe` / `description` |
| all | — | `gotest` green; `GOOS=js GOARCH=wasm go build ./...` |
