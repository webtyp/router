package router

// HeaderIdempotencyKey is the request header that names one logical operation so the server
// can recognise a retry of it and answer it once (IETF draft "The Idempotency-Key HTTP Header
// Field"). The client side sends it (webtyp.com/rpc, Caller.CallKeyed) and the server side reads
// it (webtyp.com/idempotency middleware); both name it here so the two can never drift.
const HeaderIdempotencyKey = "Idempotency-Key"
