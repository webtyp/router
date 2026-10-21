---
PLAN: "feat(layoutscan)!: build-destination rules — exact tags for module and web/ mains (browser / wasm && !browser / !wasm), no edge/, browser never with !wasm"
EXECUTOR: jules
REVIEWER: none
STATUS: running
SESSION: 42405233861288978
---

> Read AGENTS.md at the repo root before any change.

# layoutscan: verify the build-destination layout

## Why

The ecosystem changed its build tags (breaking, no compatibility period). The **file name says
which binary it goes to**, and there are two tag axes:

- **platform** `wasm` / `!wasm`, set by the compiler (`GOARCH=wasm`: browser **and** Cloudflare Worker);
- **destination** `browser` / `!browser`, passed only by the client compiler (`-tags browser`).

| File | Exact build line | Binary |
|---|---|---|
| `web/browser.go` | `//go:build browser` | `web/public/browser.wasm` |
| `web/edge.go` | `//go:build wasm && !browser` | `web/edge.wasm` (Worker) |
| `web/server.go` | `//go:build !wasm` | `web/server` |
| `modules/server.go`, `modules/<m>/server.go` | `//go:build !browser` | native server + Worker |
| `modules/browser.go`, `modules/<m>/browser.go` | `//go:build browser` | browser only |
| `modules/<m>/module.go` | none | all |
| `modules/<m>/svg.go`, `modules/<m>/css.go` | `//go:build !wasm` | read by `sitec` at build time |

The `edge/` root directory no longer exists (its `main.go` became `web/edge.go`), `web/client.go`
became `web/browser.go`, and `browser` combined with `!wasm` is impossible (the browser is wasm).
The Worker is defined by the **absence** of `browser`, so a mislabelled main would silently build
as another destination: `layoutscan.VerifyLayout` must catch every one of these.

This package is build tooling: `go/ast`, `go/parser`, `go/token`, `go/build/constraint`, `os`,
`path/filepath` from the standard library are correct here (see the package comment).

## Tools

```bash
go install webtyp.com/devflow/cmd/gotest@latest
```

## Environment

```bash
export GOPRIVATE=webtyp.com/*,github.com/veltylabs/*
export GOFLAGS=-mod=mod
```

A PR that does not compile, or whose tests could not run, is a failed delivery. Never pause to ask
permission or report progress: when the Delivery check passes, commit, push and open the PR.
Stuck for 30 minutes: open the PR anyway and list what fails.

## APIs used

| Symbol | Exact signature | Where |
|---|---|---|
| `constraint.IsGoBuild` | `func IsGoBuild(line string) bool` | stdlib `go/build/constraint` |
| `constraint.Parse` | `func Parse(line string) (Expr, error)` | stdlib |
| `constraint.Expr.Eval` | `Eval(ok func(tag string) bool) bool` | stdlib |
| `constraint.Expr.String` | `String() string` (canonical form: `wasm && !browser`) | stdlib |
| `parseHeaderComments` | `func parseHeaderComments(fset *token.FileSet, f *ast.File) []string` | `layoutscan/layoutscan.go:296` (existing) |
| `isIgnoredDir` | `func isIgnoredDir(rel string) bool` | `layoutscan/layoutscan.go:62` (existing) |

If a signature differs, stop and name it in the PR; never adapt it by guessing.

## Stage 1 — one exact-tag check replaces R3/R4 (`layoutscan/layoutscan.go`)

1. Add constants next to the existing `Rule*`:

   ```go
   RuleWrongBuildTag      = "wrong-build-tag"
   RuleUnknownWebMain     = "unknown-web-main"
   RuleEdgeDirectory      = "edge-directory"
   RuleBrowserWithoutWasm = "browser-without-wasm"
   ```
   and the tag texts:

   ```go
   const (
   	tagBrowser    = "browser"
   	tagNotBrowser = "!browser"
   	tagNotWasm    = "!wasm"
   	tagEdge       = "wasm && !browser"
   	tagWasm       = "wasm"
   )
   ```
2. New unexported helper:

   ```go
   // buildLine returns the //go:build expression of the file header, or "" when
   // there is none. The text is the canonical form of constraint.Expr.String().
   func buildLine(header []string) (string, error)
   ```
   It takes the first header line for which `constraint.IsGoBuild(line)` is true, parses it with
   `constraint.Parse` and returns `expr.String()`; a parse error is returned as is.
3. New unexported helper `func checkExactTag(relFile, want string, header []string, violations *[]Violation)`:

   | `buildLine` result | `want` | Violation |
   |---|---|---|
   | `""` | `""` | none |
   | `""` | non-empty | `RuleMissingBuildTag`, message `"<relFile>: must carry ` + "`//go:build <want>`" + `"` |
   | non-empty `got` | `""` | `RuleUnexpectedBuildTag`, message `"<relFile>: must carry NO build tag (found ` + "`//go:build <got>`" + `)"` |
   | `got != want` | non-empty | `RuleWrongBuildTag`, message `"<relFile>: carries ` + "`//go:build <got>`" + `, must carry ` + "`//go:build <want>`" + `"` |
   | parse error | any | `RuleWrongBuildTag`, message = `relFile + ": " + err.Error()` |

   `Line` is always `0`.
4. In `checkModuleFile`, delete `mustHaveNotWasm`, `mustHaveNoBuildTag`, the two header loops and
   the old messages (`"server.go, svg.go and css.go must carry"`, `"browser.go must carry NO build tag"`).
   Replace them with a `want` chosen from the file name:

   | File | `want` |
   |---|---|
   | `server.go` (registry or module) | `tagNotBrowser` |
   | `browser.go` (registry or module) | `tagBrowser` |
   | `module.go` (in a module dir) | `""` |
   | `svg.go`, `css.go` (in a module dir) | `tagNotWasm` |
   | any other file | not checked |

   then `checkExactTag(relFile, want, headerComments, violations)`. On a parse error of the whole
   file keep today's behaviour but use `RuleWrongBuildTag` for the files that have a `want`.
5. `isForbiddenFileName`: add `client.go`. The `init.go` message becomes
   ``"%s: `init.go` is not a module file — split it into `server.go` (`//go:build !browser`) and `browser.go` (`//go:build browser`)"``.

## Stage 2 — `web/` mains and `edge/` (`layoutscan/layoutscan.go`)

New unexported `func checkWebMains(rootDir string, violations *[]Violation)`, called from
`VerifyLayout` after `checkModules`:

1. If `rootDir/edge` is a directory: one violation `RuleEdgeDirectory`, `File: "edge"`, message
   ``"edge/: the Worker main is web/edge.go (`//go:build wasm && !browser`); delete edge/"``.
2. For every `*.go` file directly in `rootDir/web` (not in subdirectories, not `_test.go`):

   | File | Check |
   |---|---|
   | `browser.go` | `checkExactTag("web/browser.go", tagBrowser, …)` |
   | `edge.go` | `checkExactTag("web/edge.go", tagEdge, …)` |
   | `server.go` | `checkExactTag("web/server.go", tagNotWasm, …)` |
   | `client.go` | `RuleUnknownWebMain`, message ``"web/client.go: rename to web/browser.go with `//go:build browser` (the binary is browser.wasm)"`` |
   | any other | `RuleUnknownWebMain`, message ``"web/<name>: web/ holds only browser.go, edge.go and server.go"`` |

   The header is read with `parser.ParseFile(fset, path, nil, parser.ParseComments)` +
   `parseHeaderComments`, like `checkModuleFile`. No `web/` directory → nothing to check.

## Stage 3 — `browser` never with `!wasm` (`layoutscan/layoutscan.go`)

New unexported `func checkBrowserWithoutWasm(rootDir string, violations *[]Violation)`, called from
`VerifyLayout`. It walks every `.go` file under `rootDir` (skip dirs with `isIgnoredDir`, same walk
shape as `checkTestsOutsideTests`, **including** `tests/`), reads its build line with `buildLine`
and reports `RuleBrowserWithoutWasm` when this function returns true:

```go
// onlyBuildsAsNativeBrowser reports whether the expression can hold with
// browser=true only when wasm=false — a build that never exists, because the
// browser binary is always wasm.
func onlyBuildsAsNativeBrowser(expr constraint.Expr) bool {
	for _, others := range []bool{false, true} {
		eval := func(browser, wasm bool) bool {
			return expr.Eval(func(tag string) bool {
				switch tag {
				case tagBrowser:
					return browser
				case tagWasm:
					return wasm
				}
				return others
			})
		}
		if eval(true, false) && !eval(true, true) {
			return true
		}
	}
	return false
}
```

Message: ``"<rel>: `//go:build <got>` combines browser with !wasm; the browser binary is always wasm"``.

| Build line | Violation |
|---|---|
| `//go:build browser && !wasm` | yes |
| `//go:build browser` | no |
| `//go:build !wasm` | no |
| `//go:build wasm && !browser` | no |
| none | no |

## Stage 4 — tests (`layoutscan/layoutscan_test.go`, same package and helpers as today)

Update the existing R3/R4 tests to the new tags (a module `server.go` with `//go:build !wasm` is
now `RuleWrongBuildTag`; a module `browser.go` without a tag is now `RuleMissingBuildTag`). Add,
each with `writeFile` into `t.TempDir()` and `VerifyLayout(root)`:

| # | Files | Expected rules (exactly) |
|---|---|---|
| T1 | `modules/a/server.go` `//go:build !browser`, `modules/a/browser.go` `//go:build browser`, `modules/a/module.go` no tag, `modules/a/css.go` `//go:build !wasm` | none |
| T2 | `modules/a/server.go` `//go:build !wasm` | `RuleWrongBuildTag` |
| T3 | `modules/a/browser.go` no tag | `RuleMissingBuildTag` |
| T4 | `modules/a/module.go` `//go:build browser` | `RuleUnexpectedBuildTag` |
| T5 | `modules/server.go` `//go:build !browser`, `modules/browser.go` `//go:build browser` | none |
| T6 | `web/browser.go` `//go:build browser`, `web/edge.go` `//go:build wasm && !browser`, `web/server.go` `//go:build !wasm` | none |
| T7 | `web/edge.go` `//go:build wasm` | `RuleWrongBuildTag` |
| T8 | `web/client.go` `//go:build wasm` | `RuleUnknownWebMain` |
| T9 | `edge/main.go` `//go:build wasm` | `RuleEdgeDirectory` |
| T10 | `config/x.go` `//go:build browser && !wasm` | `RuleBrowserWithoutWasm` |
| T11 | `modules/a/client.go` no tag | `RuleForbiddenFileName` |
| T12 | `web/public/app.go` (any content) | none (ignored dir) |

Every file content is `"<build line>\n\npackage x\n"` (package `main` for `web/*.go`).

## Stage 5 — docs

- `README.md`: the `layoutscan` section (create it if missing, after the routing sections) with the
  table of «Why» and the rule constants.
- `docs/ARCHITECTURE.md`: one paragraph «layoutscan» pointing to the README table.
- `grep -rn "must carry NO build tag — a" --include=*.go .` → empty;
  `grep -rn "browser.go.*neutral\|(neutral)" layoutscan/` → empty.

## Delivery check

```bash
go build ./...
gotest
git diff --stat main   # layoutscan/layoutscan.go, layoutscan/layoutscan_test.go, README.md, docs/ARCHITECTURE.md
```

## Stages

| # | Files | Done when |
|---|---|---|
| 1 | `layoutscan/layoutscan.go` | exact-tag check, old R3/R4 code deleted |
| 2 | `layoutscan/layoutscan.go` | `checkWebMains` wired |
| 3 | `layoutscan/layoutscan.go` | `checkBrowserWithoutWasm` wired |
| 4 | `layoutscan/layoutscan_test.go` | T1–T12 and updated R3/R4 green |
| 5 | `README.md`, `docs/ARCHITECTURE.md` | docs match, greps empty |
