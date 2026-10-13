package router_test

import (
	"testing"
	"webtyp.com/fmt"
	"webtyp.com/router"
	"webtyp.com/router/mock"
	"webtyp.com/router/routertest"
)

type recordingTB struct {
	testing.TB
	errors []string
}

func (r *recordingTB) Helper() {}

func (r *recordingTB) Errorf(format string, args ...any) {
	r.errors = append(r.errors, fmt.Sprintf(format, args...))
}

func TestRoutertestAssertPublicRoutesExact(t *testing.T) {
	r := &mock.Router{}
	r.Get("/a", func(ctx router.Context) {}).Public()
	r.Post("/b", func(ctx router.Context) {}).Public()
	r.Get("/c", func(ctx router.Context) {}).Requires("x", 1) // Not public

	tb := &recordingTB{TB: t}
	routertest.AssertPublicRoutes(tb, r, "POST /b", "GET /a")
	if len(tb.errors) > 0 {
		t.Errorf("exact match should not report errors: %v", tb.errors)
	}
}

func TestRoutertestAssertPublicRoutesUnexpected(t *testing.T) {
	r := &mock.Router{}
	r.Get("/a", func(ctx router.Context) {}).Public()
	r.Post("/b", func(ctx router.Context) {}).Public()

	tb := &recordingTB{TB: t}
	routertest.AssertPublicRoutes(tb, r, "GET /a")
	if len(tb.errors) != 1 || tb.errors[0] != "unexpected public route (add it to the inventory only if it must be reachable with no identity): POST /b" {
		t.Errorf("unexpected error output: %v", tb.errors)
	}
}

func TestRoutertestAssertPublicRoutesMissing(t *testing.T) {
	r := &mock.Router{}
	r.Get("/a", func(ctx router.Context) {}).Public()

	tb := &recordingTB{TB: t}
	routertest.AssertPublicRoutes(tb, r, "GET /a", "POST /b")
	if len(tb.errors) != 1 || tb.errors[0] != "declared public route is not registered: POST /b" {
		t.Errorf("unexpected error output: %v", tb.errors)
	}
}

func TestRoutertestAssertPublicRoutesDuplicate(t *testing.T) {
	r := &mock.Router{}
	r.Get("/a", func(ctx router.Context) {}).Public()

	tb := &recordingTB{TB: t}
	routertest.AssertPublicRoutes(tb, r, "GET /a", "GET /a")

	// Expect one duplicate error
	var hasDup bool
	for _, e := range tb.errors {
		if e == "public route listed twice in the inventory: GET /a" {
			hasDup = true
		}
	}
	if !hasDup {
		t.Errorf("expected duplicate want error, got %v", tb.errors)
	}
}
