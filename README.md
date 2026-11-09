# webtyp/router
<img src="docs/img/badges.svg">

Isomorphic routing, operation contracts, and query codecs for native and WebAssembly environments.

## Why This Exists

`router` defines the transport contracts connecting application modules, server transports, and client callers. Without it, each transport invents custom path parsing, string operation conventions, and conflicting query formats. With `router`, operations have a typed, comparable identity (`Operation`), routing and error handling follow RFC 9457 standards, and query arguments share a unified codec.

| Without | With |
|---|---|
| Each transport parses magic strings (`"patient_c"`) | Typed, comparable [`Operation`](op.go) `{Module, Entity, Action}` |
| Divergent query parsing logic across server and browser | Unified [`QueryReader`](query_codec.go) and [`EncodeQuery`](query_codec.go) |
| Hard-coded standard library `net/http` dependencies | Isomorphic [`Context`](context.go) and [`Router`](router.go) compiling to WASM |
| Accidental exposure of private routes | Fail-closed RBAC (`Requires`/`Public`) and introspection |

## Before You Start

- **Isomorphic package**: compiles to both native and WebAssembly (`GOOS=js GOARCH=wasm`).
- **No `net/http` in public contracts**: handlers receive `router.Context`, never `http.Request`/`http.ResponseWriter`.
- **Imports**: use `webtyp.com/fmt` for string conversions and formatting instead of stdlib `strings` or `strconv`.

## Step-by-Step Guide

### Step 1: Define an Operation, Encode Query, and Decode Arguments

**Goal:** Declare a typed operation, serialize read arguments into a query string, and decode them into a typed structure.

```go
package main

import (
	"webtyp.com/fmt"
	"webtyp.com/model"
	"webtyp.com/router"
)

type PatientFilter struct {
	Search string
	Limit  int64
}

func (f *PatientFilter) EncodeFields(w model.FieldWriter) {
	if f.Search != "" {
		w.String("search", f.Search)
	}
	if f.Limit != 0 {
		w.Int("limit", f.Limit)
	}
}

func (f *PatientFilter) DecodeFields(r model.FieldReader) {
	if s, ok := r.String("search"); ok {
		f.Search = s
	}
	if n, ok := r.Int("limit"); ok {
		f.Limit = n
	}
}

func (f *PatientFilter) IsNil() bool { return f == nil }

func main() {
	op := router.Operation{
		Module: "patient_directory",
		Entity: "patient",
		Action: model.Read,
	}

	filter := &PatientFilter{Search: "smith", Limit: 20}
	query, err := router.EncodeQuery(filter)
	if err != nil {
		fmt.Println("encode error:", err)
		return
	}

	path := "/api/" + op.Module + "/" + op.Entity + "?" + query
	extractedQuery := router.QueryOf(path)

	var decoded PatientFilter
	err = router.DecodeArgs("GET", extractedQuery, nil, &decoded, nil)
	if err != nil {
		fmt.Println("decode error:", err)
		return
	}

	fmt.Println(op.String(), decoded.Search, decoded.Limit)
}
```

**Expected result:** Output `patient_directory.patient.read smith 20`.

## Documentation Map

| Order | Document | What question it answers & when to read it |
|---|---|---|
| 1 | [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) | Component architecture, doors, operation dispatch, and security boundaries |
| 2 | [`docs/INTROSPECTION.md`](docs/INTROSPECTION.md) | Route table discovery, serialization, and verification |
| 3 | [`docs/DESIGN.md`](docs/DESIGN.md) | Architectural comparison of native route introspection vs OpenAPI |
| 4 | [`docs/TESTS.md`](docs/TESTS.md) | Index of domain test suite covering router and codec behaviors |
