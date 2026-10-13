package router

// HeaderIdempotencyKey is the request header that names one logical operation so the server
// can recognise a retry of it and answer it once (IETF draft "The Idempotency-Key HTTP Header
// Field"). The client side sends it (webtyp.com/rpc, Caller.CallKeyed) and the server side reads
// it (webtyp.com/idempotency middleware); both name it here so the two can never drift.
const HeaderIdempotencyKey = "Idempotency-Key"

// HeaderWWWAuthenticate is the response header carrying an authentication challenge.
const HeaderWWWAuthenticate = "WWW-Authenticate"

// ChallengeMFARequired is the RFC 9470 step-up challenge an enforcer sends with
// 401 when a route RequiresMFA and the caller's Assurance is lower.
const ChallengeMFARequired = `Bearer error="insufficient_user_authentication", error_description="multi-factor authentication required"`
