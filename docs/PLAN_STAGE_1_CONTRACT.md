← [Index](PLAN.md) | Next → [Stage 2](PLAN_STAGE_2_MOCK.md)

> Read AGENTS.md and the index `PLAN.md` first. Commit when this stage compiles.

# Stage 1 — contract

## 1.1 `router/query_codec.go` — add next to `QueryOf`

```go
// PathOf returns the path of a request target, without its query and fragment ("/api/q?a=1#f" is "/api/q").
// It is what every transport's Context.Path answers.
//
//	path := router.PathOf("/api/q?a=1") // "/api/q"
func PathOf(target string) string {
	if idx := fmt.Index(target, string(queryFragment)); idx != -1 {
		target = target[:idx]
	}
	if idx := fmt.Index(target, string(queryStart)); idx != -1 {
		target = target[:idx]
	}
	return target
}
```

## 1.2 `router/router.go` — `Context` (lines 16–19)

Replace

```go
type Context interface {
	Method() string
	Path() string
	Body() []byte
```

with

```go
type Context interface {
	Method() string
	// Path is the path of the request only: never the query ("/api/items/42", not "/api/items/42?x=1").
	Path() string
	// RawQuery is the query of the request without the "?" and without any fragment ("a=1&b=two"), "" when there is none.
	// Read it as a record with QueryReader: rec.DecodeFields(router.QueryReader(ctx.RawQuery())).
	RawQuery() string
	Body() []byte
```

## 1.3 Check

`grep -rn "QueryOf(c.Path())\|QueryOf(ctx.Path())" --include=*.go . | grep -v _test` lists the call sites that read the query out of `Path()`. In THIS repo the only one is `mock/context.go` (Stage 2). Do not edit other repos.
