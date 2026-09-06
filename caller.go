package router

import "webtyp.com/model"

// Caller is how a client-side view invokes a named server operation without
// knowing the wire protocol or transport. It mirrors APIModule: APIModule is
// the mount-side contract, Caller is the call-side contract.
//
// op is the logical operation name (e.g. "list_services") — NEVER a wire-level
// method. Translating op to the concrete envelope is the adapter's job (e.g.
// mcp.NewCaller adapts *mcp.Client). Test doubles satisfy Caller with canned
// results and no transport.
type Caller interface {
	// Call invokes op and DECODES the response into `into` using the transport's
	// codec — the call-side mirror of Context.Decode/Encode on the mount side. A
	// caller (a view, a module) works in typed model values and NEVER imports a
	// codec package (json, jsvalue); which codec runs is the transport's decision.
	//
	// into may be nil when the caller does not care about the response body (a
	// save/delete that only needs the error). done reports the outcome; result/err
	// arrive asynchronously (works for wasm fetch and for in-process test doubles
	// alike). Implementations MUST propagate every error — never swallow.
	Call(op string, args model.Encodable, into model.Decodable, done func(err error))

	// Dispatch is fire-and-forget (no response expected).
	Dispatch(op string, args model.Encodable)
}
