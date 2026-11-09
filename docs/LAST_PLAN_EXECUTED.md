---
PLAN: "refactor!: router.Operation is the only operation identity (Caller and OperationRegistry take it); query codec for GET arguments"
EXECUTOR: antigravity
REVIEWER: none
---

> Read AGENTS.md at the repo root before any change.
> **Finish by yourself:** when the delivery check passes, commit after every stage and stop. Never ask whether to
> proceed. This plan runs **locally**: do not tag, push or publish anything; the owner reviews first.
> Never run `git reset`, `git restore`, `git checkout -- <file>` or `git stash` on your own work.

## Goal

Today an operation is a string (`"patient_c"`) that each transport parses its own way. After this plan an operation is a typed, comparable value:
the module that serves it, the entity it acts on and exactly one of the four `model.Action`s. Nobody parses a name. `rpc` turns it into
`POST|GET|PATCH|DELETE /api/<module>/<entity>`; `mcp` into a tool name; the in-process transports key on the value itself.
The same plan adds the query-string codec a `GET` needs (arguments travel in the query), so the server and the client share one implementation.

## Ecosystem search

Searched `Operation`, `QueryReader`, `formReader`, `QueryEscape` over `webtyp/*`: `router.QueryParam`/`QueryUnescape` (`query.go`) already parse one key of a query
(**extend**: the codec is built on them); `server/httpd/formdecode.go` has a private `formReader` over `net/url` that only handles flat form bodies (**replace**: its plan
deletes it and calls `router.QueryReader`, so the same logic is not in two repos; `QueryReader` accepts the same forms: `Bool` takes `true|on|1` and `false|off|0|""`, case-insensitive). `crud.Op` (a string builder) is deleted by the `crud` plan. No new library.

## Design gate

1. **Prior art** (unverified in this session): AWS Lambda `Invoke` takes a typed `InvocationType` instead of a magic string (`RequestResponse` | `Event`);
   gRPC addresses a call by (service, method), not by a parsed path; HTTP APIs of Stripe/GitHub use method + resource. Difference: the set is closed and tiny (4 actions).
2. **Novice-name test**: `router.Operation{Module: "patient_directory", Entity: "patient", Action: model.Create}` reads as "create a patient in patient_directory".
3. **Ledger**: concepts +1 (`Operation`); string-name parsing sites −4 (`rpc.Caller.send`, `rpc.activeRegistry`, `mcp` tool names, `crud.Op`); ways to name an operation 2 (string, `crud.Op`) → 1.
4. **Where**: `router` already owns `Caller`, `OperationRegistry`, `RouteInfo` and `model.Action`; the query codec sits next to `QueryParam`.
5. **Deletes**: the `string` operation name in `Caller` and `OperationRegistry`; the synthetic `"OP"` method path in `mock`.

## Tools

```bash
go install webtyp.com/devflow/cmd/gotest@latest
go install webtyp.com/lang/cmd/langc@latest
```

## APIs used

| symbol | exact signature | file |
|---|---|---|
| `model.Action` | `type Action uint8`; `Create, Read, Update, Delete`; `(a Action) Has(want Action) bool` | `model/rbac.go:83` |
| `model.FieldReader` | `String/Int/Float/Bool/Bytes(name) (T, bool)`, `Object(name string, into Decodable) bool`, `Array(name string) (ArrayReader, bool)`, `Raw(name string) (string, bool)` | `model/codec.go:41` |
| `model.FieldWriter` | `String/Int/Float/Bool/Bytes/Null/Raw`, `Object(name string, val Encodable)`, `Array(name string, n int) ArrayWriter` | `model/codec.go:6` |
| `model.Action.Verb`, `model.ParseVerb` | `func (a Action) Verb() string` (Create → `"create"`; a set or 0 → `""`); `func ParseVerb(verb string) (Action, bool)` | `model/rbac.go` (model next version, `v0.2.17`; until published, `go mod edit -replace webtyp.com/model=../model`) |
| `fmt.Index`, `fmt.Split`, `fmt.Convert(v).Int64()`, `fmt.Err` | `webtyp.com/fmt` (WASM-safe replacements of `strings`/`strconv`) — read `fmt/*.go` for the exact return types before using | `fmt/` |
| `router.QueryParam` | `func QueryParam(path, key string) (value string, ok bool)` (takes a path **with** `?`) | `query.go:36` |
| `router.QueryUnescape` | `func QueryUnescape(s string) string` | `query.go` |

If a signature differs from this table, stop and name the symbol in the PR; never adapt it by guessing.

## Stage 1 — tests first (red)

`tests/op_test.go` (package `tests`; import `webtyp.com/router`, `webtyp.com/model`):

| Test | setup → call → expected |
|---|---|
| `TestOperation_Valid` | `{Module:"m",Entity:"e",Action:Create}` → true; empty Module, empty Entity, `Action: 0`, `Action: Create\|Read` → each false |
| `TestOperation_StringRendersModuleEntityVerb` | `{m,e,Create}.String()=="m.e.create"`; an op whose Action is not exactly one of the four renders `"m.e.?"` and never panics |
| `TestParseOperation_RoundTrips` | for each of the four actions: `ParseOperation(op.String())` returns `op` and a nil error |
| `TestParseOperation_Rejects` | `""`, `"m.e"`, `"m.e.create.x"`, `"m.e.drop"`, `"..create"`, `"m..create"` → each returns an error whose text contains the input |
| `TestOperation_IsComparable` | `a := Operation{...}`, `b := Operation{...same}` → `a == b`; different Action → `a != b` |

`tests/query_codec_test.go` (a small record `qrec` with `ID string`, `Search string`, `Limit int64`, `On bool`, and a nested `qfilter{Name string}` field named `filter`, written
by hand in the test with `EncodeFields`/`DecodeFields`/`IsNil`):

| Test | setup → call → expected |
|---|---|
| `TestQueryOf` | `QueryOf("/api/m/e?id=1&x=2#frag")=="id=1&x=2"`; `QueryOf("/api/m/e")==""`; `QueryOf("/p?")==""` |
| `TestQuery_RoundTrip` | `EncodeQuery(&qrec{ID:"a b", Search:"12.345-6", Limit:50})` → `"id=a%20b&search=12.345-6&limit=50"`; `QueryReader(that)` decodes back an equal record |
| `TestQuery_ObjectUsesPrefix` | record with `filter{Name:"x"}` → string contains `filter.name=x`; decoding fills `filter.Name=="x"` |
| `TestQuery_ObjectAbsentIsNotDecoded` | query without any `filter.` key → `Object("filter", …)` returns false |
| `TestQuery_PresentButEmptyIsPresent` | `"search="` → `String("search")` returns `("", true)`; a missing key returns `("", false)` |
| `TestQuery_BoolForms` | `on=true`, `on=1`, `on=on`, `on=TRUE` → true; `on=false`, `on=0`, `on=off`, `on=` → false with ok; `on=maybe` → not ok (these are the forms an HTML checkbox and `httpd`'s old form reader accepted) |
| `TestQuery_IntRejectsGarbage` | `limit=abc` → `Int("limit")` returns `(0,false)` |
| `TestQuery_ArrayIsRefusedOnEncode` | record that writes `w.Array("ids",1)` → `EncodeQuery` returns an error whose text contains `array` |
| `TestDecodeArgs_GetUsesTheQuery` | `DecodeArgs("GET", "code=abc&qty=3", nil, &t, fromBody)` → `t` decoded from the query; `fromBody` never called |
| `TestDecodeArgs_GetIgnoresTheBody` | `DecodeArgs("GET", "code=q", []byte("{\"code\":\"body\"}"), &t, fromBody)` → `t.Code=="q"`; `fromBody` never called |
| `TestDecodeArgs_OtherMethodsUseTheBody` | for `POST`, `PATCH`, `DELETE`: `fromBody` is called once with that body and `into`; the query is not read |
| `TestDecodeArgs_BodyErrorIsReturned` | `fromBody` returns an error → `DecodeArgs` returns that same error |
| `TestQuery_EscapeReservedBytes` | `QueryEscape("a&b=c d%é")=="a%26b%3Dc%20d%25%C3%A9"`, and `QueryUnescape(QueryEscape(s))==s` for that `s` |

Run `gotest`: it must fail to compile (symbols missing). Commit: `test: Operation and query codec (red)`.

## Stage 2 — `op.go` and the signature change

New `op.go`:

```go
package router

import (
	"webtyp.com/fmt"
	"webtyp.com/model"
)

// Operation identifies one operation: the module that serves it, the entity it acts on and exactly one of the four actions.
// It is a comparable value: transports key on it and nobody parses a name.
//
//	op := router.Operation{Module: "patient_directory", Entity: "patient", Action: model.Create}
type Operation struct {
	Module string       // the module's ModelName(), e.g. "patient_directory"
	Entity string       // the entity's ModelName(), e.g. "patient"
	Action model.Action // exactly one of model.Create, model.Read, model.Update, model.Delete
}

const (
	opSep     = "."
	opUnknown = "?"

	errBadOperation = "router: an operation is written <module>.<entity>.<create|read|update|delete>, got: "
)

// ParseOperation is the inverse of String, for a stored operation (an offline queue). It refuses anything that is not exactly
// <module>.<entity>.<create|read|update|delete> with non-empty parts.
//
//	op, err := router.ParseOperation("patient_directory.patient.create")
func ParseOperation(s string) (Operation, error) {
	parts := fmt.Split(s, opSep)
	if len(parts) != 3 {
		return Operation{}, fmt.Err(errBadOperation + s)
	}
	action, known := model.ParseVerb(parts[2])
	if !known {
		return Operation{}, fmt.Err(errBadOperation + s)
	}
	op := Operation{Module: parts[0], Entity: parts[1], Action: action}
	if !op.Valid() {
		return Operation{}, fmt.Err(errBadOperation + s)
	}
	return op, nil
}

// Valid reports whether Module and Entity are set and Action is exactly one of the four actions (the word comes from model.Action.Verb).
func (o Operation) Valid() bool { return o.Module != "" && o.Entity != "" && o.Action.Verb() != "" }

// String renders "<module>.<entity>.<verb>": for logs and error messages and for the ONE place that must persist an operation
// (an offline queue stores it as text and reads it back with ParseOperation). Everything else holds an Operation.
func (o Operation) String() string {
	verb := o.Action.Verb()
	if verb == "" {
		verb = opUnknown
	}
	return o.Module + opSep + o.Entity + opSep + verb
}
```



Signature changes (find every site with `grep -rn "op string\|name string" --include=*.go .` and the list below):

| File | Old | New |
|---|---|---|
| `caller.go:23,26` | `Call(op string, args …)`, `Dispatch(op string, args …)` | `Call(op Operation, args …)`, `Dispatch(op Operation, args …)`; fix the doc comment: `op` is the typed operation, "NEVER a wire-level method" stays true |
| `router.go:185` | `Operation(name string, h HandlerFunc) Route` | `Operation(op Operation, h HandlerFunc) Route`; doc: "registers an operation by its typed identity" |
| `mock/caller.go` | `Call{Op string; Args}`, `Call(op string…)`, `Dispatch(op string…)` | `Call{Operation router.Operation; Args}`, `Call(op router.Operation…)`, `Dispatch(op router.Operation…)` |
| `mock/router.go:145` | `Operation(name string, h)`; synthetic method `"OP"`, path `"/"+name` | `Operation(op router.Operation, h)`; same synthetic method `"OP"`, path `"/"+op.String()` (an implementation detail the mock owns) |
| `conformance/conformance.go` | `opReg(t, r).Operation("with_args", …)`, `"do_thing"`, `"guarded_thing"`, `"failing_thing"`, the `ServeOperation` field taking `name string` | add `func op(entity string) router.Operation { return router.Operation{Module: "conformance", Entity: entity, Action: model.Read} }`; every call becomes `Operation(op("with_args"), …)`; `ServeOperation` takes `router.Operation`; update every implementer found by `grep -rn "ServeOperation" --include=*.go .` inside this repo (`routertest`, `mock`) |

`gotest` compiles the tests of stage 1 for `Operation`. Commit: `refactor!: router.Operation replaces the string operation name`.

## Stage 3 — the query codec (`query_codec.go`, new)

```go
package router

import (
	"webtyp.com/fmt"
	"webtyp.com/model"
)

const (
	queryObjectSep = "."
	hexDigits      = "0123456789ABCDEF"
	errQueryArray  = "router: a query string cannot carry an array; send the arguments in a request body"
)

// QueryOf returns the query of a request path ("" when it has none), without the "?" and without any fragment.
//
//	args.DecodeFields(router.QueryReader(router.QueryOf(ctx.Path())))
func QueryOf(path string) string {
	if idx := fmt.Index(path, string(queryFragment)); idx != -1 {
		path = path[:idx]
	}
	idx := fmt.Index(path, string(queryStart))
	if idx == -1 {
		return ""
	}
	return path[idx+1:]
}

// QueryEscape percent-encodes s for a query: every byte except A-Z a-z 0-9 - _ . ~ becomes %XX.
func QueryEscape(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.' || c == '~' {
			out = append(out, c)
			continue
		}
		out = append(out, queryEscape, hexDigits[c>>4], hexDigits[c&15])
	}
	return string(out)
}

// QueryReader reads the fields of a record from a query ("a=b&c=d", without the "?"). A present key with no value is present and empty.
// An object field "filter" is read from the keys "filter.<field>". Arrays are not supported in a query: Array returns false.
func QueryReader(query string) model.FieldReader { return &queryReader{query: query} }

type queryReader struct {
	query  string
	prefix string // "filter." while reading the object "filter"
}

func (q *queryReader) String(name string) (string, bool) { return QueryParam("?"+q.query, q.prefix+name) }
func (q *queryReader) Raw(name string) (string, bool)    { return q.String(name) }

func (q *queryReader) Int(name string) (int64, bool) {
	s, ok := q.String(name)
	if !ok {
		return 0, false
	}
	n, err := fmt.Convert(s).Int64()
	if err != nil {
		return 0, false
	}
	return n, true
}

func (q *queryReader) Float(name string) (float64, bool) {
	s, ok := q.String(name)
	if !ok {
		return 0, false
	}
	f, err := fmt.Convert(s).Float64()
	if err != nil {
		return 0, false
	}
	return f, true
}

func (q *queryReader) Bool(name string) (bool, bool) {
	s, ok := q.String(name)
	if !ok {
		return false, false
	}
	switch fmt.ToLower(s) {
	case "true", "on", "1":
		return true, true
	case "false", "off", "0", "":
		return false, true
	}
	return false, false
}

func (q *queryReader) Bytes(name string) ([]byte, bool) {
	s, ok := q.String(name)
	return []byte(s), ok
}

func (q *queryReader) Object(name string, into model.Decodable) bool {
	prefix := q.prefix + name + queryObjectSep
	found := false
	for _, part := range fmt.Split(q.query, string(querySep)) {
		if len(part) > len(prefix) && QueryUnescape(part[:len(prefix)]) == prefix {
			found = true
			break
		}
	}
	if !found {
		return false
	}
	into.DecodeFields(&queryReader{query: q.query, prefix: prefix})
	return true
}

func (q *queryReader) Array(name string) (model.ArrayReader, bool) { return nil, false }

const methodGet = "GET"

// DecodeArgs reads the arguments of a request into into. The method decides where they travel: a GET carries them in its query
// and never in a body (a body on a GET is ignored); every other method carries them in its body, decoded by fromBody. It is the
// one place that policy lives; httpd and the edge both call it.
//
//	err := router.DecodeArgs(ctx.Method(), router.QueryOf(ctx.Path()), ctx.Body(), &args, decodeJSON)
func DecodeArgs(method, query string, body []byte, into model.Decodable, fromBody func(body []byte, into model.Decodable) error) error {
	if method == methodGet {
		into.DecodeFields(QueryReader(query))
		return nil
	}
	return fromBody(body, into)
}

// EncodeQuery writes the fields of v as a query ("a=b&c=d"), the counterpart of QueryReader. An object field is written as
// "name.<field>"; an array is an error (send a body instead). A Null field is left out.
//
//	query, err := router.EncodeQuery(&crud.ReadArgs{Search: "12.345"})
func EncodeQuery(v model.Encodable) (string, error) {
	w := &queryWriter{}
	v.EncodeFields(w)
	if w.failed {
		return "", fmt.Err(errQueryArray)
	}
	return string(w.out), nil
}

type queryWriter struct {
	out    []byte
	prefix string
	failed bool
}

func (w *queryWriter) put(name, val string) {
	if len(w.out) > 0 {
		w.out = append(w.out, querySep)
	}
	w.out = append(w.out, QueryEscape(w.prefix+name)...)
	w.out = append(w.out, queryEq)
	w.out = append(w.out, QueryEscape(val)...)
}

func (w *queryWriter) String(name, val string)       { w.put(name, val) }
func (w *queryWriter) Raw(name, val string)          { w.put(name, val) }
func (w *queryWriter) Int(name string, val int64)    { w.put(name, fmt.Convert(val).String()) }
func (w *queryWriter) Float(name string, val float64) { w.put(name, fmt.Convert(val).String()) }
func (w *queryWriter) Bool(name string, val bool) {
	if val {
		w.put(name, "true")
		return
	}
	w.put(name, "false")
}
func (w *queryWriter) Bytes(name string, val []byte) { w.put(name, string(val)) }
func (w *queryWriter) Null(name string)              {}
func (w *queryWriter) Object(name string, val model.Encodable) {
	inner := &queryWriter{out: w.out, prefix: w.prefix + name + queryObjectSep}
	val.EncodeFields(inner)
	w.out, w.failed = inner.out, w.failed || inner.failed
}
func (w *queryWriter) Array(name string, n int) model.ArrayWriter {
	w.failed = true
	return discardArray{}
}

// discardArray swallows the elements of an array the query refused.
type discardArray struct{}

func (discardArray) String(string)          {}
func (discardArray) Int(int64)              {}
func (discardArray) Float(float64)          {}
func (discardArray) Bool(bool)              {}
func (discardArray) Bytes([]byte)           {}
func (discardArray) Object(model.Encodable) {}
func (discardArray) Close()                 {}
```

Note on `queryWriter.Object`: if `inner.out` begins with the bytes of `w.out`, the separator logic in `put` still works because `inner.out` starts non-empty. Check `TestQuery_ObjectUsesPrefix` proves it.
If `fmt.Convert(...).String()`, `.Int64()` or `.Float64()` have other names, use the ones in `webtyp.com/fmt` (read `fmt/convert.go`); do not use `strconv`.

`gotest` passes (including stage 1). Commit: `feat: QueryOf, QueryEscape, QueryReader, EncodeQuery, DecodeArgs`.

## Stage 4 — translations and documentation

- `langc sync .`: fill every empty value (all languages in `lang.json`); zero untranslated keys and zero reuse warnings (`errQueryArray` and `errBadOperation` are the new user-facing texts).
- `README.md` (≤ 150 lines, four ordered sections): *Why This Exists* add Without/With of a typed `Operation` (without: each transport parsed `"patient_c"`; with: a comparable value); *Step-by-Step Guide*
  add a compilable example of `Operation`, `QueryOf`+`QueryReader` and `EncodeQuery`; *Documentation Map* unchanged.
- `docs/ARCHITECTURE.md`: four sections; add `Operation` and the query codec.
- `docs/TESTS.md`: one row per test of stage 1.
- Delete the old `docs/PLAN.md` content: this file replaces it (the previous plan, "router owns WriteError", was executed — commit `74d454e`).

## Delivery check

1. `gotest`. A PR that does not compile, or whose tests could not run, is a failed delivery.
2. `langc sync .` → zero untranslated, zero reuse warnings.
3. `grep -rn "Call(op string\|Dispatch(op string\|Operation(name string" --include=*.go .` → empty.
4. Consumers (`crud`, `rpc`, `mcp`, `dbsync`, `view`, `layout`, `appkit`, `server`, `cloudflare`, `access`, `auth`) stop compiling against this change; each has its own plan. Do not touch them here.

| Stage | Files | Commit |
|---|---|---|
| 1 | `tests/op_test.go`, `tests/query_codec_test.go` | `test: … (red)` |
| 2 | `op.go`, `caller.go`, `router.go`, `mock/*`, `conformance/*` | `refactor!: …` |
| 3 | `query_codec.go` | `feat: …` |
| 4 | `lang.json`, `README.md`, `docs/*` | `docs: …` |
