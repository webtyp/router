← [Stage 3](PLAN_STAGE_3_CONFORMANCE.md) | Next → [Index](PLAN.md)

> Read AGENTS.md and the index `PLAN.md` first. Commit when this stage compiles.

# Stage 4 — tests and documentation

## 4.1 Tests (`router/tests/`; one row each in `docs/TESTS.md`, `| [Name](../tests/file_test.go) | Rule |`)

| Test | setup → call → expected |
|---|---|
| `TestPathOf_StripsQueryAndFragment` (`query_codec_test.go`) | `PathOf("/a/b?x=1#f")` → `/a/b`; `PathOf("/a/b")` → `/a/b`; `PathOf("/a?")` → `/a`; `PathOf("")` → `""` |
| `TestMockContext_PathAndRawQuery` (`mock_decode_test.go`) | `&mock.Context{InPath: "/p?a=1"}` → `Path()` `/p`, `RawQuery()` `a=1`; `InPath: ""` → `Path()` `/`, `RawQuery()` `""` |
| `TestMockContextConformance` (`conformance_test.go`) | `conformance.RunContext(t, func(m, target string, b []byte) router.Context { return &mock.Context{InMethod: m, InPath: target, InBody: b} })` passes |
| `TestMockConformance` (exists) | passes with the five new clauses (no edit: the mock's `ServeFunc` already puts the target in `InPath`) |

## 4.2 Documentation (English, `webtyp/*`)

`README.md` (≤150 lines, 4 sections) and `docs/ARCHITECTURE.md`: say that `Path()` is the path only and `RawQuery()` the query in every transport, with the three-line example `rec.DecodeFields(router.QueryReader(ctx.RawQuery()))`; `docs/diagrams/` — if the repo has a diagram of the request context, add the two values; otherwise none. `docs/TESTS.md`: the new rows, sorted as the file is.

## 4.3 Delivery

`gotest` green; the greps of the index «Delivery check» empty; the pull request is open.
