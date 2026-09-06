// Package conformance is the executable contract of router.Router.
//
// The interface states the SIGNATURES; this package states the BEHAVIOUR. Two
// implementations can satisfy the interface — compile without a complaint — and disagree on
// everything that matters. That is not hypothetical: it is what happened. One registered
// routes by path alone (so two methods on one path panicked), the other ran its access gate
// before establishing identity (so every guarded route was a permanent 403). Both compiled.
// Both were wrong. Nothing caught it, because the only implementation under test was the
// mock — the one nobody deploys.
//
// A compiler cannot catch "these two implementations behave differently": both satisfy the
// interface. So it has to become something that goes red. This is that something.
//
// An implementation proves conformance from its own test package:
//
//	func TestHTTPDConformance(t *testing.T) {
//	    conformance.Run(t, conformance.Factory{New: newHTTPD, Verify: verifyStartup})
//	}
//
// The shape (exported suite, parameterized by a factory, one t.Run per clause) is the one
// the Go team uses for the same job: golang.org/x/net/nettest.TestConn for net.Conn, and
// testing/fstest.TestFS for fs.FS. This package importing "testing" in non-test code is
// deliberate and is the whole point — a _test.go file cannot be imported by another repo.
package conformance

import (
	"testing"

	"webtyp.com/model"
	"webtyp.com/router"
)

// Resource is the resource the suite guards its routes with.
const Resource model.Resource = "conformance"

// Action is the permission the suite's guarded routes require.
const Action = model.Update

// The identities the suite drives through ServeFunc.
const (
	// UserAuthorized holds Action on Resource: a guarded route must SERVE it.
	UserAuthorized = "conformance-authorized"
	// UserUnauthorized is a valid identity WITHOUT the permission: a guarded route
	// must reject it — authentication is not authorization.
	UserUnauthorized = "conformance-unauthorized"
	// Anonymous is no identity at all.
	Anonymous = ""
)

// Authorize is the authorizer the suite drives its cases with. An implementation MUST wire
// the one handed to it in Setup — it is how the suite tells an authorized caller from a
// merely authenticated one.
func Authorize(userID string, r model.Resource, a model.Action) bool {
	return userID == UserAuthorized && r == Resource && a.Has(Action)
}

// Setup is what the suite hands the implementation so it can be built the way the suite
// needs to drive it.
type Setup struct {
	// Authorize answers whether an identity holds a permission. The implementation MUST
	// install it as its authorizer. Wiring something else — or nothing — makes the
	// authorization cases meaningless.
	Authorize model.Authorizer
}

// Response is what came back from the transport. It is deliberately not an http.Response:
// the contract is isomorphic, and one of its implementations does not run on a server.
type Response struct {
	Status int
	Body   []byte
}

// echoPayload is the minimal model.Decodable+model.Encodable fixture used to prove
// Context.Decode/Encode round-trip through the transport's real codec, instead of the
// suite hand-parsing bytes — that would test the suite's parser, not the implementation's.
type echoPayload struct{ Value string }

func (e *echoPayload) IsNil() bool                      { return e == nil }
func (e *echoPayload) Schema() []model.Field            { return nil } // fixture: no validation exercised here
func (e *echoPayload) Pointers() []any                  { return []any{&e.Value} }
func (e *echoPayload) EncodeFields(w model.FieldWriter) { w.String("value", e.Value) }
func (e *echoPayload) DecodeFields(r model.FieldReader) {
	if v, ok := r.String("value"); ok {
		e.Value = v
	}
}

var _ model.Fielder = (*echoPayload)(nil)

// echoPayloadJSON is the wire form of echoPayload — Context is byte-oriented (Body/Write),
// and every deployed transport (httpd, edge) carries JSON on that wire today.
const echoPayloadJSON = `{"value":"conformance"}`

// extractEchoValue reads the "value" string back out of an echoPayloadJSON-shaped response.
// Hand-written, not a codec import: conformance stays codec-agnostic in production code, and
// this parses exactly the one fixed shape this fixture ever produces.
func extractEchoValue(body []byte) string {
	const prefix = `{"value":"`
	s := string(body)
	if len(s) <= len(prefix)+1 || s[:len(prefix)] != prefix {
		return ""
	}
	rest := s[len(prefix):]
	end := 0
	for end < len(rest) && rest[end] != '"' {
		end++
	}
	return rest[:end]
}

// ServeFunc drives ONE request through the Router under test and reports what came back.
//
// It must go through the implementation's REAL pipeline — identity, access gate,
// middleware, handler. A ServeFunc that calls the matched handler directly proves the
// opposite of what it claims to: the gate is precisely what is under test.
//
// userID is the identity the transport reports for this caller ("" = anonymous). The
// implementation routes it through whatever its authentication seam is, the same one a
// production caller would go through.
type ServeFunc func(method, path string, body []byte, userID string) Response

// Factory builds a fresh Router plus the driver that sends a request through it.
type Factory struct {
	// New returns an EMPTY Router and the ServeFunc that drives it. It is called once per
	// case, so no case can be polluted by another's routes.
	New func(t *testing.T, s Setup) (router.Router, ServeFunc)

	// Verify reports the error the implementation raises AT STARTUP for the routes
	// registered so far — nil meaning "this configuration is legal".
	//
	// Optional. An implementation that cannot fail at startup leaves it nil, and the
	// contradiction case skips with a loud reason instead of passing quietly.
	Verify func(r router.Router) error

	// ServeOp drives ONE request through a route registered via OperationRegistry.Operation(name, h) — the
	// provider-side counterpart of router.Caller.Call(name, args, cb). It receives the SAME
	// Router New built, so registration (by the clause) and invocation (by this func) share
	// one instance; name is the op name the clause registered.
	//
	// Optional. An implementation that does not yet implement Operation leaves this nil, and the
	// Operation clauses skip with a loud reason instead of failing to compile.
	ServeOp func(r router.Router, name string, body []byte, userID string) Response
}

// Run executes every clause of the contract against the implementation.
func Run(t *testing.T, f Factory) {
	if f.New == nil {
		t.Fatal("conformance: Factory.New is required")
	}

	t.Run("same_path_different_methods", func(t *testing.T) { samePathDifferentMethods(t, f) })
	t.Run("unregistered_method_is_405", func(t *testing.T) { unregisteredMethodIs405(t, f) })
	t.Run("unmatched_path_is_404", func(t *testing.T) { unmatchedPathIs404(t, f) })
	t.Run("subtree_pattern_serves_descendants", func(t *testing.T) { subtreePatternServesDescendants(t, f) })

	t.Run("undeclared_route_fails_at_startup", func(t *testing.T) { undeclaredRouteFailsAtStartup(t, f) })
	t.Run("public_route_serves_anonymous", func(t *testing.T) { publicServesAnonymous(t, f) })
	t.Run("guarded_route_rejects_anonymous", func(t *testing.T) { guardedRejectsAnonymous(t, f) })
	t.Run("guarded_route_serves_authorized_identity", func(t *testing.T) { guardedServesAuthorized(t, f) })
	t.Run("guarded_route_rejects_unauthorized_identity", func(t *testing.T) { guardedRejectsUnauthorized(t, f) })
	t.Run("authenticated_route_serves_any_identity", func(t *testing.T) { authenticatedServesAnyIdentity(t, f) })

	t.Run("authn_runs_before_the_gate", func(t *testing.T) { authnRunsBeforeTheGate(t, f) })
	t.Run("middleware_wraps_the_handler", func(t *testing.T) { middlewareWrapsTheHandler(t, f) })
	t.Run("middleware_does_not_run_on_a_rejected_route", func(t *testing.T) { middlewareSkippedOnReject(t, f) })

	t.Run("routes_reports_access_and_method", func(t *testing.T) { routesReportsAccessAndMethod(t, f) })

	t.Run("body_survives_binary_roundtrip", func(t *testing.T) { bodySurvivesBinaryRoundtrip(t, f) })
	t.Run("body_is_stable_across_reads", func(t *testing.T) { bodyIsStableAcrossReads(t, f) })

	t.Run("context_decodes_and_encodes_typed_payload", func(t *testing.T) { contextDecodesAndEncodesTypedPayload(t, f) })

	t.Run("op_route_reports_args_schema", func(t *testing.T) { opRouteReportsArgsSchema(t, f) })
	t.Run("op_route_is_invoked_by_name", func(t *testing.T) { opRouteIsInvokedByName(t, f) })
	t.Run("op_route_enforces_rbac", func(t *testing.T) { opRouteEnforcesRBAC(t, f) })

	t.Run("path_parameter_is_extracted", func(t *testing.T) { pathParameterIsExtracted(t, f) })
	t.Run("two_parameters_in_one_pattern", func(t *testing.T) { twoParametersInOnePattern(t, f) })
	t.Run("parameter_does_not_match_across_a_separator", func(t *testing.T) { parameterDoesNotMatchAcrossSeparator(t, f) })
	t.Run("parameter_does_not_match_an_empty_segment", func(t *testing.T) { parameterDoesNotMatchEmptySegment(t, f) })
	t.Run("literal_beats_parameter", func(t *testing.T) { literalBeatsParameter(t, f) })
	t.Run("unknown_parameter_reads_empty", func(t *testing.T) { unknownParameterReadsEmpty(t, f) })
	t.Run("parameters_do_not_leak_into_value", func(t *testing.T) { parametersDoNotLeakIntoValue(t, f) })

	t.Run("contradictory_route_fails_at_startup", func(t *testing.T) { contradictoryRouteFailsAtStartup(t, f) })
}

// build is the per-case constructor: a fresh Router, its driver, and the suite's authorizer
// already wired in.
func build(t *testing.T, f Factory) (router.Router, ServeFunc) {
	t.Helper()
	r, serve := f.New(t, Setup{Authorize: Authorize})
	if r == nil || serve == nil {
		t.Fatal("conformance: Factory.New returned a nil Router or ServeFunc")
	}
	return r, serve
}

// ok is the handler the suite registers when it only cares that the route was reached.
// It writes a marker so a case can tell "the handler ran" from "something answered 200".
func ok(marker string) router.HandlerFunc {
	return func(ctx router.Context) {
		ctx.WriteStatus(200)
		ctx.Write([]byte(marker))
	}
}

const testPath = "/conformance"

// --- method routing ---------------------------------------------------------------------

// samePathDifferentMethods is the clause that server/httpd broke: it registered by path
// alone, so three methods on one path were the SAME pattern and ServeMux panicked. The
// contract offers Get/Post/Options on any path; an implementation that cannot honour that
// is not implementing this interface.
func samePathDifferentMethods(t *testing.T, f Factory) {
	r, serve := build(t, f)

	r.Get(testPath, ok("get")).Public()
	r.Post(testPath, ok("post")).Public()
	r.Options(testPath, ok("options")).Public()

	for _, c := range []struct{ method, want string }{
		{"GET", "get"}, {"POST", "post"}, {"OPTIONS", "options"},
	} {
		got := serve(c.method, testPath, nil, Anonymous)
		if got.Status != 200 {
			t.Errorf("the three routes on the same path must coexist: registering by path alone collapses them; %s got %d", c.method, got.Status)
			continue
		}
		if string(got.Body) != c.want {
			t.Errorf("%s reached the wrong handler: got %q, want %q", c.method, got.Body, c.want)
		}
	}
}

func unregisteredMethodIs405(t *testing.T, f Factory) {
	r, serve := build(t, f)
	r.Get(testPath, ok("get")).Public()

	if got := serve("POST", testPath, nil, Anonymous); got.Status != 405 {
		t.Errorf("a method nobody registered on an existing path is 405, got %d", got.Status)
	}
}

// subtreePatternServesDescendants: una ruta terminada en "/" cubre su subárbol, y
// entre varias que encajan gana la más específica.
//
// El servidor de desarrollo del framework sirve el sitio entero bajo "/" y
// resuelve cada petición contra su propio índice de activos: sin subárbol
// tendría que registrar una ruta por archivo en el instante del arranque, que
// es justo lo que congelaba el sitio en la foto previa al escaneo de módulos.
//
// Estaba sin cubrir, y las implementaciones divergieron en silencio: httpd lo
// soportaba porque http.ServeMux lo trae de fábrica, y el mock comparaba rutas
// por igualdad exacta. Un consumidor que probara contra el mock afirmaba un
// comportamiento que producción no tenía, o al revés.
func subtreePatternServesDescendants(t *testing.T, f Factory) {
	r, serve := build(t, f)
	r.Get("/", ok("subtree")).Public()
	r.Get(testPath, ok("exact")).Public()

	for _, c := range []struct{ path, want string }{
		{"/", "subtree"},
		{"/img/logo.svg", "subtree"},
		{"/a/b/c/deep.txt", "subtree"},
		{testPath, "exact"},
	} {
		got := serve("GET", c.path, nil, Anonymous)
		if got.Status != 200 {
			t.Errorf("%s debe resolverse: got %d", c.path, got.Status)
			continue
		}
		if string(got.Body) != c.want {
			t.Errorf("%s llegó al handler equivocado: got %q, want %q", c.path, got.Body, c.want)
		}
	}
}

func unmatchedPathIs404(t *testing.T, f Factory) {
	r, serve := build(t, f)
	r.Get(testPath, ok("get")).Public()

	if got := serve("GET", "/nothing-here", nil, Anonymous); got.Status != 404 {
		t.Errorf("an unmatched path is 404, got %d", got.Status)
	}
}

// --- access: closed by default ----------------------------------------------------------

// undeclaredRouteFailsAtStartup: the zero value of model.Access is AccessGuarded, so a route
// that annotates nothing is unreachable. But "unreachable" is not good enough — a silent 403
// on a route somebody forgot to declare is a bug that surfaces in production, on a Friday.
// RouteInfo.Access says it outright: an enforcer must reject it LOUDLY AT STARTUP.
//
// So this is not a runtime clause. Denying at runtime is the failure mode, not the contract.
func undeclaredRouteFailsAtStartup(t *testing.T, f Factory) {
	if f.Verify == nil {
		t.Skip("implementation cannot fail at startup; an undeclared route will deny silently")
	}

	r, _ := f.New(t, Setup{Authorize: Authorize})
	r.Get(testPath, ok("get")) // declares neither Public, nor Requires, nor Authenticated

	if err := f.Verify(r); err == nil {
		t.Error("a route that declares no access must fail at startup: it is unreachable, and a silent 403 is how that gets discovered in production")
	}
}

func publicServesAnonymous(t *testing.T, f Factory) {
	r, serve := build(t, f)
	r.Get(testPath, ok("get")).Public()

	got := serve("GET", testPath, nil, Anonymous)
	if got.Status != 200 || string(got.Body) != "get" {
		t.Errorf("a Public route serves a caller with no identity: got %d %q", got.Status, got.Body)
	}
}

func guardedRejectsAnonymous(t *testing.T, f Factory) {
	r, serve := build(t, f)
	r.Get(testPath, ok("get")).Requires(Resource, Action)

	if got := serve("GET", testPath, nil, Anonymous); got.Status != 403 {
		t.Errorf("a guarded route rejects an anonymous caller with 403, got %d", got.Status)
	}
}

// guardedServesAuthorized is the clause goflare/edge could not pass: its gate ran before
// any middleware, so no caller could ever be identified and EVERY guarded route was a
// permanent 403. goflare/files mounts its upload as a guarded route — which made the whole
// file API unusable in production while its own tests stayed green.
func guardedServesAuthorized(t *testing.T, f Factory) {
	r, serve := build(t, f)
	r.Get(testPath, ok("get")).Requires(Resource, Action)

	got := serve("GET", testPath, nil, UserAuthorized)
	if got.Status != 200 {
		t.Errorf("a guarded route SERVES an identity the authorizer grants: got %d, want 200 (a gate that runs before identity makes this impossible)", got.Status)
		return
	}
	if string(got.Body) != "get" {
		t.Errorf("the guarded route answered 200 without running its handler: body %q", got.Body)
	}
}

// guardedRejectsUnauthorized: authentication is not authorization.
func guardedRejectsUnauthorized(t *testing.T, f Factory) {
	r, serve := build(t, f)
	r.Get(testPath, ok("get")).Requires(Resource, Action)

	if got := serve("GET", testPath, nil, UserUnauthorized); got.Status != 403 {
		t.Errorf("a valid identity WITHOUT the permission is rejected with 403, got %d", got.Status)
	}
}

func authenticatedServesAnyIdentity(t *testing.T, f Factory) {
	r, serve := build(t, f)
	r.Get(testPath, ok("get")).Authenticated()

	// Any identity, even one no authorizer grants anything to.
	if got := serve("GET", testPath, nil, UserUnauthorized); got.Status != 200 {
		t.Errorf("an Authenticated route serves ANY identity, no permission checked: got %d", got.Status)
	}
	if got := serve("GET", testPath, nil, Anonymous); got.Status != 403 {
		t.Errorf("an Authenticated route still rejects an anonymous caller: got %d, want 403", got.Status)
	}
}

// --- identity and middleware ------------------------------------------------------------

// authnRunsBeforeTheGate: identity must be established BEFORE access is decided. Get this
// backwards and the failure is not a wrong answer, it is a dead end — no caller can ever be
// authorized, on any route, ever. It is the bug that cost goflare its file API.
//
// The route is Public so the gate cannot be what serves it: what is under test is that the
// identity reached the handler at all.
func authnRunsBeforeTheGate(t *testing.T, f Factory) {
	r, serve := build(t, f)

	r.Get(testPath, func(ctx router.Context) {
		ctx.WriteStatus(200)
		ctx.Write([]byte(ctx.UserID()))
	}).Public()

	got := serve("GET", testPath, nil, UserAuthorized)
	if string(got.Body) != UserAuthorized {
		t.Errorf("the gate ran before identity was established: no caller can ever be authorized; handler saw UserID %q, want %q", got.Body, UserAuthorized)
	}
}

// middlewareWrapsTheHandler: Use applies to routes registered BEFORE it too — otherwise
// registration order silently decides which routes are covered by an audit middleware.
func middlewareWrapsTheHandler(t *testing.T, f Factory) {
	r, serve := build(t, f)

	r.Get(testPath, ok("handler")).Public()

	r.Use(func(next router.HandlerFunc) router.HandlerFunc {
		return func(ctx router.Context) {
			ctx.SetHeader("X-Conformance-Middleware", "1")
			next(ctx)
		}
	})

	got := serve("GET", testPath, nil, Anonymous)
	if got.Status != 200 || string(got.Body) != "handler" {
		t.Errorf("middleware must wrap the handler, not replace it: got %d %q", got.Status, got.Body)
	}
}

// middlewareSkippedOnReject: a rejected request must not run business logic. Middleware
// behind the gate is where logging, decoding and DB work live; running it on a 403 leaks
// work — and sometimes data — to a caller who was just denied.
func middlewareSkippedOnReject(t *testing.T, f Factory) {
	r, serve := build(t, f)

	ran := false
	handlerRan := false

	r.Get(testPath, func(ctx router.Context) {
		handlerRan = true
		ctx.WriteStatus(200)
	}).Requires(Resource, Action)

	r.Use(func(next router.HandlerFunc) router.HandlerFunc {
		return func(ctx router.Context) {
			ran = true
			next(ctx)
		}
	})

	if got := serve("GET", testPath, nil, Anonymous); got.Status != 403 {
		t.Fatalf("setup: expected the guarded route to reject an anonymous caller, got %d", got.Status)
	}
	if handlerRan {
		t.Error("a rejected request ran the handler")
	}
	if ran {
		t.Error("a rejected request ran business middleware: work behind the gate must not execute on a 403")
	}
}

// --- introspection ----------------------------------------------------------------------

func routesReportsAccessAndMethod(t *testing.T, f Factory) {
	r, _ := build(t, f)

	r.Get(testPath, ok("get")).Public()
	r.Post(testPath, ok("post")).Requires(Resource, Action)

	infos := r.Routes()
	if len(infos) != 2 {
		t.Fatalf("Routes() reports one entry per registered route: got %d, want 2", len(infos))
	}

	var sawPublicGet, sawGuardedPost bool
	for _, i := range infos {
		if i.Path != testPath {
			t.Errorf("Routes() reported the wrong path: %q", i.Path)
		}
		switch i.Method {
		case "GET":
			sawPublicGet = i.Access == model.AccessPublic
		case "POST":
			sawGuardedPost = i.Access == model.AccessGuarded && i.Resource == Resource && i.Action.Has(Action)
		}
	}
	if !sawPublicGet {
		t.Error("Routes() must report the GET route as AccessPublic")
	}
	if !sawGuardedPost {
		t.Error("Routes() must report the POST route as AccessGuarded, with its Resource and Action")
	}
}

// --- body -------------------------------------------------------------------------------

// binaryBody has a NUL, a high byte and a PNG magic number: anything that round-trips the
// body through a string, a UTF-8 validation or a text codec corrupts it here. This is the
// clause that decides whether an image can be uploaded at all.
var binaryBody = []byte{0x00, 0xFF, 0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x80, 0x7F}

func bodySurvivesBinaryRoundtrip(t *testing.T, f Factory) {
	r, serve := build(t, f)

	r.Put(testPath, func(ctx router.Context) {
		ctx.WriteStatus(200)
		ctx.Write(ctx.Body())
	}).Public()

	got := serve("PUT", testPath, binaryBody, Anonymous)
	if got.Status != 200 {
		t.Fatalf("setup: expected 200, got %d", got.Status)
	}
	if !sameBytes(got.Body, binaryBody) {
		t.Errorf("the body must survive byte for byte: got %v, want %v (a body that passes through a string is corrupted here)", got.Body, binaryBody)
	}
}

// bodyIsStableAcrossReads: Body() may be lazy, but it must not be destructive. A middleware
// that looks at the body must not consume it out from under the handler.
func bodyIsStableAcrossReads(t *testing.T, f Factory) {
	r, serve := build(t, f)

	r.Put(testPath, func(ctx router.Context) {
		first := ctx.Body()
		second := ctx.Body()
		if !sameBytes(first, second) {
			ctx.WriteStatus(500)
			return
		}
		ctx.WriteStatus(200)
		ctx.Write(second)
	}).Public()

	got := serve("PUT", testPath, binaryBody, Anonymous)
	if got.Status != 200 {
		t.Fatalf("reading Body() twice returned different bytes: it is lazy, but it must not be destructive (status %d)", got.Status)
	}
	if !sameBytes(got.Body, binaryBody) {
		t.Errorf("the second read must return the same bytes: got %v, want %v", got.Body, binaryBody)
	}
}

// contextDecodesAndEncodesTypedPayload: the handler reads args and writes its result through
// the transport's typed codec (Decode/Encode) — never a hand-rolled json.Decode in the module.
// Context is byte-oriented (Body/Write), so the fixture travels as the JSON every deployed
// transport already carries on that wire; what is under test is that Decode/Encode round-trip
// through it, not that the suite can parse bytes.
func contextDecodesAndEncodesTypedPayload(t *testing.T, f Factory) {
	r, serve := build(t, f)

	r.Put(testPath, func(ctx router.Context) {
		var in echoPayload
		if err := ctx.Decode(&in); err != nil {
			ctx.WriteStatus(500)
			return
		}
		ctx.WriteStatus(200)
		if err := ctx.Encode(&in); err != nil {
			ctx.WriteStatus(500)
		}
	}).Public()

	got := serve("PUT", testPath, []byte(echoPayloadJSON), Anonymous)
	if got.Status != 200 {
		t.Fatalf("Decode/Encode round-trip failed: status %d, body %s", got.Status, got.Body)
	}
	if v := extractEchoValue(got.Body); v != "conformance" {
		t.Errorf("Decode/Encode did not round-trip the typed value: got %q, want %q (body: %s)", v, "conformance", got.Body)
	}
}

// --- operation: provider-side dispatch by logical name -----------------------------------------

// opReg asserts the OperationRegistry surface off the Router the Factory built. Operation is NOT a
// method on Router (that would force an op-only transport like mcp to impersonate an
// HTTP router); a concrete HTTP router MAY also satisfy OperationRegistry, and these clauses
// skip loudly if it does not.
func opReg(t *testing.T, r router.Router) router.OperationRegistry {
	reg, isOp := r.(router.OperationRegistry)
	if !isOp {
		t.Skip("router does not implement OperationRegistry")
	}
	return reg
}

// opRouteReportsArgsSchema: Accepts is the counterpart of RouteInfo.Args — a transport that
// needs a schema (mcp's tools/list) reads it from Routes(), never from a module hand-rolling
// wire metadata. Pure introspection: no ServeOp needed.
func opRouteReportsArgsSchema(t *testing.T, f Factory) {
	r, _ := build(t, f)

	args := &echoPayload{}
	opReg(t, r).Operation("with_args", ok("op")).Public().Accepts(args)

	infos := r.Routes()
	for _, i := range infos {
		if i.Args == args {
			return
		}
	}
	t.Errorf("Routes() must report the Args declared via Accepts for an Operation route, got: %+v", infos)
}

// opRouteIsInvokedByName: a module registers by NAME (OperationRegistry.Operation), never a path — the
// provider-side symmetric to Caller.Call(name, args, into, done). This is what lets one
// router.OperationModule serve any transport (mcp tools today) without knowing it.
func opRouteIsInvokedByName(t *testing.T, f Factory) {
	if f.ServeOp == nil {
		t.Skip("implementation does not support Operation yet")
	}
	r, _ := build(t, f)

	opReg(t, r).Operation("do_thing", ok("op-ran")).Public()

	got := f.ServeOp(r, "do_thing", nil, Anonymous)
	if got.Status != 200 || string(got.Body) != "op-ran" {
		t.Errorf("Operation route was not invoked by name: got %d %q", got.Status, got.Body)
	}
}

// opRouteEnforcesRBAC: an Operation route is a route — the SAME access gate applies. A module that
// switches from Post(path,...) to Operation(name,...) must not silently lose its RBAC.
func opRouteEnforcesRBAC(t *testing.T, f Factory) {
	if f.ServeOp == nil {
		t.Skip("implementation does not support Operation yet")
	}
	r, _ := build(t, f)

	opReg(t, r).Operation("guarded_thing", ok("op-ran")).Requires(Resource, Action)

	if got := f.ServeOp(r, "guarded_thing", nil, Anonymous); got.Status != 403 {
		t.Errorf("a guarded Operation route rejects an anonymous caller with 403, got %d", got.Status)
	}
	if got := f.ServeOp(r, "guarded_thing", nil, UserAuthorized); got.Status != 200 {
		t.Errorf("a guarded Operation route serves an identity the authorizer grants: got %d, want 200", got.Status)
	}
}

// --- path parameters --------------------------------------------------------------------

func pathParameterIsExtracted(t *testing.T, f Factory) {
	r, serve := build(t, f)
	r.Get("/api/items/{id}", func(ctx router.Context) {
		ctx.WriteStatus(200)
		ctx.Write([]byte(ctx.Param("id")))
	}).Public()

	got := serve("GET", "/api/items/42", nil, Anonymous)
	if got.Status != 200 || string(got.Body) != "42" {
		t.Errorf("path parameter was not extracted: got %d %q, want 200 %q", got.Status, got.Body, "42")
	}
}

func twoParametersInOnePattern(t *testing.T, f Factory) {
	r, serve := build(t, f)
	r.Get("/api/sites/{site}/pages/{page}", func(ctx router.Context) {
		ctx.WriteStatus(200)
		ctx.Write([]byte(ctx.Param("site") + ":" + ctx.Param("page")))
	}).Public()

	got := serve("GET", "/api/sites/a/pages/b", nil, Anonymous)
	if got.Status != 200 || string(got.Body) != "a:b" {
		t.Errorf("two parameters in one pattern failed: got %d %q, want 200 %q", got.Status, got.Body, "a:b")
	}
}

func parameterDoesNotMatchAcrossSeparator(t *testing.T, f Factory) {
	r, serve := build(t, f)
	r.Get("/api/items/{id}", ok("item")).Public()

	if got := serve("GET", "/api/items/42/extra", nil, Anonymous); got.Status != 404 {
		t.Errorf("parameter matched across a separator: got %d, want 404", got.Status)
	}
}

func parameterDoesNotMatchEmptySegment(t *testing.T, f Factory) {
	r, serve := build(t, f)
	r.Get("/api/items/{id}", ok("item")).Public()

	if got := serve("GET", "/api/items/", nil, Anonymous); got.Status != 404 {
		t.Errorf("parameter matched an empty segment: got %d, want 404", got.Status)
	}
}

func literalBeatsParameter(t *testing.T, f Factory) {
	r, serve := build(t, f)
	r.Get("/api/items/{id}", ok("param")).Public()
	r.Get("/api/items/new", ok("literal")).Public()

	got := serve("GET", "/api/items/new", nil, Anonymous)
	if got.Status != 200 || string(got.Body) != "literal" {
		t.Errorf("literal did not beat parameter: got %d %q, want 200 %q", got.Status, got.Body, "literal")
	}
}

func unknownParameterReadsEmpty(t *testing.T, f Factory) {
	r, serve := build(t, f)
	r.Get("/api/items/{id}", func(ctx router.Context) {
		ctx.WriteStatus(200)
		ctx.Write([]byte(ctx.Param("nope")))
	}).Public()

	got := serve("GET", "/api/items/42", nil, Anonymous)
	if got.Status != 200 || string(got.Body) != "" {
		t.Errorf("unknown parameter did not read empty: got %d %q, want 200 %q", got.Status, got.Body, "")
	}
}

func parametersDoNotLeakIntoValue(t *testing.T, f Factory) {
	r, serve := build(t, f)
	r.Get("/api/items/{id}", func(ctx router.Context) {
		ctx.WriteStatus(200)
		ctx.Write([]byte(ctx.Value("id")))
	}).Public()

	got := serve("GET", "/api/items/42", nil, Anonymous)
	if got.Status != 200 || string(got.Body) != "" {
		t.Errorf("parameter leaked into Context.Value: got %d %q, want 200 %q", got.Status, got.Body, "")
	}
}

// --- contradictions ---------------------------------------------------------------------

// contradictoryRouteFailsAtStartup: a guarded route with no authorizer denies EVERY caller,
// forever. It looks protected and is in fact a brick. That must be a loud startup error, not
// a silent 403 discovered in production.
func contradictoryRouteFailsAtStartup(t *testing.T, f Factory) {
	if f.Verify == nil {
		t.Skip("implementation cannot fail at startup; contradictions will deny silently")
	}

	// A router built WITHOUT the suite's authorizer, carrying a guarded route.
	r, _ := f.New(t, Setup{Authorize: nil})
	r.Get(testPath, ok("get")).Requires(Resource, Action)

	if err := f.Verify(r); err == nil {
		t.Error("a guarded route with no authorizer configured must fail at startup: it would deny every caller, on a route that looks protected")
	}
}

// sameBytes compares two byte slices. Written out because this package compiles to WASM and
// the standard library is off limits here — see the repo's AGENTS.md.
func sameBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
