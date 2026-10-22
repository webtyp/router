---
PLAN: "feat!: layoutscan enforces one main per folder in web/"
EXECUTOR: jules
REVIEWER: none
STATUS: review
SESSION: 4846325970383717016
PR: https://github.com/webtyp/router/pull/12
REVIEWED_COMMIT: 3ce0fe0e07b0ce08bb62f5648bf90aa3e1f1eec8
---

> Read AGENTS.md at the repo root before any change.

# layoutscan: `web/<binary>/main.go`

## Tools

```bash
go install webtyp.com/devflow/cmd/gotest@latest
```

## Why

The ecosystem moves every binary main into its own folder under `web/` (the folder is the
binary): `web/browser/main.go` → `web/public/browser.wasm`, `web/server/main.go` →
`web/server/server`, `web/edge/main.go` → `web/edge/edge.wasm` + `edge.js`. A main carries **no
destination tag** (`browser`/`!browser`): its folder separates it, and gopls never analyses a file
gated on a custom positive tag. A main may carry a platform tag: `wasm` for browser/edge, `!wasm`
for server. No backwards compatibility.

Scope: only `checkWebMains` and its messages. `checkModuleFile` (rules for `modules/<m>/`) does
**not** change in this plan.

## Stage 1 — `layoutscan/layoutscan.go`: `checkWebMains`

Rewrite `checkWebMains(rootDir string, violations *[]Violation)` with this behaviour (keep the
`edge/` check at its top, message updated):

| Found | Rule | Message (verbatim) |
|---|---|---|
| directory `edge/` at the root | `RuleEdgeDirectory` | `edge/: the Worker main is web/edge/main.go; delete edge/` |
| any `.go` file (not `_test.go`) directly in `web/`, named `client.go` | `RuleUnknownWebMain` | `web/client.go: move to web/browser/main.go (the binary is browser.wasm)` |
| any other `.go` file (not `_test.go`) directly in `web/` | `RuleUnknownWebMain` | `web/<name>: each binary has its own folder: web/browser/main.go, web/server/main.go, web/edge/main.go` |
| a subdirectory of `web/` other than `browser`, `server`, `edge`, `public` that contains `.go` files | `RuleUnknownWebMain` | `web/<dir>/: unknown binary folder; web/ holds browser/, server/, edge/ and public/` |
| `web/<b>/main.go` (b ∈ browser, server, edge) whose `//go:build` expression mentions the tag `browser` (positive or negated) | `RuleWrongBuildTag` | `web/<b>/main.go: a main carries no destination tag; its folder already separates it` |
| `web/browser/main.go` or `web/edge/main.go` with a `//go:build` other than none or exactly `wasm` | `RuleWrongBuildTag` | `web/<b>/main.go: carries ` + "`//go:build <got>`" + `, allowed: none or ` + "`//go:build wasm`" |
| `web/server/main.go` with a `//go:build` other than none or exactly `!wasm` | `RuleWrongBuildTag` | `web/server/main.go: carries ` + "`//go:build <got>`" + `, allowed: none or ` + "`//go:build !wasm`" |

- "Mentions the tag `browser`": parse with `constraint.Parse` (already imported) and walk the
  expression for a `*constraint.TagExpr` whose `Tag == tagBrowser`.
- Read the header with the existing `parseHeaderComments` helper; reuse the existing way the file
  extracts the `//go:build` line (see `checkExactTag`) for `<got>`.
- `web/public/` is never scanned.
- Delete `tagEdge` if nothing else uses it after the rewrite (`grep -n tagEdge layoutscan/*.go`).
- Update the comment above `checkWebMains`: "enforces one main per folder in web/ and no edge/
  at the root".

## Stage 2 — tests `layoutscan/layoutscan_test.go`

Replace the tests that build `web/browser.go`/`web/edge.go`/`web/server.go` fixtures with a table
test `TestCheckWebMains_OneMainPerFolder` (same helpers the file already uses to write a temp
project and run the scan). Each row: files written → expected rules (empty = no violation).

| Files | Expected |
|---|---|
| `web/browser/main.go` (no tag), `web/server/main.go` (`!wasm`), `web/edge/main.go` (`wasm`) | none |
| `web/browser/main.go` (`wasm`) | none |
| `web/browser/main.go` (`browser`) | `RuleWrongBuildTag` |
| `web/server/main.go` (`!browser`) | `RuleWrongBuildTag` |
| `web/edge/main.go` (`wasm && !browser`) | `RuleWrongBuildTag` |
| `web/browser.go` | `RuleUnknownWebMain` |
| `web/client.go` | `RuleUnknownWebMain` (message contains `web/browser/main.go`) |
| `web/tools/main.go` | `RuleUnknownWebMain` |
| `web/public/x.go` | none |
| directory `edge/` with `edge/main.go` | `RuleEdgeDirectory` |

Every fixture file is `package main` with `func main() {}`.

## Stage 3 — docs

- `README.md` (the table near line 190): rows become
  `web/browser/main.go` | none or `wasm` | `web/public/browser.wasm`;
  `web/edge/main.go` | none or `wasm` | `web/edge/edge.wasm` (Worker);
  `web/server/main.go` | none or `!wasm` | `web/server/server`. Add one sentence below: "A main
  carries no destination tag: its folder separates it."
- `AGENTS.md`: do not edit the `<!-- webtyp:base -->` block (it is refreshed before dispatch).

## Delivery check

```bash
gotest
grep -rn 'web/browser\.go\|web/edge\.go\|web/server\.go' --include=*.go --include=README.md . | grep -v docs/PLAN.md   # only the legacy-detection message/test lines for web/browser.go
```

A PR whose tests could not run is a failed delivery. Environment:
`GOPRIVATE=webtyp.com/*,github.com/veltylabs/*`, `GOFLAGS=-mod=mod`.

Never pause to ask permission or report progress: when the delivery check passes, commit, push
and open the PR. Stuck for 30 minutes → open the PR anyway listing what fails.

## Stages

| # | Files | Result |
|---|---|---|
| 1 | `layoutscan/layoutscan.go` | rules for `web/<binary>/main.go` |
| 2 | `layoutscan/layoutscan_test.go` | table test |
| 3 | `README.md` | docs |

## Executor notes
No deviations from the plan were necessary. All stages implemented successfully.
