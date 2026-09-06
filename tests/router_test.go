package router_test

import (
	"testing"

	"webtyp.com/model"
	"webtyp.com/router"
)

// fakeContext prueba que cualquier tipo implementando Context se escribe guiado
// por la interfaz — sin fallbacks a `net/http`.
type fakeContext struct {
	values  map[string]string
	cookies map[string]router.Cookie
	userID  string
}

func (f *fakeContext) Method() string              { return "GET" }
func (f *fakeContext) Path() string                { return "/" }
func (f *fakeContext) Body() []byte                { return nil }
func (f *fakeContext) GetHeader(key string) string { return "" }
func (f *fakeContext) SetHeader(key, value string) {}
func (f *fakeContext) WriteStatus(code int)        {}
func (f *fakeContext) Write(b []byte) (int, error) { return len(b), nil }
func (f *fakeContext) SetValue(key, value string) {
	if f.values == nil {
		f.values = make(map[string]string)
	}
	f.values[key] = value
}
func (f *fakeContext) Value(key string) string {
	if f.values == nil {
		return ""
	}
	return f.values[key]
}
func (f *fakeContext) Param(name string) string {
	return ""
}
func (f *fakeContext) SetCookie(c router.Cookie) {
	if f.cookies == nil {
		f.cookies = make(map[string]router.Cookie)
	}
	f.cookies[c.Name] = c
}
func (f *fakeContext) Cookie(name string) (router.Cookie, bool) {
	if f.cookies == nil {
		return router.Cookie{}, false
	}
	c, ok := f.cookies[name]
	return c, ok
}
func (f *fakeContext) SetUserID(id string)               { f.userID = id }
func (f *fakeContext) UserID() string                    { return f.userID }
func (f *fakeContext) Decode(into model.Decodable) error { return nil }
func (f *fakeContext) Encode(v model.Encodable) error    { return nil }

var _ router.Context = (*fakeContext)(nil)

// fakeStreamer prueba que Streamer se escribe con Flush().
type fakeStreamer struct{ *fakeContext }

func (f *fakeStreamer) Flush() {}

var _ router.Streamer = (*fakeStreamer)(nil)

// fakeSocket prueba que Socket se escribe tipado.
type fakeSocket struct{}

func (f *fakeSocket) Read() ([]byte, error) { return nil, nil }
func (f *fakeSocket) Write(b []byte) error  { return nil }
func (f *fakeSocket) Close() error          { return nil }

var _ router.Socket = (*fakeSocket)(nil)

// fakeRouter prueba que Router registra rutas tipadas con metadatos.
type fakeRouter struct {
	routes     map[string]router.HandlerFunc
	registered []router.RouteInfo
}

func (f *fakeRouter) registerRoute(method, path string) router.Route {
	if f.routes == nil {
		f.routes = make(map[string]router.HandlerFunc)
	}
	r := &fakeRoute{info: router.RouteInfo{Method: method, Path: path}}
	return r
}

func (f *fakeRouter) Get(path string, h router.HandlerFunc) router.Route {
	r := f.registerRoute("GET", path)
	f.routes[path] = h
	return r
}

func (f *fakeRouter) PublicAsset(path string, h router.HandlerFunc) {
	f.registerRoute("GET", path)
	f.routes[path] = h
}

func (f *fakeRouter) PublicDir(prefix string, dir string) {
	f.registerRoute("GET", prefix)
}

func (f *fakeRouter) Post(path string, h router.HandlerFunc) router.Route {
	r := f.registerRoute("POST", path)
	f.routes[path] = h
	return r
}

func (f *fakeRouter) Put(path string, h router.HandlerFunc) router.Route {
	r := f.registerRoute("PUT", path)
	f.routes[path] = h
	return r
}

func (f *fakeRouter) Delete(path string, h router.HandlerFunc) router.Route {
	r := f.registerRoute("DELETE", path)
	f.routes[path] = h
	return r
}

func (f *fakeRouter) Options(path string, h router.HandlerFunc) router.Route {
	r := f.registerRoute("OPTIONS", path)
	f.routes[path] = h
	return r
}

func (f *fakeRouter) Handle(method, path string, h router.HandlerFunc) router.Route {
	r := f.registerRoute(method, path)
	f.routes[path] = h
	return r
}

func (f *fakeRouter) Stream(path string, h router.StreamFunc) router.Route {
	return f.registerRoute("GET", path)
}

func (f *fakeRouter) Socket(path string, h router.SocketFunc) router.Route {
	return f.registerRoute("GET", path)
}

func (f *fakeRouter) Op(name string, h router.HandlerFunc) router.Route {
	r := f.registerRoute("OP", "/"+name)
	f.routes["/"+name] = h
	return r
}

func (f *fakeRouter) Use(m ...router.Middleware) {
}

func (f *fakeRouter) Routes() []router.RouteInfo {
	return f.registered
}

var _ router.Router = (*fakeRouter)(nil)

// fakeRoute implementa Route para grabar anotaciones de permiso.
type fakeRoute struct {
	info router.RouteInfo
}

func (r *fakeRoute) Requires(resource model.Resource, action model.Action) router.Route {
	r.info.Access = model.AccessGuarded
	r.info.Resource = resource
	r.info.Action = action
	return r
}

func (r *fakeRoute) Authenticated() router.Route {
	r.info.Access = model.AccessAuthenticated
	return r
}

func (r *fakeRoute) Public() router.Route {
	r.info.Access = model.AccessPublic
	return r
}

func (r *fakeRoute) Accepts(args model.Fielder) router.Route {
	r.info.Args = args
	return r
}

var _ router.Route = (*fakeRoute)(nil)

// fakeModule prueba que APIModule embebe ModuleNaming sin tocar tipos de transporte.
type fakeModule struct{ name string }

func (f fakeModule) ModelName() string        { return f.name }
func (f fakeModule) MountAPI(r router.Router) {}

var _ model.ModuleNaming = fakeModule{}
var _ router.APIModule = fakeModule{}

// TestRouterContracts verifica que los contratos se escriben tipados.
func TestRouterContracts(t *testing.T) {
	var ctx router.Context = &fakeContext{}
	if ctx.Path() != "/" {
		t.Fatalf("Context.Path() failed")
	}

	var stream router.Streamer = &fakeStreamer{&fakeContext{}}
	stream.Flush()

	var sock router.Socket = &fakeSocket{}
	sock.Close()

	var r router.Router = &fakeRouter{}
	r.Get("/api", func(ctx router.Context) {})

	var m router.APIModule = fakeModule{name: "test"}
	if m.ModelName() != "test" {
		t.Fatalf("APIModule.ModelName() = %q, want %q", m.ModelName(), "test")
	}
	m.MountAPI(r)
}

// TestCookies verifica ida y vuelta de cookies.
func TestCookies(t *testing.T) {
	ctx := &fakeContext{}

	// Escribe una cookie
	cookie := router.Cookie{
		Name:     "sid",
		Value:    "xyz123",
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
		SameSite: router.SameSiteLax,
	}
	ctx.SetCookie(cookie)

	// Lee la cookie
	read, ok := ctx.Cookie("sid")
	if !ok {
		t.Fatalf("Cookie(\"sid\") should exist")
	}
	if read.Name != "sid" || read.Value != "xyz123" {
		t.Fatalf("Cookie mismatch: got %+v, want %+v", read, cookie)
	}
	if read.Secure != true || read.HttpOnly != true {
		t.Fatalf("Cookie flags mismatch: Secure=%v, HttpOnly=%v", read.Secure, read.HttpOnly)
	}

	// Cookie no existente
	_, ok = ctx.Cookie("nonexistent")
	if ok {
		t.Fatalf("Cookie(\"nonexistent\") should not exist")
	}
}

// TestRouteMetadata verifica que Requires() registra metadatos de permiso.
func TestRouteMetadata(t *testing.T) {
	r := &fakeRouter{}

	// Registra una ruta con metadatos de permiso
	route := r.Post("/orders", func(ctx router.Context) {})
	// "write" no existe: los verbos son CRUD. Antes esto compilaba y denegaba en silencio.
	route.Requires("orders", model.Update)

	// Verifica que la ruta está anotada (en fakeRoute)
	if fakeRoute, ok := route.(*fakeRoute); ok {
		if fakeRoute.info.Resource != "orders" || fakeRoute.info.Action != model.Update {
			t.Fatalf("Route metadata mismatch: got Resource=%q, Action=%d", fakeRoute.info.Resource, fakeRoute.info.Action)
		}
	}
}

// TestUserID verifica ida y vuelta de SetUserID/UserID.
func TestUserID(t *testing.T) {
	ctx := &fakeContext{}

	// Por defecto vacío
	if ctx.UserID() != "" {
		t.Fatalf("UserID() should be empty by default, got %q", ctx.UserID())
	}

	// Setea un ID
	ctx.SetUserID("u123")
	if ctx.UserID() != "u123" {
		t.Fatalf("UserID() mismatch: got %q, want %q", ctx.UserID(), "u123")
	}

	// Setea anónimo
	ctx.SetUserID("")
	if ctx.UserID() != "" {
		t.Fatalf("UserID() should be empty after SetUserID(\"\")")
	}
}

// TestPublicRoute verifica que Public() marca la ruta como pública.
func TestPublicRoute(t *testing.T) {
	r := &fakeRouter{}

	// Registra una ruta pública
	route := r.Get("/public", func(ctx router.Context) {})
	route.Public()

	// Verifica el marcador
	if fakeRoute, ok := route.(*fakeRoute); ok {
		if !fakeRoute.info.IsPublic() {
			t.Fatalf("Route should be public")
		}
	}
}

// TestSameSiteConstants verifica que SameSite está bien tipado.
func TestSameSiteConstants(t *testing.T) {
	tests := []struct {
		name  string
		value router.SameSite
	}{
		{"Default", router.SameSiteDefault},
		{"Lax", router.SameSiteLax},
		{"Strict", router.SameSiteStrict},
		{"None", router.SameSiteNone},
	}

	for _, tt := range tests {
		if int(tt.value) < 0 || int(tt.value) > 3 {
			t.Fatalf("SameSite %s out of range: %d", tt.name, tt.value)
		}
	}
}
