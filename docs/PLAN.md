---
PLAN: "refactor!: router owns WriteError; drop OperationModule and router/loopback (moved to crud)"
EXECUTOR: antigravity
REVIEWER: none
---

> Read AGENTS.md at the repo root before any change.
> **Finish by yourself:** when the delivery check passes, commit after every stage and stop. Never ask whether to
> proceed. This plan runs **locally**: do not tag, push or publish anything; the owner reviews first.

# Plan: `router.WriteError`, delete `OperationModule` and `router/loopback`

Context (master `ARCHITECTURE_MASTER_PLAN.md`, decisions D5, D7, D14): domain modules will only ever see
`crud.Registry`. `router.OperationRegistry` stays as the **transport primitive** (rpc, mcp, dbsync and
`crud.Loopback` implement it), but `router.OperationModule` disappears and the in-process caller moves to
`webtyp.com/crud` (`crud.Loopback`). `crud` must stop importing `webtyp.com/rpc`, so the error writer moves here.

## Tools

```bash
go install webtyp.com/devflow/cmd/gotest@latest
```
Environment: `GOPRIVATE=webtyp.com/*,github.com/veltylabs/*`, `GOFLAGS=-mod=mod`.

## APIs used (all in this repo or `webtyp.com/model` v0.2.13 — read them before coding)

| Symbol | Signature | File |
|---|---|---|
| `model.KindOf` | `func KindOf(err error) ErrorKind` | model `error.go` |
| `model.FieldOf` | `func FieldOf(err error) string` | model `error.go` |
| `model.ErrorInternal` | `ErrorKind` constant | model `error.go` |
| `router.StatusOf` | `func StatusOf(k model.ErrorKind) int` | `problem.go` |
| `router.Problem` | `struct{Status int64; Title, Detail, Field string}` | `problem.go` |
| `router.Context` | interface: `SetHeader`, `WriteStatus`, `Encode`, `SetValue(key, value string)`, `Value(key string) string` | `router.go:16` |

If a signature differs from this table, stop and name the symbol; never adapt by guessing.

## Stage 1 — `router.WriteError` and `router.InternalError` (new file `problem_write.go`)

Port verbatim from `webtyp.com/rpc` v0.2.16 `errors.go:52-71` (`WriteError`) and its constants:

```go
package router

import "webtyp.com/model"

const (
	headerContentType  = "Content-Type"
	contentTypeProblem = "application/problem+json"
	msgInternal        = "internal error"
	ctxKeyInternal     = "router.internal"
)

// WriteError answers an operation's error. The status comes from model.KindOf(err):
// invalid 400, not found 404, conflict 409, anything unclassified 500. The body is an
// RFC 9457 problem (Content-Type application/problem+json):
//
//	{"status":400,"title":"invalid","detail":"name required","field":"name"}
//
// A 500 never carries err's text: its detail is "internal error" and the real error is
// kept for the transport's logger (InternalError).
//
//	if err != nil { router.WriteError(ctx, err); return }
func WriteError(ctx Context, err error) {
	kind := model.KindOf(err)
	status := StatusOf(kind)
	detail := err.Error()
	if kind == model.ErrorInternal {
		detail = msgInternal
		ctx.SetValue(ctxKeyInternal, err.Error())
	}
	p := Problem{Status: int64(status), Title: kind.String(), Detail: detail, Field: model.FieldOf(err)}
	ctx.SetHeader(headerContentType, contentTypeProblem)
	ctx.WriteStatus(status)
	ctx.Encode(&p)
}

// InternalError is the real text of the 500 WriteError answered on ctx, "" when none.
// A transport logs it; the client never sees it.
//
//	if d := router.InternalError(ctx); d != "" { log("internal error:", d) }
func InternalError(ctx Context) string { return ctx.Value(ctxKeyInternal) }
```
If `headers.go` already declares a Content-Type constant, reuse it instead of `headerContentType`.
Update the two comments in `problem.go` (lines 6 and 45) from `rpc.WriteError` to `router.WriteError`.

Tests `tests/write_error_test.go` (port `webtyp.com/rpc` v0.2.16 `tests/write_error_test.go`, using `router/mock` instead of the loopback):
- `TestWriteError_InvalidIs400WithField`: `model.Invalid`-kind field error → status 400, body `{"status":400,"title":"invalid",...,"field":"name"}`.
- `TestWriteError_InternalHidesText`: `errors.New("db down")` → 500, detail `internal error`, `InternalError(ctx) == "db down"`.
- `TestWriteError_NotFound404` and `TestWriteError_Conflict409`.

## Stage 2 — delete `OperationModule` and `router/loopback`

- `router.go`: delete `OperationModule` (lines 199-208) and rewrite the `OperationRegistry` comment: it is the transport primitive rpc/mcp/dbsync and `crud.Loopback` implement; a domain module never receives it (it receives `crud.Registry`).
- Delete the folder `loopback/` (`loopback.go`, `loopback_test.go`). Its code is ported to `webtyp.com/crud` by the crud plan — do not copy it anywhere here.
- `conformance/conformance.go:625`: replace the mention of `router.OperationModule` with `crud.Module`.
- Grep criteria (all empty): `grep -rn "OperationModule" --include=*.go .`, `test ! -e loopback`.

## Stage 3 — documentation

`README.md` (≤150 lines, sections `Why This Exists`, `Before You Start`, `Step-by-Step Guide`, `Documentation Map`) and
`docs/ARCHITECTURE.md`: document `WriteError`/`InternalError`, state that `OperationRegistry` is a transport primitive,
remove every mention of `OperationModule` and `router/loopback` (`grep -rn "loopback\|OperationModule" README.md docs` → only
a sentence pointing to `crud.Loopback`).

## Delivery check

`gotest` passes. A tree that does not compile or whose tests did not run is a failed delivery.

| Stage | Files |
|---|---|
| 1 | `problem_write.go`, `problem.go`, `tests/write_error_test.go` |
| 2 | `router.go`, `loopback/` (deleted), `conformance/conformance.go` |
| 3 | `README.md`, `docs/ARCHITECTURE.md` |
