---
PLAN: "fix!: routescan reports PublicDir as a prefix declaration"
TAG: v0.1.34
EXECUTOR: jules
REVIEWER: none
---

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.
> Phase 1 follow-up of ROUTES_SINGLE_SOURCE_MASTER_PLAN.md — a gap found by
> Phase 2a (`goflare`): `routescan` cannot tell a build tool that a route is a
> directory subtree, so the tool would have to guess.

# Plan — `routescan` reports `PublicDir` as a prefix

## Context (the executing agent has none — read this fully)

`webtyp.com/router/routescan` parses an application's `routes/routes.go` with
`go/ast` and returns `[]routescan.Decl{Method, Path, Line}` — one entry per
route call, in source order. Build tools (`goflare`, `sitec`) read that list:
`goflare` turns it into Cloudflare's `run_worker_first` (the path prefixes that
must reach the Worker *before* the static-asset layer), `sitec` uses it to catch
route/asset path collisions.

`run_worker_first` **takes prefixes, not leaves**. `routescan` already encodes
that for `Mount`: `r.Mount("/api/auth", …)` is reported as
`Decl{Method:"MOUNT", Path:"/api/auth*"}` — the trailing `MountSuffix` (`"*"`)
*is* the "this is a prefix" signal, and it is the only thing the build side
needs.

### The defect

`r.PublicDir(prefix, dir)` registers **a directory served under a prefix** — the
`router` package's own doc comment says exactly that (`mock/router.go:94`). It is
a subtree, semantically identical to `Mount` for build tooling. But `routescan`
today reports it through the generic selector path:

`routescan.go:96` — `methodOf["PublicDir"]: VerbGet` — so
`r.PublicDir("/static", "web/public")` becomes `Decl{Method:"GET", Path:"/static"}`,
**byte-for-byte identical to `r.Get("/static", h)`**.

A build tool that receives `{"GET", "/static"}` cannot know the project meant
the whole `/static/**` subtree. It is forced to choose between two wrong
guesses:

- treat it as an exact leaf → `/static` is sent to the Worker but
  `/static/app.js` is not; that request falls through to the asset layer, and on
  a Cloudflare deploy with the default `not_found_handling` it returns
  `index.html` with **HTTP 200** and no log. This is the exact silent failure
  the master plan exists to remove.
- append `"*"` to *every* route unconditionally → `r.Get("/dom", h)` also starts
  capturing `/dominion`; routes the project never declared reach the Worker.

The distinction belongs here, in the library that reads the route call — not in
every build tool that consumes the result.

### Anti-footguns

- **`PublicAsset` is NOT a directory.** `r.PublicAsset("/asset.js", h)` serves a
  single file; it is a leaf and MUST stay `Decl{Method:"GET", Path:"/asset.js"}`,
  unchanged. Only `PublicDir` changes.
- This package is **build tooling**, not WASM code: `go/ast`, `go/parser`,
  `go/token`, `strconv` are correct and deliberate. Do not "fix" stdlib imports
  and do not move this code into the package root.
- `Decl.Method` for a `PublicDir` stays `"GET"` — the HTTP method is still GET.
  Only `Decl.Path` gains the `MountSuffix`. Do not invent a new verb constant.

## Design gate

Required by skill **api-design**: this changes the observable output contract of
an exported function (`Scan`).

**Prior art.** Build tools that read a route/asset table all treat a directory
mount as a prefix, never a leaf: Next.js's route manifest marks a segment
`dynamic` vs `static`; Vite/webpack treat `publicDir` as a copy-only subtree
matched by prefix; Go's own `http.FileServer` is always mounted with
`http.StripPrefix("/static/", …)` — a prefix. None of them represents a served
directory as a single exact path.

**Novice-name test.** "`PublicDir` registers a directory served under a prefix"
(the router's own words) reads as *prefix*. A scanner that reports it as the
same shape as `Get` contradicts the name.

**Complexity ledger.** Concepts ±0 (the `MountSuffix` prefix convention already
exists). Files ±0. Call-site lines ±0 for consumers — `goflare`/`sitec` already
read `Decl.Path`. Ways-to-do-it ±0: one branch changes from wrong to right.
Net: **−0 / −1** (one misleading map entry deleted).

**Where it belongs.** `routescan` owns "translate one route call in
`routes/routes.go` into the `Decl` a build tool needs". `PublicDir`'s subtree
nature is part of that translation. Putting it downstream forks the knowledge
into every consumer.

**What it deletes.** The `MethodPublicDir: VerbGet` entry in the `methodOf` map
(`routescan.go:96`) — `PublicDir` no longer flows through the generic selector
branch.

## Stage 1 — special-case `PublicDir` in `collectDecls`

In `routescan/routescan.go`, `collectDecls` already has a dedicated branch for
`MethodMount` that appends `MountSuffix`. Add an equally dedicated branch for
`MethodPublicDir`, immediately after the `MethodMount` branch:

```go
if sel.Sel.Name == MethodPublicDir {
	if len(call.Args) < 1 {
		return true
	}
	prefix, ok := resolveArg(call.Args[0], consts)
	if !ok {
		scanErr = pathError(line)
		return false
	}
	*out = append(*out, Decl{Method: VerbGet, Path: prefix + MountSuffix, Line: line})
	return true
}
```

Rules:

- The path argument is `call.Args[0]` (the prefix). The second argument (`dir`)
  is not a route path and is ignored — same as today.
- A prefix that is not a string literal or an in-file const is a scan error via
  the existing `pathError(line)` — same rule `Mount`, `Handle` and every verb
  already follow. Do not add a new message.
- `Decl.Method` is `VerbGet` (the existing constant). `Decl.Path` is
  `prefix + MountSuffix` (the existing constant). No new constants.

## Stage 2 — delete the stale map entry

Remove this line from the `methodOf` map in `routescan/routescan.go`:

```go
	MethodPublicDir:   VerbGet,
```

`PublicDir` is now handled entirely by the Stage 1 branch, which runs before the
`methodOf` lookup. Leaving the entry in would be dead and misleading — it says
"`PublicDir` is a plain GET leaf", which is the bug.

`MethodPublicAsset: VerbGet` **stays** — `PublicAsset` still flows through the
generic branch and is still a leaf.

**Acceptance:** `grep -n "MethodPublicDir" routescan/routescan.go` → exactly one
hit (the `const MethodPublicDir = "PublicDir"` declaration) plus the Stage 1
branch; **no hit inside the `methodOf` map literal.**

## Stage 3 — update the package doc comment

The package doc comment in `routescan/routescan.go` explains how route calls map
to `Decl`s. Wherever it describes `Mount` producing a `MountSuffix` path, add
one sentence that `PublicDir` does the same, and that `PublicAsset` remains a
leaf. Keep it to the existing comment's style and length — no new doc file.

## Tests

`routescan/routescan_test.go` already has `TestScanMethods`, a table that
exercises every recognised call against expected `Decl`s.

1. **Change the existing `PublicDir` expectation.** In `TestScanMethods`, the
   line

   ```go
   {Method: "GET", Path: "/static", Line: lineOf(src, `"/static"`)},
   ```

   becomes

   ```go
   {Method: "GET", Path: "/static*", Line: lineOf(src, `"/static"`)},
   ```

   The `PublicAsset` expectation (`{Method:"GET", Path:"/asset.js"}`) is
   **unchanged** — that assertion staying green is the proof `PublicAsset` was
   not touched.

2. **Add `TestScanPublicDirIsPrefix`** — a focused regression test, fixture
   written into `t.TempDir()`:

   ```go
   func Register(r router.Router) {
   	r.Get("/static", h)
   	r.PublicDir("/assets", "web/public")
   }
   ```

   Assert the two `Decl`s are, in order:
   `{Method:"GET", Path:"/static"}` and `{Method:"GET", Path:"/assets*"}` —
   i.e. an exact `Get` and a `PublicDir` on adjacent lines produce **different**
   paths. This is the regression proof: it fails against `main` today, where
   both come back as bare paths.

3. **Add `TestScanPublicDirNonLiteralPrefix`** — a `PublicDir` whose prefix is a
   local variable:

   ```go
   func Register(r router.Router) {
   	p := "/assets"
   	r.PublicDir(p, "web/public")
   }
   ```

   Assert `Scan` returns an error and that its message contains
   `routescan.ErrPathNotLiteral`'s text (`route path must be a string literal or
   a const declared in this file`). Same contract as every other selector.

## Acceptance criteria

1. `gotest ./...` → clean (vet, race, cover, WASM suite, README badges — all
   handled by `gotest`; never call `go test` directly).
2. `grep -n "MethodPublicDir" routescan/routescan.go` shows the const
   declaration and the Stage 1 branch, but **not** a `methodOf` map entry.
3. `TestScanPublicDirIsPrefix` passes and fails against `main`
   (`git stash` the source change, keep the test, run it → red).
4. The `PublicAsset` row in `TestScanMethods` is unchanged and green.

## Prerequisite — install the test runner

External agents run in isolated environments where `gotest` is not installed.
Run this **before anything else**:

```bash
go install webtyp.com/devflow/cmd/gotest@latest
```

Then use `gotest` for the whole suite and `gotest -run TestName` for one test.

## Stages

| # | Stage | File(s) | Gate |
|---|---|---|---|
| 1 | `PublicDir` branch in `collectDecls` | `routescan/routescan.go` | tests 1–3 |
| 2 | delete `methodOf[MethodPublicDir]` | `routescan/routescan.go` | criterion 2 |
| 3 | package doc comment | `routescan/routescan.go` | — |

Sequential.
