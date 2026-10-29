---
PLAN: "feat: a request says whether it came through the internet door (ContextKeyNetwork, FromInternet)"
EXECUTOR: jules
REVIEWER: none
STATUS: review
SESSION: 12560403996210911804
PR: https://github.com/webtyp/router/pull/14
REVIEWED_COMMIT: 65f7d5eb84830c155bd07dc29dcc62116b56ba40
---

> Read AGENTS.md at the repo root before any change.

# PLAN — the local door and the internet door

## Why

`veltylabs/mjosefa-cms` will serve the same app on two ports in one process: the clinic's local
network (login by RUT on a registered device, first-admin setup, enrolling a passkey) and the
internet through Cloudflare Tunnel (only a passkey). The code is the same; only what is allowed
changes (decisions D8 and D25 of `mjosefa-cms/docs/MVP_DEPLOYMENT_ROADMAP.md`). Through the tunnel
every request arrives from `127.0.0.1`, so the client IP cannot tell the doors apart; the **port**
can, and only the transport knows the port. The transport writes it into the request; `auth` and
the enforcer read it. This plan adds only the shared key and the reader. Next plans:
`webtyp/server` (marks the two listeners, demands a passkey on the internet door) and
`webtyp/auth` (RUT login, setup and passkey enrolment refuse the internet door).

## Design gate (api-design)

1. **Prior art.** Microsoft Entra Conditional Access «named locations» / «trusted locations» and
   Okta «network zones»: a policy reads where the request came from and demands more (MFA) or
   blocks (registering security info) outside the trusted one. Spring Boot `management.server.port`
   serves a second port with other rules from the same application. Here the location is the
   listener, which a visitor cannot forge (an IP header can be forged).
2. **Names.** `router.FromInternet(ctx)` reads as «did this come from the internet?».
   `ContextKeyNetwork` follows the existing `ContextKeyRemoteAddr`.
3. **Ledger.** Concepts +1 (local vs internet), files the app touches: 0 here, ways: +0.
4. **Where.** `router`: the key crosses the transport boundary (transports write it, `auth` reads
   it) — exactly what the `ContextKey*` constants are for (`router.go:24-31`).
5. **Deletes.** Nothing.

## Stage 1 — `router.go`

Next to `ContextKeyRemoteAddr`:

```go
// ContextKeyNetwork is the Context.Value key under which a transport says which
// network a request came through: NetworkLocal or NetworkInternet.
const ContextKeyNetwork = "Network"

// The two networks a request can come through.
const (
	NetworkLocal    = "local"
	NetworkInternet = "internet"
)

// FromInternet reports whether ctx came through the internet door. Only a request
// the transport marked NetworkLocal is local: an unmarked request counts as
// internet, so a transport that forgets to say gets the strict rules.
//
//	if router.FromInternet(ctx) { ctx.WriteStatus(404); return }
func FromInternet(ctx Context) bool {
	return ctx.Value(ContextKeyNetwork) != NetworkLocal
}
```

## Stage 2 — `tests/network_test.go` (new)

With `mock.Context` (`webtyp.com/router/mock`):

| Test | Context | `FromInternet` |
|---|---|---|
| `TestFromInternet_Unmarked` | fresh `&mock.Context{}` | `true` |
| `TestFromInternet_Local` | `SetValue(router.ContextKeyNetwork, router.NetworkLocal)` | `false` |
| `TestFromInternet_Internet` | `SetValue(…, router.NetworkInternet)` | `true` |
| `TestFromInternet_Unknown` | `SetValue(…, "lan")` | `true` |

## Stage 3 — docs

- `README.md`: under the `Context` section, one paragraph: what `ContextKeyNetwork` is, that the
  transport writes it per listener, that unmarked = internet, and the `FromInternet` example.
- `docs/ARCHITECTURE.md`: why the door is the listener and not the IP (the tunnel delivers every
  visit as `127.0.0.1`; `X-Forwarded-For` is forgeable), and why unmarked is strict.

## Delivery check

```
gotest
```

- A PR that does not compile, or whose tests could not run, is a failed delivery.
- Never pause to ask permission: when the check passes, commit, push and open the PR. Stuck 30
  minutes: open the PR listing what fails.
- `git diff --stat main` lists only: `router.go`, `tests/network_test.go`, `README.md`,
  `docs/ARCHITECTURE.md`, this plan.

## Stages

| # | Files | Done when |
|---|---|---|
| 1 | `router.go` | compiles |
| 2 | `tests/network_test.go` | 4 tests pass |
| 3 | `README.md`, `docs/ARCHITECTURE.md` | paragraphs present |
