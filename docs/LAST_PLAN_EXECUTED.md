---
PLAN: "feat: strict hash-based CSP, Trusted Types, report-to and RequestTooLarge reporting in router/security"
EXECUTOR: jules
REVIEWER: none
---

> Read AGENTS.md at the repo root before any change.
> **Finish by yourself:** when the delivery check passes, commit, push and open the pull request. Never ask whether to proceed or finalize — nobody answers, and a session that waits ends without your work.

# Plan: `router/security` — CSP estricta por hashes, Trusted Types, `report-to` y reporte 413

Master: `SECURITY_OBSERVABILITY_MASTER_PLAN.md` (D3, O3b-7).

## Tools

```bash
export GOPRIVATE=webtyp.com/*,github.com/veltylabs/*
export GOFLAGS=-mod=mod
go get webtyp.com/securityevent@v0.0.2
go install webtyp.com/devflow/cmd/gotest@latest
export PATH="$PATH:$(go env GOPATH)/bin"
```

## Ecosystem search

- Buscado en `~/Dev/Project/webtyp` (`router/security`, `server/httpd`, `securityevent`).
- `router/security` ya es el contrato único de cabeceras de seguridad y límite de cuerpo (413) compartido por `server/httpd` y `cloudflare/edge`.
- `webtyp.com/securityevent` (`v0.0.2`) ya define `securityevent.Reporter` y `securityevent.RequestTooLarge`.
- **Decisión:** extender `router/security.Policy` y reutilizar `webtyp.com/securityevent` para reportar 413.

## Design gate

1. **Prior art:**
   - [web.dev — Strict CSP](https://web.dev/articles/strict-csp): para páginas estáticas, `script-src 'sha384-...' 'strict-dynamic' 'wasm-unsafe-eval'; object-src 'none'; base-uri 'none'`.
   - [MDN — CSP `report-to` y `Reporting-Endpoints`](https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Content-Security-Policy/report-to): `Reporting-Endpoints: csp="<path>"` junto a `report-to csp` en la CSP.
   - [MDN — `require-trusted-types-for`](https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Content-Security-Policy/require-trusted-types-for): `require-trusted-types-for 'script'` bloquea sinks XSS (`innerHTML`, `insertAdjacentHTML`).
2. **Novice-name test:**
   - `Policy.ScriptHashes(hashes ...string) Policy` — recibe hashes SRI (`"sha384-..."`) y emite `script-src 'sha384-...' 'strict-dynamic' 'wasm-unsafe-eval'`.
   - `Policy.ReportEndpoint(path string) Policy` — configura la ruta de reportes (`Reporting-Endpoints: csp="<path>"` + `report-to csp`).
   - `Policy.WithReporter(r securityevent.Reporter) Policy` — registra el `securityevent.Reporter` para informar `RequestTooLarge` al responder 413.
   - `HeaderReportingEndpoints = "Reporting-Endpoints"`.
3. **Complexity ledger:**
   - Conceptos a aprender: `+3` métodos fluidos en `Policy` / `-0`
   - Archivos a tocar: `0`
   - Líneas en el call site (`server/httpd` y `edge`): `+3` líneas
   - Formas de hacer lo mismo: `0` (una sola `Policy` para origen y edge).
4. **Where it belongs:** `router/security` es la capa compartida por `server/httpd` y `cloudflare/edge`; `securityevent` es isomórfico y WASM-safe (solo importa `events`, `model`, `time`, `msgtype`), por lo que no rompe el build WASM del edge ni crea ciclos.
5. **What does this change delete?** Nada paralelo: fortalece `DefaultContentSecurityPolicy` añadiendo `require-trusted-types-for 'script'` por defecto.

## APIs used

| Symbol | Exact signature | Tag | File |
|---|---|---|---|
| `securityevent.Reporter` | `type Reporter struct { ... }` | `v0.0.2` | `webtyp.com/securityevent` |
| `(securityevent.Reporter).Report` | `func (r Reporter) Report(e Event) error` | `v0.0.2` | `webtyp.com/securityevent` |
| `securityevent.RequestTooLarge` | `const RequestTooLarge Kind` | `v0.0.2` | `webtyp.com/securityevent` |
| `router.ContextKeyRemoteAddr` | `const ContextKeyRemoteAddr` | `v0.5.7` | `webtyp.com/router` |

If a signature in the published tag differs from this table, stop and name the symbol in the PR; never adapt it by guessing.

## Reglas de entrega y anti-footguns

- `router/security` compila a WASM (edge Worker): usar `webtyp.com/fmt`, **no usar stdlib `strings`/`fmt`/`net/http` ni `map`**.
- No editar `go.mod`/`go.sum` a mano; usar `go get` y `go mod tidy`.
- Hacer commit después de cada etapa. No usar `git reset`, `git restore`, `git checkout -- <file>` ni `git stash`.

---

## Etapa 1: Diagrama y pruebas (TDD)

1. Crear `docs/diagrams/SECURITY_POLICY.md` (contexto, `flowchart TD`, walkthrough numerado) mostrando:
   - Solicitud entra a `Policy.Middleware()` → añade cabeceras (`Content-Security-Policy` con `require-trusted-types-for 'script'`, hashes `'sha384-...' 'strict-dynamic'`, `report-to csp`, `Reporting-Endpoints`) → verifica `bodyAllowed(ctx)`.
   - Si excede `maxBytes()`: reporta `securityevent.RequestTooLarge` (si reporter configurado) y responde `413` sin llamar `next`.
   - Si cabe: llama `next(ctx)`.
2. En `security/security_test.go` (y registrar en `docs/TESTS.md` bajo `| Test | Rule |`):
   - `TestPolicy_DefaultIncludesTrustedTypes`: la CSP por defecto incluye `require-trusted-types-for 'script'`.
   - `TestPolicy_ScriptHashesUsesStrictDynamic`: `Policy{}.ScriptHashes("sha384-abc")` emite `script-src 'sha384-abc' 'strict-dynamic' 'wasm-unsafe-eval'` (sin `'self'` en `script-src`).
   - `TestPolicy_ReportEndpointEmitsHeaderAndDirective`: `Policy{}.ReportEndpoint("/_csp-report")` emite `Reporting-Endpoints: csp="/_csp-report"` y `report-to csp` en la CSP.
   - `TestPolicy_BodyTooLargeReportsSecurityEvent`: con `WithReporter(rep)` y cuerpo > `MaxRequestBytes`, responde 413 y publica exactamente 1 `securityevent.Event` con `Kind == securityevent.RequestTooLarge`.

## Etapa 2: Implementar extensiones en `security/security.go` y `security/body.go`

1. En `security/security.go`:
   - Agregar constante `HeaderReportingEndpoints = "Reporting-Endpoints"`.
   - Agregar `directiveRequireTrustedTypes = "require-trusted-types-for"`, `directiveReportTo = "report-to"`, `reportGroupCSP = "csp"`, `sourceStrictDynamic = "'strict-dynamic'"`, `sourceWasmUnsafeEval = "'wasm-unsafe-eval'"`, `trustedTypesScript = "'script'"`.
   - Añadir `; require-trusted-types-for 'script'` al final de `DefaultContentSecurityPolicy`.
   - Agregar campos en `Policy`:
     ```go
     scriptHashes   []string
     reportEndpoint string
     reporter       securityevent.Reporter
     hasReporter    bool
     ```
   - Agregar métodos inmutables:
     - `func (p Policy) ScriptHashes(hashes ...string) Policy`
     - `func (p Policy) ReportEndpoint(path string) Policy`
     - `func (p Policy) WithReporter(r securityevent.Reporter) Policy`
   - En `scriptSrcValue()`: si `len(p.scriptHashes) > 0`, construir `'sha384-...' ... 'strict-dynamic' 'wasm-unsafe-eval'` (+ `p.scripts` si hay). Si está vacío, mantener `extendSource(baseScriptSrc, p.scripts)`.
   - En `contentSecurityPolicy()`: incluir `require-trusted-types-for 'script'` siempre, y `report-to csp` cuando `p.reportEndpoint != ""`.
   - En `Middleware()`:
     - Si `p.reportEndpoint != ""`, `ctx.SetHeader(HeaderReportingEndpoints, reportGroupCSP+"=\""+p.reportEndpoint+"\"")`.
     - Cuando `!p.bodyAllowed(ctx)`:
       ```go
       if p.hasReporter {
           _ = p.reporter.Report(securityevent.Event{
               Kind: securityevent.RequestTooLarge,
               IP:   ctx.Value(router.ContextKeyRemoteAddr),
           })
       }
       ctx.WriteStatus(statusRequestEntityTooLarge)
       return
       ```

## Etapa 3: Documentación y entrega

1. Actualizar `README.md` (≤ 150 líneas, 4 secciones obligatorias), `docs/ARCHITECTURE.md` (4 secciones) y `docs/TESTS.md`.
2. Ejecutar `gotest`.

## Tabla de etapas

| Etapa | Archivos | Qué |
|---|---|---|
| 1 | `docs/diagrams/SECURITY_POLICY.md`, `security/security_test.go`, `docs/TESTS.md` | Diagrama y pruebas TDD |
| 2 | `security/security.go`, `security/body.go` | `ScriptHashes`, `ReportEndpoint`, `WithReporter`, Trusted Types y reporte 413 |
| 3 | `README.md`, `docs/ARCHITECTURE.md`, `docs/TESTS.md` | Documentación y `gotest` verde |
