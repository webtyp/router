# webtyp/router

<img src="docs/img/badges.svg">

Isomorphic routing contract — `Context`, `Router`, `HandlerFunc` identical on native and edge/wasm targets. Defines request handling, streaming (SSE), WebSocket upgrade, and middleware. Modules mount their API via `APIModule`. The router defines the shape; concrete servers implement it.

## Quick Start

```go
import "webtyp.com/router"

type MyModule struct{ name string }

func (m MyModule) ModelName() string { return m.name }
func (m MyModule) MountAPI(r router.Router) {
    r.Get("/api/data", func(ctx router.Context) {
        ctx.WriteStatus(200)
        ctx.Write([]byte("hello"))
    })
}
```

## Caller — the call-side contract

```go
func (v *MyView) Refresh() {
    v.Caller.Call("list_services", nil, func(res []byte, err error) {
        if err != nil {
            v.HandleError(err)
            return
        }
        v.Update(res)
    })
}
```

Modules and views depend on `Caller` to invoke server operations without knowing the wire protocol or transport. Adapters live with each transport (e.g. `mcp.NewCaller` in `webtyp/mcp` adapts a JSON-RPC client), while tests use a `mock.Caller`.

## Contracts

- **`Context`**: minimal I/O (read method/path/body, write headers/status) + cookies (SetCookie/Cookie) + identity (`SetUserID`/`UserID`) + typed codec (`Decode`/`Encode`)
- **`Cookie`**: isomorphic HTTP cookie type with SameSite policy (SameSiteDefault/Lax/Strict/None)
- **`HandlerFunc`**: `func(Context)` — the unit of dispatch
- **`Route`**: registration token; supports `Requires(resource, action)` for RBAC, `Public()` for explicit public access, and `Accepts(model.Fielder)` to declare the request-body schema
- **`RouteInfo`**: read-only view of a registered route with method, path, resource, action, public flag, and `Args` (the schema declared via `Accepts`)
- **`Router`**: register HTTP routes (Get/Post/Put/Delete/Handle) returning Route + streaming (Stream/Socket) + middleware (Use) + Routes() for introspection
- **`Streamer`**: Context + Flush() for SSE/streaming responses
- **`Socket`**: bidirectional connection (WebSocket)
- **`Middleware`**: `func(HandlerFunc) HandlerFunc` — transversal logic (auth, logging)
- **`APIModule`**: transport module + `MountAPI(Router)` — registers HTTP routes (mcp endpoint, SSE, assets)
- **`OpRegistry`**: transport-neutral surface — register operations by name (`Op`), no HTTP verb/path
- **`OpModule`**: reusable domain module + `MountOps(OpRegistry)` — depends only on neutral contracts
- **`Caller`**: call-side contract — how a client-side view invokes a named server operation
- **`mock`**: subpackage with canonical test doubles (Router, Context, Route, Caller) — no `net/http`, WASM-safe

## Path Parameters

Routes use `{name}` syntax (matching Go 1.22+ `net/http.ServeMux`):

```go
r.Get("/api/sites/{id}", func(ctx router.Context) {
    id := ctx.Param("id")
    ctx.Write([]byte("site: " + id))
}).Public()
```

- Each parameter `{name}` matches **one non-empty** path segment.
- Parameter names must be non-empty and unique within the pattern.
- Literal segments always take precedence over parameters (e.g., `/api/sites/new` beats `/api/sites/{id}`).
- Wildcards such as `{name...}` are **not** supported and are rejected at registration.

## Query String Parameters

Query string parsing is provided via package-level helper functions `QueryParam` and `QueryUnescape`:

```go
r.Get("/oauth/callback", func(ctx router.Context) {
    code, ok := router.QueryParam(ctx.Path(), "code")
    if !ok {
        ctx.WriteStatus(400)
        return
    }
    // ...
})
```

`QueryParam` and `QueryUnescape` are package-level functions rather than `Context` methods because `Context.Path()` already provides the full path (including any query string), and query parsing is transport-independent—avoiding unnecessary additions to the `Context` interface that would break existing implementations.

## Introspection

Every router implementation shares a standard introspection endpoint returning the route table as JSON.
See [docs/INTROSPECTION.md](docs/INTROSPECTION.md) for details.

## Op — transport-neutral operations, with a typed codec at the edge

`OpRegistry.Op` is the mount-side counterpart of `Caller.Call(name, args, into, done)`: a domain
module registers an operation by **logical name**, never a path or an HTTP verb. It is a **separate
interface from `Router`** on purpose — a transport that only harvests operations (mcp turns each Op
into a tool) must not be forced to impersonate an HTTP router. A reusable module implements
`OpModule` and depends only on these neutral contracts:

```go
func (m *Module) MountOps(r router.OpRegistry) {
    r.Op("upsert_catalog_item", m.upsert).
        Requires("catalog_item", model.Create).
        Accepts(&CatalogItem{})
}

func (m *Module) upsert(ctx router.Context) {
    var in CatalogItem
    if err := ctx.Decode(&in); err != nil {
        ctx.WriteStatus(400)
        return
    }
    // … domain logic …
    ctx.Encode(&out)
}
```

- `Op` + `Accepts` let a transport (e.g. `mcp`) harvest the operation's name, RBAC and schema
  without the module ever importing that transport. `OpModule` makes "transport-agnostic" a
  compile-time fact, not a convention.
- `Context.Decode`/`Encode` let the handler work in typed `model.Decodable`/`Encodable` values —
  it never imports a codec package (`json`, `jsvalue`) directly; the transport supplies the codec.
- `router/conformance` covers all four additions (`op_route_*`, `context_decodes_and_encodes_typed_payload`); an HTTP router that also satisfies `OpRegistry` proves them the same way it proves the rest.

## Design

No `net/http` in the public API. Handlers never import Go's standard library HTTP types. All routing is self-describing via signatures — no runtime type assertions, no hidden machinery.


