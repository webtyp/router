package router

// HeaderIdempotencyKey is the request header that names one logical operation so the server
// can recognise a retry of it and answer it once (IETF draft "The Idempotency-Key HTTP Header
// Field"). The client side sends it (webtyp.com/rpc, Caller.CallKeyed) and the server side reads
// it (webtyp.com/idempotency middleware); both name it here so the two can never drift.
const HeaderIdempotencyKey = "Idempotency-Key"

// HeaderWWWAuthenticate is the response header carrying an authentication challenge.
const HeaderWWWAuthenticate = "WWW-Authenticate"

// HeaderHost is the request header naming the host the client asked for
// ("user2.localhost:6060"). Transports answer ctx.GetHeader(HeaderHost) even
// where the platform keeps it outside the header map (net/http's r.Host).
const HeaderHost = "Host"

// HeaderContentType is the header naming the media type of a body.
const HeaderContentType = "Content-Type"

// ContentTypeJSON is the media type of a JSON body.
const ContentTypeJSON = "application/json"

// ChallengeMFARequired is the RFC 9470 step-up challenge an enforcer sends with
// 401 when a route RequiresMFA and the caller's Assurance is lower.
const ChallengeMFARequired = `Bearer error="insufficient_user_authentication", error_description="multi-factor authentication required"`
