package router_test

import (
	"testing"

	"webtyp.com/fmt"
	"webtyp.com/router"
)

func TestQueryParam(t *testing.T) {
	tests := []struct {
		path   string
		key    string
		want   string
		wantOK bool
	}{
		{"/x", "a", "", false},
		{"/x?", "a", "", false},
		{"/x?a=1", "a", "1", true},
		{"/x?a=1&b=2", "b", "2", true},
		{"/x?a=1&b=2", "c", "", false},
		{"/x?a=", "a", "", true},
		{"/x?a", "a", "", true},
		{"/x?a=b=c", "a", "b=c", true},
		{"/x?s=YWJj==", "s", "YWJj==", true},
		{"/x?u=https%3A%2F%2Fm.velty.cl%2Fp", "u", "https://m.velty.cl/p", true},
		{"/x?q=a+b", "q", "a b", true},
		{"/x?a=1#b=2", "b", "", false},
		{"/x?a=1#frag", "a", "1", true},
		{"/x?a=1&a=2", "a", "1", true},
		{"/x?%61=1", "a", "1", true},
		{"/x?a=%ZZ", "a", "%ZZ", true},
		{"/x?a=100%", "a", "100%", true},
	}

	for _, tt := range tests {
		got, gotOK := router.QueryParam(tt.path, tt.key)
		if gotOK != tt.wantOK || got != tt.want {
			t.Errorf("QueryParam(%q, %q) = (%q, %v), want (%q, %v)",
				tt.path, tt.key, got, gotOK, tt.want, tt.wantOK)
		}
	}
}

func TestQueryUnescape(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"a+b", "a b"},
		{"%20", " "},
		{"%2F", "/"},
		{"%2f", "/"},
		{"%", "%"},
		{"%A", "%A"},
		{"100%25", "100%"},
	}

	for _, tt := range tests {
		got := router.QueryUnescape(tt.in)
		if got != tt.want {
			t.Errorf("QueryUnescape(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestQueryParam_ValidatesAnOAuthRedirectURI(t *testing.T) {
	path := "/oauth/google?redirect_uri=https%3A%2F%2Fmisitio.velty.cl%2Fpanel&state=YWJj=="

	redirectURI, ok1 := router.QueryParam(path, "redirect_uri")
	if !ok1 {
		t.Fatalf("QueryParam redirect_uri missing")
	}

	state, ok2 := router.QueryParam(path, "state")
	if !ok2 {
		t.Fatalf("QueryParam state missing")
	}

	if redirectURI != "https://misitio.velty.cl/panel" {
		t.Errorf("redirect_uri = %q, want %q", redirectURI, "https://misitio.velty.cl/panel")
	}

	if state != "YWJj==" {
		t.Errorf("state = %q, want %q", state, "YWJj==")
	}

	// Domain validator toy implementation matching: prefix "https://" + suffix ".velty.cl" (before path)
	validateDomain := func(uri string) bool {
		if !fmt.HasPrefix(uri, "https://") {
			return false
		}
		rest := fmt.TrimPrefix(uri, "https://")
		parts := fmt.Split(rest, "/")
		host := parts[0]
		return fmt.HasSuffix(host, ".velty.cl")
	}

	if !validateDomain(redirectURI) {
		t.Errorf("domain validator rejected valid redirect_uri: %q", redirectURI)
	}
}
