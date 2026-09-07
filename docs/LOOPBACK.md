# `router/loopback` — the in-process Caller

`loopback.New(mods ...router.OperationModule) router.Caller` invokes, **in
process and synchronously**, the operations that one or more domain modules
registered via `MountOperations`. It is the reference in-process `Caller` of
the ecosystem, in contrast with `mcp.NewCaller` (a real JSON-RPC/SSE client).

## When to use it

- A demo or offline app that imports the **real domain modules** and wants to
  drive them with no server (`app-demo` is the canonical consumer).
- Consumer tests that exercise a module's full op flow (mount → call →
  decode) without a transport.
- Any composition root that wants the same codec path (`webtyp/json`) a
  deployed transport would use, but zero network.

## How it works

`New(mods...)` mounts each module onto an internal `router.OperationRegistry`
backed by a linear-scanned `[]struct{name, handler}` — no `map`, so the
package compiles into a WASM binary. Each `Operation` returns a `noopRoute`
that swallows the `.Requires(...).Accepts(...)` annotations: the loopback
caller trusts the composition root, so RBAC metadata is irrelevant to it, but
the chains still compile and run unchanged.

`Call(op, args, into, done)`:

1. Encodes `args` with `webtyp/json` and hands it to the handler through an
   in-memory `router.Context`.
2. Handler runs synchronously; `ctx.Decode`/`ctx.Encode` are backed by the
   real codec.
3. Status ≥ 400 → `done` with an error carrying the response body.
   Otherwise, if `into != nil`, the body is decoded into it and `done(nil)`.

`Dispatch(op, args)` is fire-and-forget: the handler still runs, but a failing
handler does not propagate.

## Example

```go
db := orm.New(mem.New())
ab := appointment_booking.New(db, deps)

caller := loopback.New(ab, catalog, work_schedule)

var rows []appointment_booking.Reservation
caller.Call("list_reservations_by_staff", &ListReservationsByStaffArgs{...},
    &rows, func(err error) { /* … */ })
```

## Difference vs `mcp.NewCaller`

| | `mcp.NewCaller` | `loopback.New` |
|---|---|---|
| Transport | JSON-RPC over SSE/WebSocket | in-process (no transport) |
| RBAC | enforced by the server | trusted (no gate) |
| Async | yes (network I/O) | synchronous; `done` fires before return |
| Codec | `webtyp/json` at the edge | same `webtyp/json`, same round-trip |