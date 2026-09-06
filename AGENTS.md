# Agent Guide — `webtyp/router`

Constraints for agents working on this library. Read this before any change.

---

## Construction Harness — the WebTyp way (read first)

The typed, explicit code **is** the harness. Whoever writes against this library is
often an agent that does **not** know it; they must produce correct code guided only
by the signatures, and the compiler must **reject** whatever is wrong. Correctness
lives in the compiler and the signatures, not in a manual you must remember. A
harness moves correctness to the compiler; a manual moves it to the reader — the
first is orders of magnitude more reliable for someone with no context.

Every public API must hold to these principles:

1. **Typed over `any`.** No generic holes (`func(...any)`, `interface{}`) in the
   API — intent-typed methods, like the `webtyp/json` writer (`String`, `Int`,
   `Bool`, `Object`, `Array`). `any` is allowed **only** at the I/O edge, never in
   the data. **Reuse already-declared types** (e.g. `fmt.KeyValue`) instead of
   inventing new ones. Generics with an `any` constraint are the same hole in
   disguise: a signature that does not name its real type is not self-describing.
2. **Explicit over implicit.** The name declares the intent; reading the call is
   enough to know what it does, without opening the implementation.
3. **Illegal states unrepresentable.** If something must not happen, it must not be
   writable. One intent = one path, typed to demand exactly what it needs.
4. **One way to do each thing.** A single construction pattern, with no alternatives
   that force a choice or a trip to the docs.
5. **Minimal public surface.** Export exactly what the author uses; internal
   machinery stays unexported — you cannot misuse what you cannot see.
6. **Fail at compile time, not at runtime.** Order of preference to catch an error:
   compile error → noisy dev-mode diagnostic → (never) silent failure.
7. **Self-describing signatures.** Autocomplete must be enough to build. If using
   the API needs a long document, the API is incomplete.

**Litmus test:** if an agent with no context produces correct code guided only by
autocomplete and a few-line example, the harness is closed. If it needs a manual to
avoid mistakes, something is still untyped.

- **Pattern validation:** Patterns are validated at registration (via `ValidatePattern`), never at request time.

---

## Scope — single responsibility

This library declares **only** the isomorphic routing contract — `Context`,
`HandlerFunc`, `Router` — plus the `APIModule` contract by which a module publishes
its API by mounting onto a `Router`. It **implements no concrete server**: it only
defines the shape.

- **Routing only.** It knows nothing of native HTTP, Cloudflare, static files or UI.
  Those are other pieces that *implement* (a concrete server, an edge runtime) or
  *consume* this contract. A concrete router is an interchangeable implementor.
- **Isomorphic.** The same `Context` and the same `Router` hold on the native target
  (`!wasm`) and the edge/`wasm` target. No build tags, and **no `net/http` in the
  public surface** — a handler never touches the standard net stack directly.
- **One dependency only.** The identity contract (zero-dep), embedded by `APIModule`
  so that every API-capable module also carries `ModelName()`. Nothing else.

---

## Access: closed by default, opening is typed

Routes are **private by default**. A `Get`/`Post`/… that declares neither `Public()`
nor `Requires()` denies a caller with no identity. Never weaken this.

Opening is not a marker you must remember to add — it is a different method:

| Intent | Write this | Why |
|---|---|---|
| Serve one generated file (index.html, css, bundle, wasm) | `r.PublicAsset(path, h)` | Public by construction. Returns no `Route`: nothing to gate, nothing to forget. |
| Serve a directory (`web/public`) | `r.PublicDir(prefix, dir)` | Same. Also keeps the directory **visible** to `Routes()` instead of being served by a file-server fallback outside the router. |
| Serve a file that needs permissions | `r.Get(path, h).Requires(res, action)` | A normal route. A PDF body changes nothing. |

The asymmetry is deliberate. **Forgetting to open** used to be a silent failure — a
403 on a blank page, which happened in three repos independently — so opening gets
its own typed, `grep`-able method. **Forgetting to close** already fails safe (the
route stays private), so it needs no new method.

## Access is ONE declaration, not a combination of flags

`RouteInfo.Access` (`model.Access`) has three states, and the **zero value is
`AccessGuarded`**: identity AND a permission on a `Resource`.

| State | Set with | Means |
|---|---|---|
| `AccessGuarded` (zero) | `.Requires(res, action)` | identity + permission. **A route that annotates nothing lands here and is unreachable** until it declares a resource — an enforcer must reject it loudly at startup. |
| `AccessAuthenticated` | `.Authenticated()` | any identity, no resource check. For operations on the caller themselves. |
| `AccessPublic` | `.Public()`, `PublicAsset`, `PublicDir` | no identity at all. |

This replaced a `Public bool` sitting next to an empty-or-not `Resource`. That encoding made
an illegal state writable — `.Public().Requires(...)` — where the gate kept `Public` and
**silently dropped the permission check**: a route that looked protected and was not. A
declared value cannot contradict itself.

## The RBAC vocabulary is typed, and this library never declares it

`Requires` takes `model.Resource` and `model.Action`. Both used to be bare strings, so
swapping them compiled — and the failure was not an error but a **silent denial** at
runtime, in the one place where silence is unacceptable.

- **Resources are open**: the app declares its own (`"service_catalog"`). `router` only
  *types* the vocabulary — a `router.ResourceUsers` constant here would be the very bug
  this closes. The vocabulary belongs to the consumer.
- **Actions are a closed CRUD set** (`model.Create/Read/Update/Delete`), a bit mask.
  Persistence has four verbs and no tool in this ecosystem ever needed a fifth; a
  free-form verb bought nothing and let `"raed"` compile. A domain verb like `approve`
  is **not** a fifth action — it is another *resource*.
- The zero value of `Action` is `0`: **no action**, not "all". Nothing is granted by
  omission.

Never convert to `string` to sidestep the type.

Never add `.Public()` to an asset route: if you are reaching for it, you want
`PublicAsset`. An implementer that serves files outside the router (a `FileServer`
fallback, a prefix rule in middleware) puts them beyond the permission gate and is a
bug, not an optimization.

---

## Testing

```bash
go install webtyp.com/devflow/cmd/gotest@latest
gotest
```

- `gotest`, never `go test`. Stdlib assertions only. Dual WASM/stdlib.
- A fake `Router` (recording into a map) and a fake `Context` (buffering the write)
  prove a handler is writable guided only by the interface. Fix the contracts at
  compile time: `var _ Router = (*fakeRouter)(nil)`, `var _ APIModule =
  (*fakeModule)(nil)`. Publish with `gopush 'message'`.

---

## Documentation First

Update docs **before** code and before `gopush`: keep `docs/PLAN.md` and (when
present) `docs/ARCHITECTURE.md` in sync with the contract, and re-index `README.md`
so every `docs/` file is linked. Diagrams use `flowchart TD`, no `subgraph`, `<br/>`
for line breaks.
