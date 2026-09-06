package security

import (
	"testing"

	"webtyp.com/fmt"
	"webtyp.com/router"
	"webtyp.com/router/mock"
)

// servePolicy drives one request through the policy middleware and reports
// the response headers, status, and whether the handler ran.
func servePolicy(p Policy, body []byte, proto string) (headers map[string]string, status int, ran bool) {
	r := &mock.Router{}
	r.Get("/x", func(ctx router.Context) {
		ran = true
		ctx.WriteStatus(200)
	}).Public()
	r.Use(p.Middleware())

	ctx := &mock.Context{InMethod: "GET", InPath: "/x", InBody: body}
	if proto != "" {
		ctx.SetHeader("X-Forwarded-Proto", proto)
	}
	r.Invoke("GET", "/x", ctx)

	headers = map[string]string{
		HeaderContentSecurityPolicy:   ctx.GetHeader(HeaderContentSecurityPolicy),
		HeaderXContentTypeOptions:     ctx.GetHeader(HeaderXContentTypeOptions),
		HeaderReferrerPolicy:          ctx.GetHeader(HeaderReferrerPolicy),
		HeaderPermissionsPolicy:       ctx.GetHeader(HeaderPermissionsPolicy),
		HeaderStrictTransportSecurity: ctx.GetHeader(HeaderStrictTransportSecurity),
		HeaderXFrameOptions:           ctx.GetHeader(HeaderXFrameOptions),
	}
	return headers, ctx.Status, ran
}

// TestPolicyDefaults: Policy{} emits all six headers with the documented values.
func TestPolicyDefaults(t *testing.T) {
	headers, status, ran := servePolicy(Policy{}, nil, "https")
	if status != 200 || !ran {
		t.Fatalf("handler ran = %v, status = %d; want true, 200", ran, status)
	}
	want := map[string]string{
		HeaderContentSecurityPolicy:   DefaultContentSecurityPolicy,
		HeaderXContentTypeOptions:     DefaultXContentTypeOptions,
		HeaderReferrerPolicy:          DefaultReferrerPolicy,
		HeaderPermissionsPolicy:       DefaultPermissionsPolicy,
		HeaderStrictTransportSecurity: DefaultStrictTransportSecurity,
		HeaderXFrameOptions:           DefaultXFrameOptions,
	}
	for header, value := range want {
		if headers[header] != value {
			t.Errorf("%s = %q, want %q", header, headers[header], value)
		}
	}
}

// TestPolicyNoTLS: over a non-TLS request HSTS is absent, the other five present.
func TestPolicyNoTLS(t *testing.T) {
	headers, status, ran := servePolicy(Policy{}, nil, "")
	if status != 200 || !ran {
		t.Fatalf("handler ran = %v, status = %d; want true, 200", ran, status)
	}
	if headers[HeaderStrictTransportSecurity] != "" {
		t.Errorf("HSTS over plain HTTP = %q, want absent", headers[HeaderStrictTransportSecurity])
	}
	for _, header := range []string{
		HeaderContentSecurityPolicy,
		HeaderXContentTypeOptions,
		HeaderReferrerPolicy,
		HeaderPermissionsPolicy,
		HeaderXFrameOptions,
	} {
		if headers[header] == "" {
			t.Errorf("%s absent over plain HTTP, want present", header)
		}
	}
}

// TestAllowImages: the allowance joins the directive's base sources; every
// other directive is unchanged.
func TestAllowImages(t *testing.T) {
	headers, _, _ := servePolicy(Policy{}.AllowImages("https://x.test"), nil, "https")
	csp := headers[HeaderContentSecurityPolicy]
	if !fmt.Contains(csp, "img-src 'self' data: https://x.test") {
		t.Errorf("img-src = %q, want 'self', data: and the new origin", csp)
	}
	for _, directive := range []string{
		"default-src 'self';",
		"script-src 'self' 'wasm-unsafe-eval';",
		"style-src 'self';",
		"connect-src 'self';",
		"font-src 'self';",
		"object-src 'none';",
		"base-uri 'none';",
		"form-action 'self';",
		"frame-ancestors 'none'",
	} {
		if !fmt.Contains(csp, directive) {
			t.Errorf("CSP = %q, want unchanged directive %q", csp, directive)
		}
	}
}

// TestAllowancesAccumulate: chained allowances add to their own directives
// without overwriting each other.
func TestAllowancesAccumulate(t *testing.T) {
	p := Policy{}.AllowImages("https://img.test").AllowScripts("https://js.test").AllowImages("https://img2.test")
	headers, _, _ := servePolicy(p, nil, "https")
	csp := headers[HeaderContentSecurityPolicy]
	if !fmt.Contains(csp, "img-src 'self' data: https://img.test https://img2.test") {
		t.Errorf("img-src allowances did not accumulate: %q", csp)
	}
	if !fmt.Contains(csp, "script-src 'self' 'wasm-unsafe-eval' https://js.test") {
		t.Errorf("script-src allowance lost beside img-src: %q", csp)
	}
}

// TestAllowFrameAncestors: frame-ancestors and X-Frame-Options move together,
// so the two never disagree with the stricter one silently winning.
func TestAllowFrameAncestors(t *testing.T) {
	headers, _, _ := servePolicy(Policy{}.AllowFrameAncestors("https://x.test"), nil, "https")
	csp := headers[HeaderContentSecurityPolicy]
	if !fmt.Contains(csp, "frame-ancestors 'self' https://x.test") {
		t.Errorf("frame-ancestors = %q, want 'self' and the new origin", csp)
	}
	if headers[HeaderXFrameOptions] == DefaultXFrameOptions {
		t.Errorf("X-Frame-Options = %q, want lifted beside frame-ancestors", headers[HeaderXFrameOptions])
	}
	if headers[HeaderXFrameOptions] != "SAMEORIGIN" {
		t.Errorf("X-Frame-Options = %q, want SAMEORIGIN", headers[HeaderXFrameOptions])
	}
}

// TestBodyOverLimit: a body past the cap gets 413 and the handler never runs.
func TestBodyOverLimit(t *testing.T) {
	_, status, ran := servePolicy(Policy{}, make([]byte, DefaultMaxRequestBytes+1), "https")
	if status != 413 {
		t.Errorf("status = %d, want 413", status)
	}
	if ran {
		t.Error("handler ran on a body past the cap")
	}
}

// TestBodyRaisedLimit: MaxRequestBytes lets a larger body reach the handler.
func TestBodyRaisedLimit(t *testing.T) {
	body := make([]byte, 5<<20)
	_, status, ran := servePolicy(Policy{}.MaxRequestBytes(10<<20), body, "https")
	if !ran {
		t.Error("handler did not run below the raised cap")
	}
	if status != 200 {
		t.Errorf("status = %d, want 200", status)
	}
}

// TestCSPKeepsWasmUnsafeEval: removing 'wasm-unsafe-eval' breaks every WASM
// page in the ecosystem. This is the regression guard.
func TestCSPKeepsWasmUnsafeEval(t *testing.T) {
	if !fmt.Contains(DefaultContentSecurityPolicy, "'wasm-unsafe-eval'") {
		t.Errorf("DefaultContentSecurityPolicy = %q, want 'wasm-unsafe-eval'", DefaultContentSecurityPolicy)
	}
}
