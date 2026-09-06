// Package security is the response security policy shared by both traffic
// implementations: webtyp.com/server/httpd (the origin) and
// webtyp.com/cloudflare/edge (the Worker at the edge). If the policy lived in
// one of them, the other would serve without it — the same application
// hardened in development and bare in production.
//
// The policy's entire surface is router.Middleware over router.Context, so it
// is reachable by both with no new repository and no new dependency.
//
// This package is imported by the edge Worker, which is compiled to WASM. It
// uses webtyp.com/fmt rather than the standard string/number/error packages,
// matching the rest of the WASM-reachable tree, and never touches the standard
// network stack.
package security

import (
	"webtyp.com/fmt"
	"webtyp.com/router"
)

// Response header names emitted by the policy.
const (
	HeaderContentSecurityPolicy   = "Content-Security-Policy"
	HeaderXContentTypeOptions     = "X-Content-Type-Options"
	HeaderReferrerPolicy          = "Referrer-Policy"
	HeaderPermissionsPolicy       = "Permissions-Policy"
	HeaderStrictTransportSecurity = "Strict-Transport-Security"
	HeaderXFrameOptions           = "X-Frame-Options"
)

// DefaultContentSecurityPolicy is the hardened Content-Security-Policy.
//
// 'wasm-unsafe-eval' is mandatory and permanent: this framework compiles Go to
// WebAssembly, and instantiating a WASM module requires it. It is NOT
// 'unsafe-eval' and does not enable JavaScript eval().
const DefaultContentSecurityPolicy = "default-src 'self'; " +
	"script-src 'self' 'wasm-unsafe-eval'; " +
	"style-src 'self'; " +
	"img-src 'self' data:; " +
	"connect-src 'self'; " +
	"font-src 'self'; " +
	"object-src 'none'; " +
	"base-uri 'none'; " +
	"form-action 'self'; " +
	"frame-ancestors 'none'"

// Remaining default header values, taken verbatim from the reviewed per-app
// policy this package replaces.
const (
	DefaultXContentTypeOptions     = "nosniff"
	DefaultReferrerPolicy          = "strict-origin-when-cross-origin"
	DefaultPermissionsPolicy       = "camera=(), microphone=(), geolocation=(), payment=()"
	DefaultStrictTransportSecurity = "max-age=63072000; includeSubDomains"
	DefaultXFrameOptions           = "DENY"
)

// CSP directive names and base sources. The zero value of Policy rebuilds
// DefaultContentSecurityPolicy from exactly these pieces.
const (
	directiveDefaultSrc     = "default-src"
	directiveScriptSrc      = "script-src"
	directiveStyleSrc       = "style-src"
	directiveImgSrc         = "img-src"
	directiveConnectSrc     = "connect-src"
	directiveFontSrc        = "font-src"
	directiveObjectSrc      = "object-src"
	directiveBaseURI        = "base-uri"
	directiveFormAction     = "form-action"
	directiveFrameAncestors = "frame-ancestors"
)

const (
	sourceSelf = "'self'"
	sourceNone = "'none'"
)

const (
	baseDefaultSrc = "'self'"
	baseScriptSrc  = "'self' 'wasm-unsafe-eval'"
	baseStyleSrc   = "'self'"
	baseImgSrc     = "'self' data:"
	baseConnectSrc = "'self'"
	baseFontSrc    = "'self'"
)

const (
	directiveSeparator = "; "
	sourceSeparator    = " "
)

// frameOptionsSameOrigin is emitted as X-Frame-Options once frame ancestors
// are allowed: the deny-all default lifted on both headers together, so the
// two never disagree with the stricter one silently winning.
const frameOptionsSameOrigin = "SAMEORIGIN"

// forwardedProtoHeader carries the request scheme through proxies and the
// edge: Strict-Transport-Security is emitted only when it says https.
// Sending it over plain HTTP is meaningless and misleading.
const forwardedProtoHeader = "X-Forwarded-Proto"
const forwardedProtoHTTPS = "https"

// Policy is the response security policy. The ZERO VALUE is the hardened
// policy: every header emitted, every directive at its strictest.
//
// Every method ADDS an allowance to one directive. No method removes a
// directive, and none disables a header: a response with no security headers
// is not representable through this type.
//
// Policy is a value type and every method returns a new Policy, so a partially
// built policy cannot be mutated from elsewhere.
type Policy struct {
	images          []string
	connections     []string
	styles          []string
	scripts         []string
	fonts           []string
	frameAncestors  []string
	maxRequestBytes int64
}

// AllowImages adds origins to the img-src directive, beside 'self' and data:.
func (p Policy) AllowImages(origins ...string) Policy {
	p.images = append(append([]string(nil), p.images...), origins...)
	return p
}

// AllowConnections adds origins to the connect-src directive, beside 'self'.
func (p Policy) AllowConnections(origins ...string) Policy {
	p.connections = append(append([]string(nil), p.connections...), origins...)
	return p
}

// AllowStyles adds origins to the style-src directive, beside 'self'.
func (p Policy) AllowStyles(origins ...string) Policy {
	p.styles = append(append([]string(nil), p.styles...), origins...)
	return p
}

// AllowScripts adds origins to the script-src directive, beside 'self' and
// 'wasm-unsafe-eval'.
func (p Policy) AllowScripts(origins ...string) Policy {
	p.scripts = append(append([]string(nil), p.scripts...), origins...)
	return p
}

// AllowFonts adds origins to the font-src directive, beside 'self'.
func (p Policy) AllowFonts(origins ...string) Policy {
	p.fonts = append(append([]string(nil), p.fonts...), origins...)
	return p
}

// AllowFrameAncestors adds origins to the frame-ancestors directive and lifts
// X-Frame-Options from DENY to SAMEORIGIN together: the two headers never
// disagree with the stricter one silently winning.
func (p Policy) AllowFrameAncestors(origins ...string) Policy {
	p.frameAncestors = append(append([]string(nil), p.frameAncestors...), origins...)
	return p
}

// Middleware returns the policy as router middleware. Install it with r.Use()
// before any route.
func (p Policy) Middleware() router.Middleware {
	return func(next router.HandlerFunc) router.HandlerFunc {
		return func(ctx router.Context) {
			ctx.SetHeader(HeaderContentSecurityPolicy, p.contentSecurityPolicy())
			ctx.SetHeader(HeaderXContentTypeOptions, DefaultXContentTypeOptions)
			ctx.SetHeader(HeaderReferrerPolicy, DefaultReferrerPolicy)
			ctx.SetHeader(HeaderPermissionsPolicy, DefaultPermissionsPolicy)
			ctx.SetHeader(HeaderXFrameOptions, p.frameOptions())
			if tlsRequest(ctx) {
				ctx.SetHeader(HeaderStrictTransportSecurity, DefaultStrictTransportSecurity)
			}
			if !p.bodyAllowed(ctx) {
				ctx.WriteStatus(statusRequestEntityTooLarge)
				return
			}
			next(ctx)
		}
	}
}

// contentSecurityPolicy renders the effective policy: the hardened default
// with every allowance added to its directive.
func (p Policy) contentSecurityPolicy() string {
	return fmt.JoinSlice([]string{
		directiveDefaultSrc + sourceSeparator + baseDefaultSrc,
		directiveScriptSrc + sourceSeparator + extendSource(baseScriptSrc, p.scripts),
		directiveStyleSrc + sourceSeparator + extendSource(baseStyleSrc, p.styles),
		directiveImgSrc + sourceSeparator + extendSource(baseImgSrc, p.images),
		directiveConnectSrc + sourceSeparator + extendSource(baseConnectSrc, p.connections),
		directiveFontSrc + sourceSeparator + extendSource(baseFontSrc, p.fonts),
		directiveObjectSrc + sourceSeparator + sourceNone,
		directiveBaseURI + sourceSeparator + sourceNone,
		directiveFormAction + sourceSeparator + sourceSelf,
		directiveFrameAncestors + sourceSeparator + p.frameAncestorsValue(),
	}, directiveSeparator)
}

// extendSource appends allowances to a directive's base sources.
func extendSource(base string, extra []string) string {
	if len(extra) == 0 {
		return base
	}
	return base + sourceSeparator + fmt.JoinSlice(extra, sourceSeparator)
}

// frameAncestorsValue renders the frame-ancestors directive: 'none' stands
// alone, so any allowance replaces it with 'self' plus the origins.
func (p Policy) frameAncestorsValue() string {
	if len(p.frameAncestors) == 0 {
		return sourceNone
	}
	return sourceSelf + sourceSeparator + fmt.JoinSlice(p.frameAncestors, sourceSeparator)
}

// frameOptions tracks frame-ancestors: the deny-all default, SAMEORIGIN once
// ancestors are allowed.
func (p Policy) frameOptions() string {
	if len(p.frameAncestors) == 0 {
		return DefaultXFrameOptions
	}
	return frameOptionsSameOrigin
}

// tlsRequest reports whether the request arrived over TLS, as carried by the
// edge and the origin through the forwarded-proto header.
func tlsRequest(ctx router.Context) bool {
	return ctx.GetHeader(forwardedProtoHeader) == forwardedProtoHTTPS
}
