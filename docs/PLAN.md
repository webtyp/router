---
PLAN: "feat(conformance): pin that a transport surfaces a plain-text error body verbatim"
EXECUTOR: jules
REVIEWER: none
STATUS: running
SESSION: 6524733168918920564
---

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.

# Plan — the error-body clause of the `router.Context` contract

## Why this exists (a real defect, already measured)

`router.Context` has at least five implementations: `server/httpd`, `cloudflare/edge`,
`router/mock`, `router/loopback`'s `inCtx`, and `webtyp/mcp`'s `opContext`. Three of them run
`router/conformance`; two do not.

A handler reports failure like this — it is the convention **every** domain module in the
ecosystem follows:

```go
ctx.WriteStatus(404)
ctx.Write([]byte(err.Error()))   // PLAIN TEXT, never a JSON value
```

`router/loopback` reads it back exactly that way ([loopback.go](../loopback/loopback.go)):

```go
if ctx.status >= 400 {
    done(fmt.Err(string(ctx.out)))   // error body is a message
    return
}
if into != nil && len(ctx.out) > 0 {
    json.Decode(ctx.out, into)       // only the SUCCESS body is JSON
}
```

`webtyp/mcp`'s `opContext` assumed the error body was JSON too, and embedded it verbatim into
its JSON-RPC response. The moment an error message contained a space — i.e. for every real
error message — the response was syntactically broken JSON and the client got
`json decode unexpected character` instead of "patient not found". It shipped that way and
nothing went red, because **no clause of this suite states what a transport must do with a
`status >= 400` plus a plain-text body.** The defect was found by an application integration
test, three repos downstream. That is the exact failure mode this package's own header
comment describes: *"Two implementations can satisfy the interface — compile without a
complaint — and disagree on everything that matters."*

The fix that belongs here is the missing clause. (`mcp` has already been corrected separately
and is not part of this plan.)

## Design gate (skill: api-design)

### 1. Prior art

The suite's **shape** is already justified in `conformance.go`'s own header: it follows
`golang.org/x/net/nettest.TestConn` (for `net.Conn`), `testing/fstest.TestFS` (for `fs.FS`),
and this ecosystem's own `storage/conformance` and `ddl/conformance`. This plan invents no new
mechanism — it adds two clauses to that existing suite, using the `ServeFunc` and `ServeOp`
seams the `Factory` already exposes.

### 2. The novice-name test

Clause names read as sentences, matching the existing style (`op_route_is_invoked_by_name`,
`body_survives_binary_roundtrip`):

- `route_surfaces_plain_text_error_body`
- `op_route_surfaces_plain_text_error_body`

### 3. The complexity ledger

```
Concepts the developer must learn   +0   (no new exported symbol, no new Factory field)
Files they must touch to do X       +0   (implementations already pass a Factory)
Lines at the call site              +0   (both clauses reuse ServeFunc / ServeOp as they are)
Ways to do the same thing            0   (pins the behaviour of the one path that exists)
```

### 4. Where does it belong

`webtyp/router` — the repository that owns `router.Context`. The skill's rule is explicit: *a
contract with a second implementation is not published until a conformance suite in the
repository owning the contract proves them substitutable.* Fixing only the implementation that
got it wrong (`mcp`) is a leaf patch that leaves the next implementation free to repeat it.

### 5. What does this change delete?

Nothing is deleted — this is new contract coverage, and saying so is the honest answer. What it
makes impossible to reintroduce silently: a transport that mangles, swallows or re-interprets a
handler's error body.

## Constraints of this repository (restated inline — do not skip)

- **Scope.** This library declares only the isomorphic routing contract (`Context`,
  `HandlerFunc`, `Router`, `APIModule`). It implements no concrete server. Do not add an
  implementation here.
- **Isomorphic, no build tags.** The same `Context` holds on `!wasm` and on the edge/`wasm`
  target. **No `net/http` anywhere in the public surface.**
- **One dependency only** (the identity contract). Do not add a dependency; in particular
  `conformance` must stay codec-agnostic — it hand-parses its own one fixed fixture shape on
  purpose (see `extractEchoValue`) rather than importing a JSON package.
- **No Go stdlib beyond `testing`** in `conformance` (it already imports `testing` in non-test
  code deliberately — that is the whole point of an importable suite).
- **Tests:** run `gotest`, never `go test`. Do not run `gopush` or `codejob` — those are
  handled outside the agent.

## Step 1 — add the two clauses to `conformance/conformance.go`

Add a helper next to the existing `ok(...)` helper (around line 202):

```go
// failing is the handler the suite registers when it needs a transport to carry a FAILURE
// back. The body is plain text on purpose: ctx.WriteStatus(4xx/5xx) followed by
// ctx.Write([]byte(err.Error())) is how every domain module in this ecosystem reports an
// error, and err.Error() is a sentence, never a JSON value.
func failing(status int, message string) router.HandlerFunc {
	return func(ctx router.Context) {
		ctx.WriteStatus(status)
		ctx.Write([]byte(message))
	}
}
```

Add the two clause functions at the end of the file:

```go
// --- error bodies -----------------------------------------------------------------------

// errorMessage is deliberately a sentence with spaces: a transport that embeds the body
// somewhere a JSON value is expected breaks on exactly this, and on nothing shorter.
const errorMessage = "patient not found"

// routeSurfacesPlainTextErrorBody: a handler reports failure with a status and a plain-text
// body. Both must arrive unchanged. A transport may not re-encode, wrap, truncate or drop the
// body just because the status says failure — the message IS the payload of a failed call, and
// the caller has nothing else to show a user.
func routeSurfacesPlainTextErrorBody(t *testing.T, f Factory) {
	r, serve := build(t, f)

	r.Post(testPath, failing(404, errorMessage)).Public()

	got := serve("POST", testPath, nil, Anonymous)
	if got.Status != 404 {
		t.Errorf("a handler's failure status must reach the caller: got %d, want 404", got.Status)
	}
	if string(got.Body) != errorMessage {
		t.Errorf("a plain-text error body must arrive verbatim: got %q, want %q", got.Body, errorMessage)
	}
}

// opRouteSurfacesPlainTextErrorBody is the same clause on the Operation seam, and it is the one
// that went undetected: an op transport that decodes the SUCCESS body as JSON must not apply
// that assumption to the FAILURE body. webtyp/mcp did, and every business error it carried
// became an unparseable response.
func opRouteSurfacesPlainTextErrorBody(t *testing.T, f Factory) {
	if f.ServeOp == nil {
		t.Skip("implementation does not support Operation yet")
	}
	r, _ := build(t, f)

	opReg(t, r).Operation("failing_thing", failing(404, errorMessage)).Public()

	got := f.ServeOp(r, "failing_thing", nil, Anonymous)
	if got.Status != 404 {
		t.Errorf("an Operation handler's failure status must reach the caller: got %d, want 404", got.Status)
	}
	if string(got.Body) != errorMessage {
		t.Errorf("a plain-text error body must arrive verbatim: got %q, want %q", got.Body, errorMessage)
	}
}
```

Register both in `Run`, in the body/codec group (after the two `body_*` clauses, before
`context_decodes_and_encodes_typed_payload`):

```go
	t.Run("route_surfaces_plain_text_error_body", func(t *testing.T) { routeSurfacesPlainTextErrorBody(t, f) })
	t.Run("op_route_surfaces_plain_text_error_body", func(t *testing.T) { opRouteSurfacesPlainTextErrorBody(t, f) })
```

**Check the exact registration call used by the neighbouring clauses before writing
`r.Post(...).Public()`** — copy the spelling the file already uses for a public POST route (see
`publicServesAnonymous`), do not invent a variant.

## Step 2 — run the suite against the in-repo implementation

`gotest` in this repository runs `tests/conformance_test.go`, which holds `router/mock` to the
contract. Both new clauses must pass there without touching `mock`.

**If `mock` fails either clause, that is a real defect in `mock` and it is in scope:** fix
`mock` so the status and the body survive, and say so in the PR description. Do not weaken the
clause to match the implementation — the clause is the contract.

## Step 3 — documentation

- `README.md`: if it lists what the conformance suite covers, add the error-body clause to that
  list. If it does not enumerate clauses, change nothing.
- Do not add a new `docs/` file. This plan document is deleted when the loop closes.

## Out of scope — do not do these

- **Do not touch `webtyp/mcp`.** Its `opContext` was already fixed and published separately.
  Making `mcp` run this suite is a follow-up in that repository, not here.
- **Do not make `loopback` run the suite.** `Factory.New` must return a `router.Router` and
  `loopback` has none; inventing one is a design decision this plan has not made.
- **Do not add a `Factory` field, a `Setup` field or any exported symbol.** The ledger above is
  `+0` on purpose; a new field makes it positive and fails the gate.
- **Do not change `Response`.** It already carries `Status` and `Body`, which is everything both
  clauses need.

## Acceptance criteria

- `gotest` green in this repository, with `route_surfaces_plain_text_error_body` and
  `op_route_surfaces_plain_text_error_body` both listed as run (not skipped) for `mock`.
- `grep -rn "TODO\|FIXME\|Deprecated" --include='*.go' conformance/ mock/` shows no hit
  introduced by this change.
- No new exported symbol in `conformance` (`failing` is unexported), no new dependency in
  `go.mod`, no `net/http` anywhere.
- A reader of `conformance.go` can tell, from the clause comments alone, why a plain-text error
  body is a contract and not an accident.

## Stages

| # | Stage | Deliverable |
|---|---|---|
| 1 | `failing(...)` helper + two clause functions + registration in `Run` | `conformance/conformance.go` |
| 2 | Suite green against `mock` (fix `mock` if it fails a clause) | `gotest` verde |
| 3 | README touch-up only if it enumerates clauses | `README.md` |
