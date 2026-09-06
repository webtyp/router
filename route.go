package router

import "webtyp.com/model"

// Route describes a registered route and allows annotating it. It is returned by
// each Router registration method. Annotations are declarative: the contract does
// not enforce them — each concrete implementer (native server, edge runtime) enforces them.
type Route interface {
	// Requires binds an RBAC permission to the route: the (resource, action) pair.
	//
	// Both are typed (model.Resource, model.Action). They used to be two bare strings in a
	// row, so swapping them compiled — and the failure was not an error but a SILENT denial
	// at runtime, in the one place where silence is unacceptable. Now it does not compile.
	//
	// The resource is open vocabulary: the app declares its own ("service_catalog"). The
	// action is a closed CRUD set (model.Read, model.Update, …): persistence has four verbs
	// and no tool in this ecosystem ever needed a fifth.
	Requires(resource model.Resource, action model.Action) Route

	// Authenticated marks the route as reachable by any identity, with no permission check.
	// For operations on the CALLER themselves, where authentication already is the check.
	Authenticated() Route

	// Public marks the route as reachable with no identity at all.
	Public() Route

	// Accepts declares the typed schema of the request body — the Args a caller
	// must send. It is the counterpart of RouteInfo.Args: a transport that needs
	// to advertise a schema (mcp's tools/list) reads it from here instead of the
	// module hand-rolling wire metadata. nil means "no args" (a Route that never
	// calls Accepts has Args == nil, the same as passing nil explicitly).
	Accepts(args model.Fielder) Route
}

// RouteInfo is the read-only view of a registered route — for introspection.
type RouteInfo struct {
	Method   string         // e.g. "GET", "POST"
	Path     string         // e.g. "/api/users", "/api/orders/{id}"
	Resource model.Resource // required by AccessGuarded; must be empty otherwise
	Action   model.Action   // e.g. model.Read; 0 = none
	// Access is what the route declared. The ZERO VALUE is model.AccessGuarded: a route that
	// annotates nothing is unreachable until it declares a Resource, and an enforcer must
	// reject it loudly at startup.
	//
	// It replaced a `Public bool` alongside an empty-or-not Resource. That encoding made an
	// illegal state writable — a route could be Public AND carry a Requires, and the gate
	// silently dropped the permission check: a route that looked protected and was not.
	Access model.Access
	// Dir is the directory served by PublicDir; "" for every other route.
	// It exists so a whole served directory is visible to introspection instead of
	// being smuggled past the router by a file-server fallback.
	Dir string
	// Args is the schema a caller must send, set via Route.Accepts; nil = no args.
	// It is a Go-side value read directly by a transport that needs it (mcp builds
	// its tools/list schema from it) — deliberately NOT part of EncodeFields: a
	// Fielder's schema is a different shape of data than this route-metadata wire
	// view, and serializing it is that transport's concern, not RouteInfo's.
	Args model.Fielder
}

// IsPublic reports whether the route is reachable with no identity.
func (r RouteInfo) IsPublic() bool { return r.Access == model.AccessPublic }

// EncodeFields makes RouteInfo a model.Encodable, so it is serialized by webtyp/json
// through this DECLARED shape instead of by reflection over its Go fields.
//
// Reflection got it actively wrong, and wrong in the worst direction. `Access` and
// `Action` are numeric types, so a reflection-based encoder emitted them as bare numbers:
// the ZERO value of Access is AccessGuarded, so the MOST protected route in the server
// reported itself as `"Access":0` — which any human or agent reading the routes endpoint
// takes for "nothing declared", the exact opposite of the truth. `"Action":6` was equally
// unreadable. An endpoint whose whole job is to expose the security posture of a server
// must not invert it.
//
// Here the shape is stated, not guessed: the enums travel as the words they already know
// how to render, and `Dir` — an internal detail of PublicDir — stays out of the wire.
func (r RouteInfo) EncodeFields(w model.FieldWriter) {
	w.String("method", r.Method)
	w.String("path", r.Path)
	w.String("resource", string(r.Resource))
	w.String("action", r.Action.String()) // "ru", never 6
	w.String("access", r.Access.String()) // "guarded", never 0
}

// IsNil satisfies model.Encodable; a RouteInfo is a value and never nil.
func (r RouteInfo) IsNil() bool { return false }
