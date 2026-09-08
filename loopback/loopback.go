package loopback

import (
	"webtyp.com/fmt"
	"webtyp.com/json"
	"webtyp.com/model"
	"webtyp.com/router"
)

// New builds a router.Caller that invokes, in-process and synchronously, the
// operations the given modules registered via MountOperations. Args and results
// are encoded with webtyp/json (WASM-safe, already in the ecosystem tree), so a
// consumer keeps working with typed model values and never imports a codec.
//
// The caller trusts the composition root: same process, no transport, no RBAC.
// It is the reference in-process Caller for demos, offline apps and consumer
// tests that want to drive real domain modules without a server.
//
// WithTenant injects a machine-supplied tenant_id into operations whose
// declared Accepts schema carries a tenant_id field when the caller sends nil
// args (the same injection a real transport's middleware performs on the wire).
// In-process callers have no middleware; this option mirrors it for a
// single-tenant demo/offline app.
func New(mods ...router.OperationModule) router.Caller {
	return newCaller(nil, mods...)
}

// WithTenant returns the same in-proc caller, configured to inject the given
// tenant_id into tenant-scoped operations that receive nil args.
func WithTenant(tenantID string, mods ...router.OperationModule) router.Caller {
	return newCaller(&tenantID, mods...)
}

func newCaller(tenantID *string, mods ...router.OperationModule) router.Caller {
	reg := &registry{}
	for _, m := range mods {
		if m != nil {
			m.MountOperations(reg)
		}
	}
	return &caller{reg: reg, tenantID: tenantID}
}

// registry implements router.OperationRegistry, storing name→handler in a
// linear-scanned slice (no map: this compiles into the WASM binary of the
// consuming app). It returns a noopRoute stub because the modules chain
// .Requires().Accepts() on the Route returned by Operation.
type registry struct {
	ops []opEntry
}

type opEntry struct {
	name  string
	h     router.HandlerFunc
	accepts model.Fielder // save Accepts: needed to know if the op is tenant-scoped
}

func (r *registry) Operation(name string, h router.HandlerFunc) router.Route {
	r.ops = append(r.ops, opEntry{name: name, h: h})
	return noopRoute{reg: r, i: len(r.ops) - 1}
}

func (r *registry) find(name string) (opEntry, bool) {
	for i := range r.ops {
		if r.ops[i].name == name {
			return r.ops[i], true
		}
	}
	return opEntry{}, false
}

var _ router.OperationRegistry = (*registry)(nil)

// noopRoute is a router.Route stub whose chained annotation methods (Requires,
// Accepts, Public, Authenticated) return self and discard the input — except
// Accepts, which is recorded onto the registered entry so a tenant-injecting
// caller can know the op's schema. The loopback caller runs in-process against
// a trusting composition root; permission metadata is irrelevant to it.
type noopRoute struct {
	reg *registry
	i   int
}

func (r noopRoute) Requires(_ model.Resource, _ model.Action) router.Route { return r }
func (r noopRoute) Authenticated() router.Route                            { return r }
func (r noopRoute) Public() router.Route                                   { return r }
func (r noopRoute) Accepts(f model.Fielder) router.Route {
	r.reg.ops[r.i].accepts = f
	return r
}

var _ router.Route = noopRoute{}

// caller invokes registered handlers synchronously against an inCtx and
// reports the outcome through the async Caller contract.
type caller struct {
	reg      *registry
	tenantID *string
}

func (c *caller) Call(op string, args model.Encodable, into model.Decodable, done func(err error)) {
	entry, ok := c.reg.find(op)
	if !ok {
		if done != nil {
			done(fmt.Err("loopback", "unknown", "operation", op))
		}
		return
	}

	var buf []byte
	if args == nil || model.IsNil(args) {
		// An op with no args expects an object (handlers do ctx.Decode(&args)),
		// never "null" — json.Decode of "null" into a struct fails
		// ("expected object"). A real transport sends "{}"; so does the
		// loopback. With tenant injection on, a tenant-scoped op gets its
		// tenant_id here exactly like transport middleware would on the wire.
		if c.tenantID != nil && acceptsHasTenant(entry.accepts) {
			buf = []byte("{\"tenant_id\":\"" + *c.tenantID + "\"}")
		} else {
			buf = []byte("{}")
		}
	} else if err := json.Encode(args, &buf); err != nil {
		if done != nil {
			done(err)
		}
		return
	}

	ctx := &inCtx{body: buf}
	entry.h(ctx)

	if ctx.status >= 400 {
		if done != nil {
			done(fmt.Err(string(ctx.out)))
		}
		return
	}

	if into != nil && len(ctx.out) > 0 {
		if err := json.Decode(ctx.out, into); err != nil {
			if done != nil {
				done(err)
			}
			return
		}
	}
	if done != nil {
		done(nil)
	}
}

func (c *caller) Dispatch(op string, args model.Encodable) {
	// Fire-and-forget: a failing handler must not propagate into the caller's
	// goroutine. The outcome is retained only for tests via handler side effects.
	c.Call(op, args, nil, nil)
}

var _ router.Caller = (*caller)(nil)

// acceptsHasTenant reports whether the op's declared schema (Route.Accepts)
// carries a tenant_id field. Linear scan over a handful of fields; nil Accepts
// (ops that take no args at all) is not tenant-scoped.
func acceptsHasTenant(f model.Fielder) bool {
	if f == nil {
		return false
	}
	for _, fd := range f.Schema() {
		if fd.Name == "tenant_id" {
			return true
		}
	}
	return false
}

// inCtx is the in-memory router.Context a loopback handler receives. Nearly
// every method is trivial; Decode/Encode are backed by the real webtyp/json
// codec so the round-trip a handler performs matches production exactly.
type inCtx struct {
	body   []byte // request args, JSON-encoded
	out    []byte // response body, written by the handler
	status int
	values []fmt.KeyValue
	userID string
}

func (c *inCtx) Method() string              { return "" }
func (c *inCtx) Path() string                { return "" }
func (c *inCtx) Body() []byte                { return c.body }
func (c *inCtx) GetHeader(_ string) string   { return "" }
func (c *inCtx) SetHeader(_, _ string)       {}
func (c *inCtx) WriteStatus(code int)        { c.status = code }
func (c *inCtx) Write(b []byte) (int, error) { c.out = append(c.out, b...); return len(b), nil }
func (c *inCtx) SetValue(key, value string) {
	c.values = append(c.values, fmt.KeyValue{Key: key, Value: value})
}
func (c *inCtx) Param(_ string) string                 { return "" }
func (c *inCtx) SetCookie(_ router.Cookie)             {}
func (c *inCtx) Cookie(_ string) (router.Cookie, bool) { return router.Cookie{}, false }
func (c *inCtx) SetUserID(id string)                   { c.userID = id }
func (c *inCtx) UserID() string                        { return c.userID }

func (c *inCtx) Value(key string) string {
	for i := range c.values {
		if c.values[i].Key == key {
			return c.values[i].Value
		}
	}
	return ""
}

func (c *inCtx) Decode(into model.Decodable) error {
	return json.Decode(c.Body(), into)
}

func (c *inCtx) Encode(v model.Encodable) error {
	var out []byte
	if err := json.Encode(v, &out); err != nil {
		return err
	}
	_, err := c.Write(out)
	return err
}

var _ router.Context = (*inCtx)(nil)
