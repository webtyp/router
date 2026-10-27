---
PLAN: "feat: ResourceIntrospection — the one permission name for /_routes and the API explorer"
TAG: v0.4.9
EXECUTOR: jules
REVIEWER: none
---

> Read AGENTS.md at the repo root before any change.

# Plan — `webtyp.com/router`: name the permission of `/_routes` once

## Why

`/_routes` (served by `MountIntrospection`) and the API explorer screen that reads it are guarded by the
same RBAC resource, `"api_explorer"`. Today that string is written in three repos (`webtyp/appkit`
`server.go:24`, `webtyp/layout` `apiexplorer/explorer.go:94`, `veltylabs/mjosefa-cms` `config/config.go:26`).
This repo owns `/_routes` (`IntrospectionPath`), so the name of its permission belongs here, next to the path.
The consumers switch to it in their own plans.

## Tools

```
go install webtyp.com/devflow/cmd/gotest@latest
```

## Ecosystem search

- `grep -rn '"api_explorer"' --include=*.go` over `webtyp` and `veltylabs`: three copies, listed above. No
  exported constant exists → **new** constant here (the owner of `/_routes`).
- `router.IntrospectionPath` (`introspection.go:25`) is the precedent: the path constant already lives here.

## Design gate

1. **Prior art.** Spring Boot Actuator ships its endpoints together with the authority that guards them; Laravel
   Telescope ships the `viewTelescope` gate with the dashboard; Django `admindocs` is guarded by the `is_staff`
   check the admin owns. The piece that serves the data names its permission; consumers import it.
2. **Novice-name test.** `router.ResourceIntrospection` reads «the resource of introspection», beside
   `router.IntrospectionPath` and `router.MountIntrospection`. The VALUE stays `"api_explorer"` because stored
   permission rows already use it; renaming the value would silently revoke every existing grant.
3. **Complexity ledger.** Concepts +1 / −0 (in this repo); copies of the literal across the ecosystem −3 once the
   consumers adopt it; ways to name the permission +1 now, −3 later → ends at one.
4. **Where it belongs.** The repo that serves `/_routes`. `appkit` and `layout` already import `router`.
5. **What it deletes.** Nothing here; it enables deleting `appkit.ResourceAPIExplorer`, the literal in
   `apiexplorer` and `mjosefa-cms`'s `config.ResourceAPIExplorer` (their plans).

## Stage 1 — the constant

**File:** `introspection.go`, right after `IntrospectionPath`:

```go
// ResourceIntrospection is the RBAC resource that guards the route table at
// IntrospectionPath and every screen that reads it (the API explorer). Mount it as
//
//	router.MountIntrospection(r, router.IntrospectionPath, policy).Requires(router.ResourceIntrospection, model.Read)
//
// The value is "api_explorer" because stored permission rows already use it: changing
// it would silently revoke every grant that names it.
const ResourceIntrospection model.Resource = "api_explorer"
```

## Stage 2 — test

**File:** `tests/introspection_test.go` (extend; do not add a new file).

Add `TestResourceIntrospectionValue`: `router.ResourceIntrospection == model.Resource("api_explorer")`, failing
with the message `ResourceIntrospection changed: stored grants name "api_explorer"`.

## Stage 3 — docs

- `docs/INTROSPECTION.md`, «Mounting the Endpoint»: replace the whole code block with ONE line —
  `router.MountIntrospection(r, router.IntrospectionPath, policy).Requires(router.ResourceIntrospection, model.Read)` —
  and one sentence: the route table is always guarded, in development too (it is reconnaissance for an attacker,
  OWASP API9). Delete the `.Public()` example and the `ResourceRoutes` placeholder.
- `docs/DESIGN.md` line 56: replace «In development environments, the server automatically mounts
  `…Public()` at `/_routes`» with «The server mounts it guarded by `router.ResourceIntrospection` (read) in every
  environment».
- `README.md`: if it lists exported constants or `IntrospectionPath`, add `ResourceIntrospection` beside it.

Check: `grep -rn "Public()" docs/INTROSPECTION.md docs/DESIGN.md` → no line about `MountIntrospection`.

## Delivery

- `gotest` green. A PR that does not compile, or whose tests could not run, is a failed delivery.
  Environment: `GOPRIVATE=webtyp.com/*,github.com/veltylabs/*`, `GOFLAGS=-mod=mod`.
- Never pause to ask permission or report progress: when the check passes, commit, push and open the PR. Stuck 30
  minutes → open the PR listing what fails. Commit after every stage; never `git reset`, `git restore`,
  `git checkout -- <file>` or `git stash` on your own work.

| Stage | File | Action |
|---|---|---|
| 1 | `introspection.go` | add `ResourceIntrospection` |
| 2 | `tests/introspection_test.go` | pin the value |
| 3 | `docs/INTROSPECTION.md`, `docs/DESIGN.md`, `README.md` | always-guarded mount |
