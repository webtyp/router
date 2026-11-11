---
PLAN: "feat!: router.Context.RawQuery, Path() is the path only, and the conformance clauses that make every transport agree"
TAG: v0.5.0
EXECUTOR: jules
REVIEWER: none
STATUS: running
SESSION: 15121901154878335519
---

> Read AGENTS.md at the repo root before any change.
> **Finish by yourself:** when the delivery check passes, commit, push and open the pull request. Never ask whether to
> proceed or finalize — nobody answers, and a session that waits ends without your work. Commit after every stage.
> Never run `git reset`, `git restore`, `git checkout -- <file>` or `git stash` on your own work.
> **Publishing:** you publish only by OPENING THE PULL REQUEST (a `git push` from your sandbox does not reach GitHub). Open it after Stage 2 with whatever compiles and keep committing to it.
> **Never ask a yes/no question about progress**; ask only when a signature in this plan is wrong, and name the symbol.

# Plan: one meaning for `Path()` and a `RawQuery()` in every transport

## Why (read once)

`Context.Path()` means two things today: `server/httpd` and `router/mock` return the path **with** its `?query`; `cloudflare/edge` returns the path only (`cloudflare/edge/edge.go:77`). A handler cannot read the query portably, and `cloudflare/files` already depends on «path only» (`ctx.Path()[len(prefix):]`). The file-upload design (D46) needs to read the query of a request that carries a raw file in its body. Decision (owner, 2026-11-10): `Path()` is the path only in every transport, and the query has its own method, `RawQuery()`; and a conformance clause makes a transport that disagrees fail.

## Ecosystem search

Searched `QueryOf`, `QueryReader`, `DecodeArgs` (`router/query_codec.go`): the reader of a query as a record **already exists**; this plan adds no reader. `router/conformance` (`Run`, `Factory`) already is the executable contract of `Router`; this plan adds clauses to it and a sibling suite for contexts that no `Router` serves (`crud` loopback, `mcp/harvest.go`). New names: `Context.RawQuery`, `router.PathOf`, `conformance.RunContext`.

## Design gate

1. **Prior art:** Go `net/http` `URL.Path` + `URL.RawQuery`; Node `URL.pathname` + `URL.search`; chi/gin `Query` helpers on top of the same split. All keep path and query as two values. https://pkg.go.dev/net/url#URL (unverified in this session: from memory).
2. **Novice name:** `RawQuery` is Go's own word for the query string before parsing; `PathOf` mirrors the existing `QueryOf`.
3. **Ledger:** concepts +1 (`RawQuery`); files to touch to read a query: −N (one call instead of a transport-specific parse); lines at the call site −1; ways to do the same thing: 0 (the `Path()`-with-query form dies in the same change).
4. **Where:** contract and its conformance live in `router` (one concern); each transport implements it in its own repo (separate plans, after this one is published).
5. **Deletes:** the «Path() may carry a query» behaviour in `router/mock`; the comment that says so in `server/httpd` is for that repo's plan.

## Tools

```bash
go install webtyp.com/devflow/cmd/gotest@latest
go get webtyp.com/model@v0.2.20 webtyp.com/fmt@latest
go mod tidy
```
`router` has no `replace`. If a module cannot be fetched, stop and say so in the PR; never write a hash by hand.

## APIs used

| symbol | exact signature | tag | file |
|---|---|---|---|
| `router.QueryOf` | `func QueryOf(path string) string` (query of a target, without `?` and fragment) | this repo | `query_codec.go:17` |
| `router.QueryReader` | `func QueryReader(query string) model.FieldReader` | this repo | `query_codec.go:44` |
| `fmt.Index` | `func Index(s, substr string) int` | webtyp/fmt | used by `QueryOf` |
| `mock.Context` | struct with `InMethod`, `InPath`, `InBody` | this repo | `mock/context.go:16` |
If a signature in the repository differs from this table, stop and name the symbol in the PR; never adapt it by guessing.

## Stages

| Stage | File | What |
|---|---|---|
| 1 | [PLAN_STAGE_1_CONTRACT.md](PLAN_STAGE_1_CONTRACT.md) | `PathOf`, `Context.RawQuery`, doc of `Path()` |
| 2 | [PLAN_STAGE_2_MOCK.md](PLAN_STAGE_2_MOCK.md) | `mock.Context` implements the new meaning |
| 3 | [PLAN_STAGE_3_CONFORMANCE.md](PLAN_STAGE_3_CONFORMANCE.md) | clauses in `Run` + `RunContext` |
| 4 | [PLAN_STAGE_4_TESTS_DOCS.md](PLAN_STAGE_4_TESTS_DOCS.md) | tests, `docs/TESTS.md`, README, ARCHITECTURE |

## Delivery check

`gotest` passes (it builds, vets, races and runs every destination); `grep -rn "InPath: \"[^\"]*?" --include=*.go . | grep -v _test` → empty; the pull request is open. A tree that does not compile or whose tests did not run is a failed delivery. **Every other implementation of `Context` (`server/httpd`, `cloudflare/edge`, `crud` loopback, `mcp/harvest.go`) stops compiling against this version until its own plan lands: that is expected, do not touch those repos.**
