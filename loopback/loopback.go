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
func New(mods ...router.OperationModule) router.Caller {
	reg := &registry{}
	for _, m := range mods {
		if m != nil {
			m.MountOperations(reg)
		}
	}
	return &caller{reg: reg}
}

// registry implements router.OperationRegistry, storing name→handler in a
// linear-scanned slice (no map: this compiles into the WASM binary of the
// consuming app). It returns a noopRoute stub because the modules chain
// .Requires().Accepts() on the Route returned by Operation.
type registry struct {
	ops []opEntry
}

type opEntry struct {
	name string
	h    router.HandlerFunc
}

func (r *registry) Operation(name string, h router.HandlerFunc) router.Route {
	r.ops = append(r.ops, opEntry{name: name, h: h})
	return noopRoute{}
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
// Accepts, Public, Authenticated) return self and discard the input. The
// loopback caller runs in-process against a trusting composition root; the
// operations' access metadata is irrelevant to it. It exists so real modules
// that chain .Requires(...).Accepts(...) compile and run unchanged.
type noopRoute struct{}

func (noopRoute) Requires(_ model.Resource, _ model.Action) router.Route { return noopRoute{} }
func (noopRoute) Authenticated() router.Route                            { return noopRoute{} }
func (noopRoute) Public() router.Route                                   { return noopRoute{} }
func (noopRoute) Accepts(_ model.Fielder) router.Route                   { return noopRoute{} }

var _ router.Route = noopRoute{}

// caller invokes registered handlers synchronously against an inCtx and
// reports the outcome through the async Caller contract.
type caller struct {
	reg *registry
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
	if err := json.Encode(args, &buf); err != nil {
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
