← [Stage 2](PLAN_STAGE_2_MOCK.md) | Next → [Stage 4](PLAN_STAGE_4_TESTS_DOCS.md)

> Read AGENTS.md and the index `PLAN.md` first. Commit when this stage compiles.

# Stage 3 — conformance

All code goes in `router/conformance/conformance.go` (helpers `build`, `ok`, `Anonymous`, `echoPayload` already exist there).

## 3.1 Five clauses in `Run`

Add after the `path_parameter…` group (line ~104):

```go
	t.Run("path_excludes_the_query", func(t *testing.T) { pathExcludesTheQuery(t, f) })
	t.Run("raw_query_is_the_query_only", func(t *testing.T) { rawQueryIsTheQueryOnly(t, f) })
	t.Run("raw_query_is_empty_without_a_query", func(t *testing.T) { rawQueryIsEmptyWithoutAQuery(t, f) })
	t.Run("raw_query_reads_as_a_record", func(t *testing.T) { rawQueryReadsAsARecord(t, f) })
	t.Run("body_with_a_query_arrives_intact", func(t *testing.T) { bodyWithAQueryArrivesIntact(t, f) })
```

and the clauses, each with the same shape as `pathParameterIsExtracted`:

```go
// echo registers POST /api/q whose handler writes what pick returns.
func echo(r router.Router, pick func(ctx router.Context) []byte) {
	r.Post("/api/q", func(ctx router.Context) {
		ctx.WriteStatus(200)
		ctx.Write(pick(ctx))
	}).Public()
}

func pathExcludesTheQuery(t *testing.T, f Factory) {
	r, serve := build(t, f)
	echo(r, func(ctx router.Context) []byte { return []byte(ctx.Path()) })
	got := serve("POST", "/api/q?a=1&b=two", nil, Anonymous)
	if got.Status != 200 || string(got.Body) != "/api/q" {
		t.Errorf("Path carried the query: got %d %q, want 200 %q", got.Status, got.Body, "/api/q")
	}
}

func rawQueryIsTheQueryOnly(t *testing.T, f Factory) {
	r, serve := build(t, f)
	echo(r, func(ctx router.Context) []byte { return []byte(ctx.RawQuery()) })
	got := serve("POST", "/api/q?a=1&b=two", nil, Anonymous)
	if got.Status != 200 || string(got.Body) != "a=1&b=two" {
		t.Errorf("RawQuery is not the query: got %d %q, want 200 %q", got.Status, got.Body, "a=1&b=two")
	}
}

func rawQueryIsEmptyWithoutAQuery(t *testing.T, f Factory) {
	r, serve := build(t, f)
	echo(r, func(ctx router.Context) []byte { return []byte("[" + ctx.RawQuery() + "]") })
	got := serve("POST", "/api/q", nil, Anonymous)
	if got.Status != 200 || string(got.Body) != "[]" {
		t.Errorf("RawQuery of a request without a query: got %d %q, want 200 %q", got.Status, got.Body, "[]")
	}
}

func rawQueryReadsAsARecord(t *testing.T, f Factory) {
	r, serve := build(t, f)
	echo(r, func(ctx router.Context) []byte {
		p := &echoPayload{}
		p.DecodeFields(router.QueryReader(ctx.RawQuery()))
		return []byte(p.Value)
	})
	got := serve("POST", "/api/q?value=hola%20mundo", nil, Anonymous)
	if got.Status != 200 || string(got.Body) != "hola mundo" {
		t.Errorf("RawQuery did not read as a record: got %d %q, want 200 %q", got.Status, got.Body, "hola mundo")
	}
}

func bodyWithAQueryArrivesIntact(t *testing.T, f Factory) {
	r, serve := build(t, f)
	echo(r, func(ctx router.Context) []byte { return ctx.Body() })
	want := []byte{0, 1, 2, 255, 254, 10, 13}
	got := serve("POST", "/api/q?a=1", want, Anonymous)
	if got.Status != 200 || string(got.Body) != string(want) {
		t.Errorf("a body sent with a query changed: got %d %v, want 200 %v", got.Status, got.Body, want)
	}
}
```

## 3.2 `RunContext` — the same contract for a Context no Router serves

Add to the end of the file:

```go
// RunContext holds a Context that no Router serves (the loopback of crud, the one of mcp, the mock) to the same
// meaning of Path, RawQuery and Body as every transport. newContext builds a Context for a request target
// ("/api/q?a=1&b=two", path plus query), a method and a body.
//
//	conformance.RunContext(t, func(method, target string, body []byte) router.Context { return newMine(method, target, body) })
func RunContext(t *testing.T, newContext func(method, target string, body []byte) router.Context) {
	if newContext == nil {
		t.Fatal("conformance: newContext is required")
	}
	t.Run("path_excludes_the_query", func(t *testing.T) {
		if got := newContext("POST", "/api/q?a=1&b=two", nil).Path(); got != "/api/q" {
			t.Errorf("Path carried the query: got %q, want %q", got, "/api/q")
		}
	})
	t.Run("raw_query_is_the_query_only", func(t *testing.T) {
		if got := newContext("POST", "/api/q?a=1&b=two", nil).RawQuery(); got != "a=1&b=two" {
			t.Errorf("RawQuery is not the query: got %q, want %q", got, "a=1&b=two")
		}
	})
	t.Run("raw_query_is_empty_without_a_query", func(t *testing.T) {
		if got := newContext("POST", "/api/q", nil).RawQuery(); got != "" {
			t.Errorf("RawQuery of a request without a query: got %q, want empty", got)
		}
	})
	t.Run("raw_query_reads_as_a_record", func(t *testing.T) {
		p := &echoPayload{}
		p.DecodeFields(router.QueryReader(newContext("POST", "/api/q?value=hola%20mundo", nil).RawQuery()))
		if p.Value != "hola mundo" {
			t.Errorf("RawQuery did not read as a record: got %q, want %q", p.Value, "hola mundo")
		}
	})
	t.Run("body_with_a_query_arrives_intact", func(t *testing.T) {
		want := []byte{0, 1, 2, 255, 254, 10, 13}
		if got := newContext("POST", "/api/q?a=1", want).Body(); string(got) != string(want) {
			t.Errorf("a body sent with a query changed: got %v, want %v", got, want)
		}
	})
}
```

Update the package doc (top of the file) with one more usage example: `conformance.RunContext(t, newContext)` for a Context no Router serves.
