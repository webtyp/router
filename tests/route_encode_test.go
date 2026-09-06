package router_test

import (
	"strings"
	"testing"

	"webtyp.com/json"
	"webtyp.com/model"
	"webtyp.com/router"
)

// RouteInfo must serialize through its DECLARED shape, never by reflection over its Go
// fields — because reflection got it wrong in the worst possible direction.
//
// Access and Action are numeric types, so a reflection-based encoder emitted them as bare
// numbers. The ZERO value of Access is AccessGuarded, so the MOST protected route in a
// server reported itself as `"Access":0` — which any human or agent reading an
// introspection endpoint takes for "nothing declared". The output inverted the very thing
// it existed to report.
func TestRouteInfoEncodesThePostureAsWords(t *testing.T) {
	guarded := router.RouteInfo{
		Method:   "POST",
		Path:     "/api/svc",
		Resource: "service_catalog",
		Action:   model.Read | model.Update,
		Access:   model.AccessGuarded, // the zero value
	}

	var out string
	if err := json.Encode(guarded, &out); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out, `"access":"guarded"`) {
		t.Errorf("access is not reported as a word: %s", out)
	}
	if strings.Contains(out, `"access":0`) {
		t.Errorf("the most protected route is reported as 0, which reads as 'unset': %s", out)
	}
	if !strings.Contains(out, `"action":"ru"`) {
		t.Errorf("action is not reported as CRUD letters: %s", out)
	}
	if strings.Contains(out, `"action":6`) {
		t.Errorf("action leaked as a raw bitmask: %s", out)
	}

	// Dir is an internal detail of PublicDir and has no business on the wire.
	if strings.Contains(strings.ToLower(out), `"dir"`) {
		t.Errorf("Dir leaked into the wire: %s", out)
	}
}

func TestRouteInfoEncodesPublicAndAuthenticated(t *testing.T) {
	cases := []struct {
		access model.Access
		want   string
	}{
		{model.AccessPublic, `"access":"public"`},
		{model.AccessAuthenticated, `"access":"authenticated"`},
	}

	for _, c := range cases {
		var out string
		if err := json.Encode(router.RouteInfo{Method: "GET", Path: "/x", Access: c.access}, &out); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out, c.want) {
			t.Errorf("expected %s, got %s", c.want, out)
		}
	}
}
