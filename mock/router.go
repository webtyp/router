package mock

import (
	"webtyp.com/fmt"
	"webtyp.com/model"
	"webtyp.com/router"
)

// Config declares WHO the caller is and WHAT they may do — the same two seams the real
// implementations expose (server/httpd, goflare/edge). The zero value is legal and means
// "no authentication, no authorizer": public routes work, and every guarded route denies.
type Config struct {
	// Authn establishes identity. It runs BEFORE the access gate: a gate that runs first
	// can never be satisfied, and every guarded route becomes a permanent 403.
	Authn router.Middleware

	// Authorize answers whether an identity holds a permission. nil DENIES.
	Authorize model.Authorizer
}

// Router is an in-memory implementation of router.Router for tests.
//
// It is NOT a recorder. It enforces the same contract the deployed implementations do —
// method routing, closed-by-default access, identity before the gate, middleware behind it
// — and it proves that by passing router/conformance, exactly like they must.
//
// It used to be a recorder: Invoke called the handler directly and Use was discarded. That
// made it a fake that could not fail, so every consumer testing against it was testing a
// fantasy — a guarded route "worked" in their tests and answered 403 in production. That is
// not a hypothetical: it is how goflare shipped an unusable file API with a green suite.
//
// Stores *Route, not RouteInfo: permission annotations (Public/Requires) are chained
// AFTER registering the route, so a copy by value taken at registration would never see
// them and the mock would assert that every route is private.
type Router struct {
	cfg         Config
	middlewares []router.Middleware
	registered  []*Route
	handlers    map[string]map[string]router.HandlerFunc // [method][path]handler
	streams     map[string]map[string]router.StreamFunc  // [method][path]handler
	sockets     map[string]map[string]router.SocketFunc  // [method][path]handler
}

// Configure installs the identity and authorization seams. Call it before dispatching.
func (r *Router) Configure(cfg Config) { r.cfg = cfg }

func (r *Router) ensureHandlers() {
	if r.handlers == nil {
		r.handlers = make(map[string]map[string]router.HandlerFunc)
		r.streams = make(map[string]map[string]router.StreamFunc)
		r.sockets = make(map[string]map[string]router.SocketFunc)
	}
}

// registerRoute creates the route WITHOUT permissions: resource and action are set by
// Requires() afterwards, chained. Passing them here were two parameters always empty.
func (r *Router) registerRoute(method, path string) *Route {
	if err := router.ValidatePattern(path); err != nil {
		panic(err)
	}
	r.ensureHandlers()
	route := &Route{
		info: router.RouteInfo{
			Method: method,
			Path:   path,
		},
	}
	r.registered = append(r.registered, route)
	return route
}

func (r *Router) Get(path string, h router.HandlerFunc) router.Route {
	route := r.registerRoute("GET", path)
	r.ensureHandlers()
	if r.handlers["GET"] == nil {
		r.handlers["GET"] = make(map[string]router.HandlerFunc)
	}
	r.handlers["GET"][path] = h
	return route
}

// PublicAsset registers a file served to the browser: public by construction,
// without Route to return — no permission to attach.
func (r *Router) PublicAsset(path string, h router.HandlerFunc) {
	route := r.registerRoute("GET", path)
	route.info.Access = model.AccessPublic
	r.ensureHandlers()
	if r.handlers["GET"] == nil {
		r.handlers["GET"] = make(map[string]router.HandlerFunc)
	}
	r.handlers["GET"][path] = h
}

// PublicDir registers a directory served under a prefix.
func (r *Router) PublicDir(prefix string, dir string) {
	route := r.registerRoute("GET", prefix)
	route.info.Access = model.AccessPublic
	route.info.Dir = dir
}

func (r *Router) Post(path string, h router.HandlerFunc) router.Route {
	route := r.registerRoute("POST", path)
	r.ensureHandlers()
	if r.handlers["POST"] == nil {
		r.handlers["POST"] = make(map[string]router.HandlerFunc)
	}
	r.handlers["POST"][path] = h
	return route
}

func (r *Router) Put(path string, h router.HandlerFunc) router.Route {
	route := r.registerRoute("PUT", path)
	r.ensureHandlers()
	if r.handlers["PUT"] == nil {
		r.handlers["PUT"] = make(map[string]router.HandlerFunc)
	}
	r.handlers["PUT"][path] = h
	return route
}

func (r *Router) Delete(path string, h router.HandlerFunc) router.Route {
	route := r.registerRoute("DELETE", path)
	r.ensureHandlers()
	if r.handlers["DELETE"] == nil {
		r.handlers["DELETE"] = make(map[string]router.HandlerFunc)
	}
	r.handlers["DELETE"][path] = h
	return route
}

func (r *Router) Options(path string, h router.HandlerFunc) router.Route {
	route := r.registerRoute("OPTIONS", path)
	r.ensureHandlers()
	if r.handlers["OPTIONS"] == nil {
		r.handlers["OPTIONS"] = make(map[string]router.HandlerFunc)
	}
	r.handlers["OPTIONS"][path] = h
	return route
}

// Operation registers a route by logical operation name — the provider-side counterpart of
// Caller.Call(name, args, cb). Internally it reuses the same method+path matching as
// every other verb, under the synthetic method "OP" and path "/"+name: an implementation
// detail this mock owns, invisible to a caller that only uses Router.Operation and Route.
func (r *Router) Operation(name string, h router.HandlerFunc) router.Route {
	path := "/" + name
	route := r.registerRoute("OP", path)
	r.ensureHandlers()
	if r.handlers["OP"] == nil {
		r.handlers["OP"] = make(map[string]router.HandlerFunc)
	}
	r.handlers["OP"][path] = h
	return route
}

func (r *Router) Handle(method, path string, h router.HandlerFunc) router.Route {
	route := r.registerRoute(method, path)
	r.ensureHandlers()
	if r.handlers[method] == nil {
		r.handlers[method] = make(map[string]router.HandlerFunc)
	}
	r.handlers[method][path] = h
	return route
}

func (r *Router) Stream(path string, h router.StreamFunc) router.Route {
	route := r.registerRoute("GET", path)
	r.ensureHandlers()
	if r.streams["GET"] == nil {
		r.streams["GET"] = make(map[string]router.StreamFunc)
	}
	r.streams["GET"][path] = h
	return route
}

func (r *Router) Socket(path string, h router.SocketFunc) router.Route {
	route := r.registerRoute("GET", path)
	r.ensureHandlers()
	if r.sockets["GET"] == nil {
		r.sockets["GET"] = make(map[string]router.SocketFunc)
	}
	r.sockets["GET"][path] = h
	return route
}

// Use registers middleware. It runs BEHIND the access gate: a rejected request must not
// execute business logic.
func (r *Router) Use(m ...router.Middleware) {
	r.middlewares = append(r.middlewares, m...)
}

// Mount registers a module's routes under a prefix. Every path the callback
// registers through the wrapper lands in this mock with the joined absolute
// path, exactly as if registered directly. Nested Mount composes.
func (r *Router) Mount(prefix string, fn func(router.Router)) {
	checkMountPrefix(prefix)
	fn(&prefixed{parent: r, prefix: prefix})
}

// Routes projects the registered routes at query time, with their permission
// annotations applied.
func (r *Router) Routes() []router.RouteInfo {
	out := make([]router.RouteInfo, 0, len(r.registered))
	for _, route := range r.registered {
		out = append(out, route.info)
	}
	return out
}

// Invoke drives one request through the FULL pipeline: identity, access gate, middleware,
// handler — the same order a deployed implementation uses.
//
// It used to call the handler directly, skipping the gate. That is why it is worth saying
// out loud: a fake that cannot reject is not testing your access control, it is hiding it.
func (r *Router) Invoke(method, path string, ctx router.Context) {
	gate := func(c router.Context) { r.gateAndServe(method, path, c) }

	// Identity FIRST. The gate decides with what Authn established; run it the other way
	// round and no caller can ever be authorized.
	if r.cfg.Authn != nil {
		gate = r.cfg.Authn(gate)
	}
	gate(ctx)
}

// gateAndServe matches the route, enforces its access, and only then runs the middleware
// chain and the handler.
func (r *Router) gateAndServe(method, path string, ctx router.Context) {
	r.ensureHandlers()

	route, h, paramValues, status := r.match(method, path)
	if route == nil {
		ctx.WriteStatus(status) // 404: no path matched — 405: the path exists, the method does not
		return
	}

	if mockCtx, ok := ctx.(*Context); ok {
		paramNames := router.ParamNames(route.info.Path)
		mockCtx.SetParams(paramNames, paramValues)
	}

	if !r.allows(route.info, ctx.UserID()) {
		ctx.WriteStatus(403)
		return
	}

	// Middleware wraps the handler and runs BEHIND the gate: a 403 executes none of it.
	// Applied in reverse so the first registered is the outermost.
	for i := len(r.middlewares) - 1; i >= 0; i-- {
		h = r.middlewares[i](h)
	}
	h(ctx)
}

// match finds the route for a method+path using router.MatchPattern and router.MoreSpecific.
func (r *Router) match(method, path string) (*Route, router.HandlerFunc, []string, int) {
	pathExists := false
	var best *Route
	var bestHandler router.HandlerFunc
	var bestVals []string

	for _, route := range r.registered {
		vals, matched := router.MatchPattern(route.info.Path, path)
		if !matched {
			continue
		}
		pathExists = true
		if route.info.Method != "" && route.info.Method != method {
			continue
		}
		if best != nil && !router.MoreSpecific(route.info.Path, best.info.Path) {
			continue
		}
		handlers, ok := r.handlers[route.info.Method]
		if !ok {
			continue
		}
		h, ok := handlers[route.info.Path]
		if !ok {
			continue
		}
		best, bestHandler, bestVals = route, h, vals
	}

	if best != nil {
		return best, bestHandler, bestVals, 200
	}
	if pathExists {
		return nil, nil, nil, 405
	}
	return nil, nil, nil, 404
}

// allows is the access gate. The zero value of Access is AccessGuarded: a route that
// declares nothing is unreachable, never accidentally open.
func (r *Router) allows(info router.RouteInfo, userID string) bool {
	switch info.Access {
	case model.AccessPublic:
		return true
	case model.AccessAuthenticated:
		return userID != ""
	default: // model.AccessGuarded — the zero value
		if userID == "" {
			return false
		}
		// model.Allowed denies when the authorizer is nil: the absence of an answer is
		// not permission.
		return model.Allowed(r.cfg.Authorize, userID, info.Resource, info.Action)
	}
}

// Verify reports the contradictions an implementation must refuse to start with. Both deny
// every caller forever, on a route that looks protected — and a silent 403 in production is
// the worst possible way to find that out.
func (r *Router) Verify() error {
	for _, route := range r.registered {
		if route.info.Access != model.AccessGuarded {
			continue
		}
		// A route that annotated nothing: AccessGuarded is the zero value, so this is a
		// declaration somebody forgot, not one they made.
		if route.info.Resource == "" {
			return fmt.Err("route", route.info.Method, route.info.Path,
				"is guarded but declares no resource: it is unreachable")
		}
		if r.cfg.Authorize == nil {
			return fmt.Err("route", route.info.Method, route.info.Path, "requires resource",
				string(route.info.Resource), "but no Authorize is configured: it would deny every caller")
		}
	}
	return nil
}

var _ router.Router = (*Router)(nil)
var _ router.OperationRegistry = (*Router)(nil)

// checkMountPrefix rejects a Mount prefix that does not begin with "/" or
// that ends with "/". Startup-time wiring — a loud failure is correct here.
func checkMountPrefix(prefix string) {
	if !fmt.HasPrefix(prefix, "/") || fmt.HasSuffix(prefix, "/") {
		panic(router.ErrMsgMountPrefix)
	}
}

// prefixed is the Router a Mount callback receives: every path registered
// through it is relative to prefix, without duplicating registration logic.
type prefixed struct {
	parent router.Router
	prefix string
}

func (p *prefixed) Get(path string, h router.HandlerFunc) router.Route {
	return p.parent.Get(p.prefix+path, h)
}

func (p *prefixed) Post(path string, h router.HandlerFunc) router.Route {
	return p.parent.Post(p.prefix+path, h)
}

func (p *prefixed) Put(path string, h router.HandlerFunc) router.Route {
	return p.parent.Put(p.prefix+path, h)
}

func (p *prefixed) Delete(path string, h router.HandlerFunc) router.Route {
	return p.parent.Delete(p.prefix+path, h)
}

func (p *prefixed) Options(path string, h router.HandlerFunc) router.Route {
	return p.parent.Options(p.prefix+path, h)
}

func (p *prefixed) Handle(method, path string, h router.HandlerFunc) router.Route {
	return p.parent.Handle(method, p.prefix+path, h)
}

func (p *prefixed) Stream(path string, h router.StreamFunc) router.Route {
	return p.parent.Stream(p.prefix+path, h)
}

func (p *prefixed) Socket(path string, h router.SocketFunc) router.Route {
	return p.parent.Socket(p.prefix+path, h)
}

func (p *prefixed) PublicAsset(path string, h router.HandlerFunc) {
	p.parent.PublicAsset(p.prefix+path, h)
}

func (p *prefixed) PublicDir(prefix string, dir string) {
	p.parent.PublicDir(p.prefix+prefix, dir)
}

func (p *prefixed) Mount(prefix string, fn func(router.Router)) {
	checkMountPrefix(prefix)
	fn(&prefixed{parent: p, prefix: prefix})
}

func (p *prefixed) Use(m ...router.Middleware) { p.parent.Use(m...) }

func (p *prefixed) Routes() []router.RouteInfo { return p.parent.Routes() }

var _ router.Router = (*prefixed)(nil)
