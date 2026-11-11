← [Stage 1](PLAN_STAGE_1_CONTRACT.md) | Next → [Stage 3](PLAN_STAGE_3_CONFORMANCE.md)

> Read AGENTS.md and the index `PLAN.md` first. Commit when this stage compiles.

# Stage 2 — mock

`mock.Context` keeps `InPath` as the **target** a test writes (`"/api/q?a=1"`); its answers follow the contract.

In `router/mock/context.go` replace `Path()` and add `RawQuery()`:

```go
func (c *Context) Path() string {
	if c.InPath != "" {
		return router.PathOf(c.InPath)
	}
	return "/"
}

func (c *Context) RawQuery() string {
	return router.QueryOf(c.InPath)
}
```

Change the `Decode` of the mock (line ~179) from `router.QueryOf(c.Path())` to `c.RawQuery()`:

```go
return router.DecodeArgs(c.Method(), c.RawQuery(), c.Body(), into, func(body []byte, dst model.Decodable) error {
```

(keep the rest of that call exactly as it is). Update the doc comment of `InPath` (line 18) to: `// InPath is the request target: the path, optionally followed by ?query.`

Check: `grep -n "c.Path()" router/mock/*.go` → no remaining use that expects a query.

## 2.2 `mock.Router` matches on the path only (found by the first run of Stage 3)

`mock.Router.Invoke(method, path, ctx)` (`router/mock/router.go:215`) receives the **target** a test sends (path plus `?query`) and passes it to `match`. Match on the path only: in `Invoke`, first line,

```go
func (r *Router) Invoke(method, target string, ctx router.Context) {
	path := router.PathOf(target)
```

(rename the parameter `path` → `target`, keep the rest of the function using `path`). `gateAndServe` and `match` keep their signatures and receive the path. Check: `TestMockConformance` passes with the five new clauses.
