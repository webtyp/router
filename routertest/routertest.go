// Package routertest holds test helpers for apps that register routes on a
// router.Router.
package routertest

import (
	"testing"
	"webtyp.com/router"
)

const (
	msgUnexpectedPublic = "unexpected public route (add it to the inventory only if it must be reachable with no identity): "
	msgMissingPublic    = "declared public route is not registered: "
	msgDuplicateWant    = "public route listed twice in the inventory: "
)

// AssertPublicRoutes fails t unless the routes r reports as public are exactly
// want, each written "METHOD /path" (for PublicDir, "GET /prefix/" as Routes()
// reports it). Order does not matter. Every unexpected public route and every
// declared-but-missing one is reported, one t.Errorf per entry, so a reviewer
// sees the whole drift in one run.
func AssertPublicRoutes(t testing.TB, r router.Router, want ...string) {
	t.Helper()

	wantList := make([]string, 0, len(want))
	seenWant := make([]bool, len(want))

	// Check for duplicates in want
	for i, w := range want {
		duplicate := false
		for j := 0; j < i; j++ {
			if want[j] == w {
				duplicate = true
				break
			}
		}
		if duplicate {
			t.Errorf("%s%s", msgDuplicateWant, w)
		} else {
			wantList = append(wantList, w)
		}
	}

	routes := r.Routes()
	var gotPublic []string

	for _, info := range routes {
		if info.IsPublic() {
			gotPublic = append(gotPublic, info.Method+" "+info.Path)
		}
	}

	for _, got := range gotPublic {
		found := false
		for i, w := range wantList {
			if got == w {
				found = true
				seenWant[i] = true
				break
			}
		}
		if !found {
			t.Errorf("%s%s", msgUnexpectedPublic, got)
		}
	}

	for i, w := range wantList {
		if !seenWant[i] {
			t.Errorf("%s%s", msgMissingPublic, w)
		}
	}
}
