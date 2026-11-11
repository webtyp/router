# Architecture

## 1. Overview in One Minute

`router` is the isomorphic routing and transport contract of the WebTyp ecosystem. It defines request handling (`Context`, `Router`, `HandlerFunc`), typed operation identity (`Operation`, `Caller`, `OperationRegistry`), and query-string argument codecs (`QueryOf`, `QueryReader`, `EncodeQuery`, `DecodeArgs`).

Rather than passing untyped operation strings across boundaries, operations are represented by `Operation{Module, Entity, Action}`. Transports map operations to their native transport representations (e.g. REST method and path in `rpc`, tool names in `mcp`), while the router core and in-process callers use the comparable `Operation` struct directly. GET requests pass structured arguments encoded in query strings, while mutating actions pass arguments in request bodies.

In every transport `Context.Path()` is the path only and `Context.RawQuery()` is the query. Read it as a record with:
`rec.DecodeFields(router.QueryReader(ctx.RawQuery()))`

## 2. Repository Pieces & On-Disk Layout

| Component | File | Responsibility |
|---|---|---|
| `Operation` | [`op.go`](../op.go) | Typed operation identity `(Module, Entity, Action)` and `ParseOperation` |
| `Caller` | [`caller.go`](../caller.go) | Call-side interface for invoking operations asynchronously |
| `Router` & `Context` | [`router.go`](../router.go), [`context.go`](../context.go) | Routing primitives, HTTP route registration, and middleware |
| Query Codec | [`query_codec.go`](../query_codec.go), [`query.go`](../query.go) | Query parsing, percent encoding, `QueryReader`, `EncodeQuery`, and `DecodeArgs` |
| Introspection | [`introspection.go`](../introspection.go) | Route table introspection and orphan detection |
| Test Doubles | [`mock/`](../mock/) | WASM-safe test doubles (`mock.Router`, `mock.Context`, `mock.Caller`) |
| Conformance | [`conformance/`](../conformance/) | Reusable test suite verifying router implementations |
| Layout Tooling | [`layoutscan/`](../layoutscan/) | Build-destination layout and build-tag verification |

## 3. Step-by-Step Flows

### Flow 1: Operation Dispatch & Invocation
1. **Definition:** A caller forms a typed `Operation{Module: "patient_directory", Entity: "patient", Action: model.Create}`.
2. **Dispatch:** The caller invokes `Caller.Call(op, args, into, done)`.
3. **Transport resolution:** The concrete transport maps `op` to its protocol: `POST /api/patient_directory/patient` over HTTP, or an in-process handler invocation.
4. **Execution:** The server matches the route registered via `OperationRegistry.Operation(op, h)` and executes `h(ctx)`.

### Flow 2: Argument Decoding with `DecodeArgs`
1. **Method Check:** `DecodeArgs` inspects the HTTP method.
2. **GET Requests:** Arguments travel in the query string. `DecodeArgs` creates a `QueryReader` over `QueryOf(path)` and populates `into` without parsing the body.
3. **Mutating Requests (POST, PATCH, DELETE):** Arguments travel in the request body. `DecodeArgs` delegates to `fromBody(body, into)`.

## 4. Security Boundaries & Contracts

- **Doors (Local vs Internet):** The listener port determines whether a request is local or internet, marked via `ContextKeyNetwork`. Unmarked requests are treated as internet by default (fail-closed).
- **Operation Identity:** `Operation.Valid()` requires non-empty `Module`, `Entity`, and exactly one valid `model.Action`. Operations are closed by default.
- **Fail-Closed RBAC:** An unannotated route is private by default; access must be explicitly granted via `Requires(resource, action)` or `Public()`.
- **Query Codec Bounds:** Arrays are strictly disallowed in query strings (`EncodeQuery` fails, `QueryReader.Array` returns false); multi-item collections must be sent in a request body.
