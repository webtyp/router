package router_test

import (
	"testing"

	"webtyp.com/router"
)

func TestPatternNames(t *testing.T) {
	tests := []struct {
		pattern string
		want    []string
	}{
		{"/api/items", nil},
		{"/api/items/{id}", []string{"id"}},
		{"/s/{a}/p/{b}", []string{"a", "b"}},
	}

	for _, tt := range tests {
		got := router.ParamNames(tt.pattern)
		if len(got) != len(tt.want) {
			t.Errorf("ParamNames(%q) = %v, want %v", tt.pattern, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("ParamNames(%q)[%d] = %q, want %q", tt.pattern, i, got[i], tt.want[i])
			}
		}
	}
}

func TestValidatePattern(t *testing.T) {
	tests := []struct {
		pattern string
		wantErr string
	}{
		{"/api/items", ""},
		{"/api/items/{id}", ""},
		{"/x/{a...}", `router: pattern "/x/{a...}": {name...} wildcards are not supported`},
		{"/x/{}", `router: pattern "/x/{}": empty parameter name`},
		{"/x/{a}/y/{a}", `router: pattern "/x/{a}/y/{a}": duplicate parameter name "a"`},
		{"/v{n}", `router: pattern "/v{n}": segment "v{n}" mixes literal text and a parameter`},
	}

	for _, tt := range tests {
		err := router.ValidatePattern(tt.pattern)
		if tt.wantErr == "" {
			if err != nil {
				t.Errorf("ValidatePattern(%q) unexpected error: %v", tt.pattern, err)
			}
		} else {
			if err == nil || err.Error() != tt.wantErr {
				t.Errorf("ValidatePattern(%q) = %v, want %q", tt.pattern, err, tt.wantErr)
			}
		}
	}
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		wantOK  bool
		wantVal []string
	}{
		{"/api/items", "/api/items", true, nil},
		{"/api/items", "/api/items/1", false, nil},
		{"/api/items/", "/api/items/1/2", true, nil},
		{"/api/items/{id}", "/api/items/42", true, []string{"42"}},
		{"/api/items/{id}", "/api/items/42/x", false, nil},
		{"/api/items/{id}", "/api/items/", false, nil},
		{"/api/items/{id}", "/api/items", false, nil},
		{"/s/{a}/p/{b}", "/s/x/p/y", true, []string{"x", "y"}},
	}

	for _, tt := range tests {
		gotVal, gotOK := router.MatchPattern(tt.pattern, tt.path)
		if gotOK != tt.wantOK {
			t.Errorf("MatchPattern(%q, %q) ok = %v, want %v", tt.pattern, tt.path, gotOK, tt.wantOK)
			continue
		}
		if gotOK {
			if len(gotVal) != len(tt.wantVal) {
				t.Errorf("MatchPattern(%q, %q) values = %v, want %v", tt.pattern, tt.path, gotVal, tt.wantVal)
				continue
			}
			for i := range gotVal {
				if gotVal[i] != tt.wantVal[i] {
					t.Errorf("MatchPattern(%q, %q) values[%d] = %q, want %q", tt.pattern, tt.path, i, gotVal[i], tt.wantVal[i])
				}
			}
		}
	}
}

func TestMoreSpecific(t *testing.T) {
	// 1. Literal beats parameter
	if !router.MoreSpecific("/api/sites/new", "/api/sites/{id}") {
		t.Errorf("literal should beat parameter")
	}
	if router.MoreSpecific("/api/sites/{id}", "/api/sites/new") {
		t.Errorf("parameter should not beat literal")
	}

	// 2. More segments wins
	if !router.MoreSpecific("/api/sites/new/extra", "/api/sites/new") {
		t.Errorf("more segments should win")
	}

	// 3. Longer pattern string wins
	if !router.MoreSpecific("/api/sites/long-name", "/api/sites/short") {
		t.Errorf("longer string should win")
	}

	// 4. Equal pattern comparison
	if router.MoreSpecific("/api/items", "/api/items") {
		t.Errorf("same pattern should not be more specific than itself")
	}
}
