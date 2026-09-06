package router

import "webtyp.com/model"

// Context is the minimal abstraction seen by a handler: request → response.
// Same interface signature for both native (!wasm) and edge/wasm targets.
//
// Ownership: a Context belongs to ONE goroutine (the handler's);
// implementations are not required to be safe for concurrent use
// (same contract as http.ResponseWriter). To feed it from other
// goroutines, send the data over a channel to the owning goroutine —
// never share the Context itself.
type Context interface {
	Method() string
	Path() string
	Body() []byte
	GetHeader(key string) string
	SetHeader(key, value string)
	WriteStatus(code int)
	Write(b []byte) (int, error)
	// Request-scoped values (middleware passes data to the next handler).
	// String-only: the edge/wasm implementation is backed by a fixed-size,
	// string-only store (webtyp/context) with no room for arbitrary types.
	// A value that needs richer shape travels as JSON in one of these strings.
	SetValue(key, value string)
	Value(key string) string

	// Param returns the value of a path parameter the matched route declared
	// with {name}. It returns "" when the route declares no such parameter —
	// an absent parameter is not an error, it is a handler asking for
	// something this route never had.
	//
	// Values are NOT decoded beyond the transport's own URL decoding, and they
	// are never trusted: a parameter is caller input like any other.
	Param(name string) string
	// Isomorphic cookies.
	SetCookie(c Cookie)                // writes a cookie to the response
	Cookie(name string) (Cookie, bool) // reads a cookie from the request; ok=false if not found
	// Request-scoped identity. An auth middleware records the caller;
	// handlers and mounted modules read it.
	SetUserID(id string) // records the authenticated identity (id "" = anonymous)
	UserID() string      // reads the identity; "" if no valid session

	// Decode reads the request body through the transport's codec, into a typed
	// destination — the handler never imports a codec package directly. Decode
	// backed by JSON, jsvalue, or any other model.FieldReader implementation is
	// the transport's decision, not the handler's.
	Decode(into model.Decodable) error
	// Encode writes v through the transport's codec as the response body.
	// Same contract as Decode, the other direction.
	Encode(v model.Encodable) error
}

// HandlerFunc is the dispatch unit: receives a Context and responds to it.
type HandlerFunc func(Context)

// Streamer is a Context that also flushes writes immediately.
// Used for incremental responses (SSE, streaming).
//
// Ownership: same single-goroutine contract as Context. A push loop
// (SSE hub, broker) must deliver messages to the handler's goroutine
// via a channel; only that goroutine calls Write/Flush.
type Streamer interface {
	Context
	Flush() // sends to the client what has been written so far, without closing the response
}

// StreamFunc is a handler that receives a typed Streamer.
type StreamFunc func(Streamer)

// Socket is the bidirectional upgraded connection (WebSocket).
// Isomorphic abstraction: does not touch concrete upgrade mechanisms.
type Socket interface {
	Read() ([]byte, error)
	Write(b []byte) error
	Close() error
}

// SocketFunc is a handler that receives a typed Socket.
type SocketFunc func(Socket)

// Middleware wraps a handler to add cross-cutting logic (auth, logging).
// Operate ONLY on Context — never on concrete transport types.
type Middleware func(HandlerFunc) HandlerFunc

// Router is what a module registers its routes on.
// A concrete implementer (native server, edge runtime) satisfies this interface;
// modules and hosts only consume it.
type Router interface {
	Get(path string, h HandlerFunc) Route
	Post(path string, h HandlerFunc) Route
	Put(path string, h HandlerFunc) Route
	Delete(path string, h HandlerFunc) Route
	Options(path string, h HandlerFunc) Route
	Handle(method, path string, h HandlerFunc) Route
	Stream(path string, h StreamFunc) Route
	Socket(path string, h SocketFunc) Route

	// PublicAsset registers ONE route serving ONE file to the browser: generated
	// content such as index.html, the stylesheet, the JS bundle or the wasm binary.
	//
	// It is public by construction — a browser fetching an asset has no identity
	// yet. It returns no Route: there is no permission to attach, so an asset can
	// neither be left private by accident (a silent 403 on a blank page) nor be
	// wrongly gated. Serving a file that DOES need permissions is a normal route:
	// Get(path, h).Requires(resource, action) — which fails closed if forgotten.
	PublicAsset(path string, h HandlerFunc)

	// PublicDir serves a whole directory under a prefix (e.g. "web/public").
	// Same contract as PublicAsset: public by construction, no Route to gate.
	PublicDir(prefix string, dir string)

	Use(m ...Middleware)
	// Routes enumerates the registered routes and their metadata.
	Routes() []RouteInfo
}

// APIModule is a module that exposes a server API.
// It is consumed by the server entry point (!wasm): which passes it the host's Router,
// and the module registers its own routes/handlers. Since Router is isomorphic,
// the module never imports net/http to describe its API. The concrete transport
// (binary upload, another protocol mounted as a route) is the module's internal decision.
type APIModule interface {
	model.ModuleNaming // provides ModelName() — identity
	MountAPI(r Router)
}

// OpRegistry is the transport-neutral surface a reusable domain module registers its
// operations on. It carries ONLY named operations — no HTTP verb, no path — so one
// module description projects onto whatever transport the host binds: mcp harvests
// each Op as a tool; a future REST/gRPC/stdio binding would map the name its own way.
//
// It is the mount-side MIRROR of Caller (the call-side, also transport-neutral):
// Caller.Call(name, args, into, done) invokes exactly what Op(name, h) registered.
// Both live here, next to Context and Route, because that is what an Op handler needs.
//
// It is deliberately NOT a method on Router. Router is the HTTP-shaped surface
// (Get/Post/path/cookies/status); a transport that only harvests operations (mcp)
// must never be forced to impersonate an HTTP router — panicking on Get/Post it can
// neither honour nor need — just to be handed the operation list. A concrete HTTP
// router MAY also satisfy OpRegistry, but the domain module sees only this.
type OpRegistry interface {
	// Op registers an operation by LOGICAL NAME. Route.Accepts declares its arg schema
	// (what a transport advertising a catalogue, like mcp's tools/list, reads); the
	// same Route.Requires/Public/Authenticated gate applies as to any HTTP route.
	Op(name string, h HandlerFunc) Route
}

// OpModule is a reusable domain module: it exposes named operations and NOTHING
// transport-specific. Where APIModule registers HTTP routes and is therefore bound to
// an HTTP host, an OpModule depends only on this package's neutral contracts, so the
// SAME module serves any transport the composition root chooses to bind (mcp tools
// today). "Is this module transport-agnostic?" is a compile-time fact — whether it
// satisfies OpModule — not a convention to remember.
type OpModule interface {
	model.ModuleNaming // provides ModelName() — identity
	MountOps(reg OpRegistry)
}
