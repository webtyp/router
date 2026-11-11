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
