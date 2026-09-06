---
PLAN: "feat: router/security — response security policy, hardened at the zero value"
EXECUTOR: jules
REVIEWER: none
---

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.
> Queued behind `docs/PLAN.md` in this repository — dispatch that one first.

## Prerequisite — install the test runner

External agents run in isolated environments where `gotest` is not installed.
Run this **before anything else**; the acceptance criteria depend on it:

```bash
go install webtyp.com/devflow/cmd/gotest@latest
```

Then use `gotest` for the whole suite and `gotest -run TestName` for one test.
Never call `go test` directly: `gotest` handles `-vet`, `-race`, `-cover`, the
WASM suite and the README badges.

# Plan — `router/security`

## Context (the executing agent has none — read this fully)

`webtyp.com/router` is the HTTP-shaped routing contract. Two independent
implementations serve real traffic: `webtyp.com/server/httpd` (the origin
server) and `webtyp.com/cloudflare/edge` (the Worker at the edge).

Neither sets a single response security header today. Verified: no
`Content-Security-Policy`, `Strict-Transport-Security`, `X-Content-Type-Options`,
`Referrer-Policy`, `X-Frame-Options` or `Permissions-Policy` anywhere in
`server/` or `router/`, and no request body size limit.

Meanwhile one application wrote them by hand —
`veltylabs/iam/routes/headers.go`, 60 lines of correct, carefully reasoned
policy trapped inside a single app. That is the exact case the ecosystem rule
names: *"the glue is written once, in the library that owns it. If every
application would write the same wiring, that wiring belongs to a piece."*

### Why this package and not `httpd`

If the policy lived in `httpd`, `cloudflare/edge` would not have it, and an app
deployed to Cloudflare would ship with no security headers while the same app
in development had them. That is the dev/production divergence the ecosystem is
currently eliminating, reintroduced.

Both implementations depend on `webtyp.com/router`. The policy's entire surface
is `router.Middleware` over `router.Context`. A subpackage here is reachable by
both, adds no dependency to anyone, and creates no new repository — the same
decision, for the same reason, as `router/routescan`.

### Anti-footgun

This package is imported by the edge Worker, which is compiled to WASM. Use
`webtyp.com/fmt` rather than `strings`/`strconv`/`errors`, matching the rest of
the WASM-reachable tree. Do not import `net/http`.

## Design gate

**1. Prior art.** Helmet (Express) and secure (Go, unrolled/secure) ship a
hardened default set and expose per-directive overrides. Django's
`SecurityMiddleware` and Rails' `default_headers` are on by default and are
configured by changing values, not by switching the middleware off. Spring
Security emits its header set unless explicitly disabled. The convention across
ecosystems is: **on by default, tuned by directive.** None of them makes the
secure state opt-in.

Where this design goes further: none of the above makes "no headers"
unrepresentable. Helmet has `helmet({contentSecurityPolicy: false})`; Spring has
`.headers().disable()`. Here there is no such call, because principle 8 says the
safe state is what you get by writing nothing and opening costs an explicit,
greppable line.

**2. Novice-name test.** `Policy{}.AllowImages("https://cdn.example.com")` reads
as a sentence and states intent. Every method is `Allow…` — a reader scanning a
composition root sees exactly what was loosened and nothing else. Rejected:
`Config` (says nothing), `Headers` (names the mechanism, not the intent),
`Disable*`/`Without*` (would make the unsafe state writable).

**3. Ledger.**

```
Concepts to learn                +1   (Policy)
Lines in an app that wants defaults  0   (nothing is written)
Lines in iam                     −60  (headers.go is deleted, replaced by one AllowImages call)
Ways to have no security headers −1   (1 → 0: it becomes unwritable)
Places the header set is defined −1   (per-app → one)
```

**4. Where it belongs.** Response security policy is one concern, owned here as
a subpackage. It is not a second concern inside `router`'s root.

**5. What it deletes.** `veltylabs/iam/routes/headers.go` in full — tracked as a
consumer follow-up, not in this repository.

## What to build

Create `security/` (package `security`) in this repository.

```go
// Policy is the response security policy. The ZERO VALUE is the hardened
// policy: every header emitted, every directive at its strictest.
//
// Every method ADDS an allowance to one directive. No method removes a
// directive, and none disables a header: a response with no security headers is
// not representable through this type.
type Policy struct { /* all fields unexported */ }

func (p Policy) AllowImages(origins ...string) Policy
func (p Policy) AllowConnections(origins ...string) Policy
func (p Policy) AllowStyles(origins ...string) Policy
func (p Policy) AllowScripts(origins ...string) Policy
func (p Policy) AllowFonts(origins ...string) Policy
func (p Policy) AllowFrameAncestors(origins ...string) Policy

// MaxRequestBytes caps the request body. The zero value is DefaultMaxRequestBytes;
// there is no way to express "unlimited".
func (p Policy) MaxRequestBytes(n int64) Policy

// Middleware returns the policy as router middleware. Install it with r.Use()
// before any route.
func (p Policy) Middleware() router.Middleware
```

`Policy` is a value type and every method returns a new `Policy`, so a partially
built policy cannot be mutated from elsewhere.

### The default values

Taken verbatim from `veltylabs/iam/routes/headers.go`, which is already reviewed
and correct. Each is an exported constant so a consumer can assert on it.

| Header | Value |
|---|---|
| `Content-Security-Policy` | `default-src 'self'; script-src 'self' 'wasm-unsafe-eval'; style-src 'self'; img-src 'self' data:; connect-src 'self'; font-src 'self'; object-src 'none'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'` |
| `X-Content-Type-Options` | `nosniff` |
| `Referrer-Policy` | `strict-origin-when-cross-origin` |
| `Permissions-Policy` | `camera=(), microphone=(), geolocation=(), payment=()` |
| `Strict-Transport-Security` | `max-age=63072000; includeSubDomains` |
| `X-Frame-Options` | `DENY` |

`'wasm-unsafe-eval'` is mandatory and permanent: this framework compiles Go to
WebAssembly, and instantiating a WASM module requires it. It is **not**
`'unsafe-eval'` and does not enable JavaScript `eval()`. Record that in the
constant's doc comment — the next reader will otherwise try to remove it.

`Strict-Transport-Security` is emitted **only when the request arrived over
TLS**. Sending it over plain HTTP is meaningless and misleading.

`X-Frame-Options` is emitted alongside `frame-ancestors` for older user agents;
`AllowFrameAncestors` must update both, or the two would disagree and the
stricter one would silently win.

### Request body limit

`DefaultMaxRequestBytes = 1 << 20` (1 MiB). The middleware caps the body before
the handler reads it. A request over the limit gets `413` and the handler never
runs.

A zero value meaning "unlimited" would make the zero value the unsafe state,
which is exactly what principle 8 forbids — hence zero means the default, and
unlimited is not offered. An application handling large uploads calls
`MaxRequestBytes` with a number.

## Constraints

- **No hardcoded strings.** Every header name and default value is an exported
  named constant.
- **Minimal surface.** Export `Policy`, its methods, and the default constants.
  Everything else is unexported.
- **No `any`.** Origins are `...string`; there is no map of arbitrary headers.

## Tests — `security_test.go`

Table-driven against `router/mock`:

1. `Policy{}` → all six headers present with the documented values.
2. `Policy{}` over a non-TLS request → HSTS absent, the other five present.
3. `AllowImages("https://x.test")` → `img-src` contains `'self'`, `data:` **and**
   the new origin; every other directive unchanged.
4. Chained allowances accumulate and do not overwrite each other.
5. `AllowFrameAncestors("https://x.test")` → `frame-ancestors` updated **and**
   `X-Frame-Options` updated consistently.
6. Body of `DefaultMaxRequestBytes + 1` → `413`, handler not invoked.
7. `MaxRequestBytes(10 << 20)` → a 5 MiB body reaches the handler.
8. The CSP constant contains `'wasm-unsafe-eval'` — a regression guard, because
   removing it breaks every WASM page in the ecosystem.

## Acceptance criteria

1. `grep -rn "func (p Policy) Disable\|func (p Policy) Without\|Enabled bool" security/` → empty.
2. `grep -rn "net/http\|\"strings\"\|\"strconv\"" security/` → empty.
3. `go build ./... && go vet ./... && go test ./...` → clean.
4. Test 8 passes.

## Stages

| # | Stage | File(s) | Gate |
|---|---|---|---|
| 1 | `Policy`, constants, defaults | `security/security.go` | tests 1, 2, 8 |
| 2 | `Allow*` methods | `security/security.go` | tests 3, 4, 5 |
| 3 | body limit | `security/body.go` | tests 6, 7 |

Sequential.

## Consumer follow-ups (not this repository)

- `server/httpd` and `cloudflare/edge`: install `security.Policy{}.Middleware()`
  by default, so an application gets it without writing anything.
- `veltylabs/iam`: delete `routes/headers.go`; replace with
  `security.Policy{}.AllowImages("https://lh3.googleusercontent.com")` for the
  Google avatars.
