package router_test

import (
	"testing"

	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/router/mock"
)

func newMountRouter() *mock.Router {
	r := &mock.Router{}
	r.Configure(mock.Config{
		Authn: func(next router.HandlerFunc) router.HandlerFunc {
			return func(ctx router.Context) {
				if id := ctx.GetHeader("X-Mock-User"); id != "" {
					ctx.SetUserID(id)
				}
				next(ctx)
			}
		},
		Authorize: func(userID string, res model.Resource, act model.Action) bool {
			return userID == "authorized" && res == "items" && act.Has(model.Read)
		},
	})
	return r
}

func serveMount(r *mock.Router, method, path, userID string) (int, []byte) {
	ctx := &mock.Context{InMethod: method, InPath: path}
	if userID != "" {
		ctx.SetHeader("X-Mock-User", userID)
	}
	r.Invoke(method, path, ctx)
	return ctx.Status, ctx.ResponseBody()
}

func okMount(marker string) router.HandlerFunc {
	return func(ctx router.Context) {
		ctx.WriteStatus(200)
		ctx.Write([]byte(marker))
	}
}

// TestMountJoinsPrefix: a mounted route appears in Routes() with the joined
// path, exactly as if registered directly.
func TestMountJoinsPrefix(t *testing.T) {
	r := newMountRouter()
	r.Mount("/api/auth", func(m router.Router) {
		m.Get("/logout", okMount("logout")).Public()
	})

	infos := r.Routes()
	if len(infos) != 1 {
		t.Fatalf("Routes() = %d entries, want 1", len(infos))
	}
	if infos[0].Method != "GET" || infos[0].Path != "/api/auth/logout" {
		t.Fatalf("Routes()[0] = %s %s, want GET /api/auth/logout", infos[0].Method, infos[0].Path)
	}
	if infos[0].Access != model.AccessPublic {
		t.Fatalf("mounted route Access = %v, want public", infos[0].Access)
	}

	status, body := serveMount(r, "GET", "/api/auth/logout", "")
	if status != 200 || string(body) != "logout" {
		t.Fatalf("mounted route serves %d %q, want 200 %q", status, body, "logout")
	}
}

// TestMountNested: nested Mount composes, prefixes concatenate in order.
func TestMountNested(t *testing.T) {
	r := newMountRouter()
	r.Mount("/api", func(m router.Router) {
		m.Mount("/auth", func(n router.Router) {
			n.Post("/logout", okMount("logout")).Public()
		})
	})

	infos := r.Routes()
	if len(infos) != 1 {
		t.Fatalf("Routes() = %d entries, want 1", len(infos))
	}
	if infos[0].Method != "POST" || infos[0].Path != "/api/auth/logout" {
		t.Fatalf("Routes()[0] = %s %s, want POST /api/auth/logout", infos[0].Method, infos[0].Path)
	}

	status, body := serveMount(r, "POST", "/api/auth/logout", "")
	if status != 200 || string(body) != "logout" {
		t.Fatalf("nested mounted route serves %d %q, want 200 %q", status, body, "logout")
	}
}

// TestMountPreservesAccess: .Public() and .Requires(...) on a mounted route
// behave identically to a direct registration.
func TestMountPreservesAccess(t *testing.T) {
	r := newMountRouter()
	r.Mount("/api", func(m router.Router) {
		m.Get("/open", okMount("open")).Public()
		m.Get("/closed", okMount("closed")).Requires("items", model.Read)
	})
	r.Get("/direct", okMount("closed")).Requires("items", model.Read)

	if status, _ := serveMount(r, "GET", "/api/open", ""); status != 200 {
		t.Errorf("mounted Public route serves anonymous: got %d, want 200", status)
	}
	if status, _ := serveMount(r, "GET", "/api/closed", ""); status != 403 {
		t.Errorf("mounted guarded route rejects anonymous: got %d, want 403", status)
	}
	if status, body := serveMount(r, "GET", "/api/closed", "authorized"); status != 200 || string(body) != "closed" {
		t.Errorf("mounted guarded route serves authorized: got %d %q, want 200 %q", status, body, "closed")
	}
	if status, _ := serveMount(r, "GET", "/api/closed", "other"); status != 403 {
		t.Errorf("mounted guarded route rejects unauthorized identity: got %d, want 403", status)
	}
	if status, _ := serveMount(r, "GET", "/direct", ""); status != 403 {
		t.Errorf("direct guarded route rejects anonymous: got %d, want 403", status)
	}
}

// TestMountInvalidPrefixPanics: a prefix that does not begin with "/", or
// that ends with "/", is a programming error caught at registration.
func TestMountInvalidPrefixPanics(t *testing.T) {
	for _, prefix := range []string{"api/auth", "/api/auth/", "/"} {
		func() {
			defer func() {
				rec := recover()
				if rec != router.ErrMsgMountPrefix {
					t.Errorf("Mount(%q) panicked with %v, want %q", prefix, rec, router.ErrMsgMountPrefix)
				}
			}()
			r := newMountRouter()
			r.Mount(prefix, func(m router.Router) {
				m.Get("/x", okMount("x")).Public()
			})
			t.Errorf("Mount(%q) did not panic", prefix)
		}()
	}
}
