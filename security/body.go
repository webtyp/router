package security

import "webtyp.com/router"

// DefaultMaxRequestBytes caps the request body when the Policy leaves it at
// the zero value (1 MiB). A zero value meaning "unlimited" would make the zero
// value the unsafe state, so zero means the default and unlimited is not
// offered: an application handling large uploads calls MaxRequestBytes with a
// number instead.
const DefaultMaxRequestBytes = 1 << 20

// statusRequestEntityTooLarge is answered when the body exceeds the cap. The
// handler never runs.
const statusRequestEntityTooLarge = 413

// MaxRequestBytes caps the request body. The zero value is
// DefaultMaxRequestBytes; there is no way to express "unlimited".
func (p Policy) MaxRequestBytes(n int64) Policy {
	p.maxRequestBytes = n
	return p
}

// maxBytes resolves the effective cap: a non-positive value means the default,
// so the zero value of Policy is the hardened policy.
func (p Policy) maxBytes() int64 {
	if p.maxRequestBytes <= 0 {
		return DefaultMaxRequestBytes
	}
	return p.maxRequestBytes
}

// bodyAllowed reports whether the request body fits the cap. The body is
// capped before the handler reads it.
func (p Policy) bodyAllowed(ctx router.Context) bool {
	return int64(len(ctx.Body())) <= p.maxBytes()
}
