---
PLAN: "feat: ContextKeyRemoteAddr — name the context key carrying the client network address"
EXECUTOR: jules
REVIEWER: none
---

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.
>
> **Phase R1 (GATE)** of
> [`LAN_RUT_AUTH_MASTER_PLAN.md`](https://github.com/tinywasm/app/blob/main/docs/LAN_RUT_AUTH_MASTER_PLAN.md).
> `webtyp/server` (R2) and `webtyp/auth` (its phase A) consume this constant;
> both wait for this tag.

# Plan — `webtyp.com/router`: one named key for the client address

## 0. Context

`router.Context.Value(key)` is the transport-agnostic bag where a server
implementation exposes request facts to handlers. `webtyp.com/auth`'s
`ClientIP` already reads `ctx.Value("RemoteAddr")` — a **bare literal in a
consumer**, with no producer: `webtyp/server/httpd` never sets it, so over
real HTTP `ClientIP` returns `""` and every IP-bound login silently fails
against a real server (it only ever worked against test doubles that called
`SetValue` by hand). The seam has no contract — by the harness, a missing
contract at a boundary is a defect in the library that owns the boundary:
`router` owns `Context.Value`'s vocabulary.

## Design gate (api-design — five answers)

1. **Prior art.** **Go `net/http`**: `Request.RemoteAddr` ("the network
   address that sent the request", `host:port` form). **Express**: `req.ip`
   / `req.socket.remoteAddress`. **ASP.NET Core**:
   `HttpContext.Connection.RemoteIpAddress`. All three name the fact in the
   request contract; none leaves it to a string convention. We differ only in
   mechanism: `router.Context` is transport-agnostic, so the fact travels
   under a named context key instead of a struct field every
   implementation must add.
2. **Novice-name test.** `router.ContextKeyRemoteAddr` — "the context key for
   the remote address"; the value is documented as the same `host:port` form
   `net/http` delivers, unparsed (parsing is `auth.ClientIP`'s job, which
   already exists).
3. **Complexity ledger.** Concepts +1 constant / −1 convention ("RemoteAddr"
   as folklore). Literals across repos −2 (`auth.ClientIP` in phase A; the
   httpd producer in phase R2 would otherwise add a second). Ways to do the
   same thing +0 / −0.
4. **Where it belongs.** `router` owns `Context` and therefore the keys any
   transport may be asked for. `auth` (a consumer) must not mint the
   vocabulary; `server` (a producer) must not guess it.
5. **What it deletes.** The `"RemoteAddr"` literal in `webtyp.com/auth`'s
   `ClientIP` (deleted by auth's phase A plan once this tag ships).

## Stage 1 — the constant

**File:** `router.go`, next to the `Context` interface.

```go
// ContextKeyRemoteAddr is the Context.Value key under which every transport
// exposes the client network address of the request, in the same "host:port"
// form the platform delivers it (net/http's Request.RemoteAddr) — unparsed.
// Transports MUST populate it; consumers (e.g. auth.ClientIP) read it
// instead of agreeing on a bare string.
const ContextKeyRemoteAddr = "RemoteAddr"
```

Extend the `Context` interface doc comment for `Value`/`SetValue` with one
sentence: keys whose meaning crosses the transport boundary are named by
`ContextKey*` constants in this package.

## Stage 2 — docs

`README.md` / `docs/`: one row in the context-key table (create the table if
absent — it has exactly one row today). VERIFY against the implementation.

## Acceptance criteria

1. `go build ./...`, `go vet ./...`, `gotest ./...` green.
2. `grep -rn "ContextKeyRemoteAddr" .` → the constant + docs; no behavior change in this repo.

| Stage | File | Action |
|---|---|---|
| 1 | `router.go` | `ContextKeyRemoteAddr` + `Value` doc |
| 2 | README/docs | verify docs |
