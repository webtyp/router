---
PLAN: "feat(layoutscan): guard de estructura de app — la convención deja de ser prosa y pasa a fallar el build"
EXECUTOR: jules
REVIEWER: none
STATUS: running
SESSION: 11423340982369407334
---

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.

# PLAN — `router/layoutscan`: la estructura de una app webtyp se verifica

## 1. Por qué existe este plan

Toda aplicación construida sobre este framework usa la misma distribución de
directorios. Hoy esa convención vive **solo en documentación**, y el resultado
medido es que tres aplicaciones del ecosistema tienen tres estructuras
distintas. Una convención que ningún compilador respalda no se cumple —
especialmente cuando quien escribe el código es un agente sin contexto sobre el
proyecto, que coloca los archivos donde le parece razonable.

El principio que gobierna el ecosistema
([CONSTRUCTION_HARNESS](https://github.com/webtyp/app-releases/blob/main/docs/CONSTRUCTION_HARNESS.md))
lo dice así: el orden de preferencia para atrapar un error es **error de
compilación → diagnóstico ruidoso de desarrollo → (nunca) fallo silencioso**.
Este plan construye el diagnóstico ruidoso.

## 2. Dónde va, y por qué ahí

**Paquete nuevo: `router/layoutscan`**, hermano de `router/routescan`.

`routescan` ya hace exactamente esta clase de trabajo — lee el árbol de la
aplicación con `go/ast` sin ejecutarlo — y su propio doc de paquete fija las
reglas que este paquete nuevo hereda literalmente:

> *This package is BUILD TOOLING, not WASM code. It uses go/ast, go/parser and
> go/token from the standard library, which is correct and deliberate: do not
> "fix" those imports, and do not move this code into the package root — the
> root must stay WASM-safe.*

**Copia ese párrafo, adaptado, al doc de `layoutscan`.** Es la restricción más
importante del paquete: la raíz de `router` compila a WASM y `go/ast` no.

No se mete dentro de `routescan` porque son dos responsabilidades: `routescan`
lee **rutas**, `layoutscan` verifica **estructura**. Un paquete por
responsabilidad es la regla del ecosistema.

## 3. La estructura canónica que se verifica

```text
/
├── config/                  la raíz de composición
├── modules/
│   ├── server.go            (!wasm)  lista tipada []router.OperationModule
│   ├── browser.go           (neutro) lista tipada []platformd.UIModule
│   └── <module_name>/
│       ├── module.go        (neutro) ID, Label — identidad
│       ├── server.go        (!wasm)  Server(...) (router.OperationModule, error)
│       ├── browser.go       (neutro) Browser(...) (platformd.UIModule, error)
│       ├── svg.go           (!wasm)  IconSvg() *sprite.Sprite
│       ├── css.go           (!wasm)  RenderCSS() *css.Stylesheet
│       ├── model.go         (neutro) definiciones de modelo
│       └── model_orm.go     GENERADO por ormc — nunca se edita a mano
├── routes/routes.go         Register(r router.Router)
├── tests/                   TODO test del repositorio
└── web/                     solo main() delgados
```

### 3.1 Los build tags NO son simétricos, y es la regla que más se incumple

| archivo | tag exigido | razón |
|---|---|---|
| `server.go` | `//go:build !wasm` | Que nadie referencie el constructor del dominio bajo wasm es lo único que mantiene su código fuera del binario del navegador: el linker elimina la función no referenciada. Sin el tag, no puede. |
| `browser.go` | **ninguno** | El carril de tests de vista corre en stdlib, sin navegador. Con `//go:build wasm` ese carril desaparece en silencio: los tests dejan de compilarse y nadie se entera. |
| `module.go` | **ninguno** | Su `ID` lo necesitan el RBAC del servidor y el nav del cliente. |
| `svg.go`, `css.go` | `//go:build !wasm` | El extractor SSR los lee en tiempo de compilación; nunca se ejecutan en el navegador. |

## 4. La API

Un único símbolo exportado más sus tipos:

```go
// Violation is one broken rule, located. Rule is the stable identifier a test
// asserts against; Message is what a human reads.
type Violation struct {
	File    string // ruta relativa a rootDir
	Line    int    // 0 cuando la violación es del archivo entero, no de una línea
	Rule    string // una de las constantes Rule* de abajo
	Message string
}

// VerifyLayout reports EVERY structural violation under rootDir.
//
// It returns all of them, never just the first: a caller fixing one rule at a
// time re-runs the whole build for each, and an agent given a single error
// fixes it and stops. The empty slice is a conforming project.
//
// It reads source; it never builds or runs anything.
func VerifyLayout(rootDir string) []Violation
```

Las reglas, como constantes exportadas — un test debe poder afirmar contra un
identificador estable, no contra una frase:

```go
const (
	RuleForbiddenFileName  = "forbidden-file-name"
	RuleModuleSubdirectory = "module-subdirectory"
	RuleMissingBuildTag    = "missing-build-tag"
	RuleUnexpectedBuildTag = "unexpected-build-tag"
	RuleTestOutsideTests   = "test-outside-tests"
	RuleUntypedResult      = "untyped-result"
	RuleRegisterArity      = "register-arity"
)
```

## 5. Las siete reglas, exactas

### R1 · `RuleForbiddenFileName`

Dentro de `modules/<m>/`, estos nombres están prohibidos:

| nombre | por qué |
|---|---|
| `init.go` | Era el bag `[]any` con ramas `if db != nil` / `if caller != nil`. Sustituido por el par tipado. |
| `backend.go` | Nombra la capa, no el target. Es `server.go`. |
| `view.go` | Ídem. Es `browser.go`. |
| `frontend.go` | Ídem. Es `browser.go`. |
| cualquier `*_wasm.go` | El target lo lleva el build tag, no el nombre. En un módulo, un sufijo `_wasm` significa que alguien recreó el patrón de stub que este diseño eliminó. |

Los demás nombres `.go` son libres: un módulo puede tener archivos con nombre
de área (`staffpicker.go`, `scheduleview.go`) para su cableado interactivo.

Mensaje: ``modules/<m>/init.go: `init.go` is not a module file — split it into `server.go` (!wasm) and `browser.go` (neutral)``

### R2 · `RuleModuleSubdirectory`

`modules/<m>/` es plano. Cualquier subdirectorio es una violación.
Comprobación equivalente: `find modules -mindepth 2 -type d` debe salir vacío.

### R3 · `RuleMissingBuildTag`

`modules/<m>/server.go`, `svg.go` y `css.go` deben declarar `//go:build !wasm`
en su cabecera. También `modules/server.go`.

Leer el tag con `go/parser` en modo `parser.ParseComments` y examinar los
comentarios previos al `package`, o con `go/build.Context.MatchFile`. **No lo
resuelvas con una búsqueda de texto sobre el archivo completo**: la cadena
`//go:build !wasm` dentro de un comentario a mitad de archivo no es una
directiva y no debe contar.

### R4 · `RuleUnexpectedBuildTag`

`modules/<m>/browser.go` y `module.go`, y `modules/browser.go`, **no** deben
declarar ningún `//go:build`. Este es el que más silenciosamente rompe cosas:
con un tag `wasm`, el carril de tests stdlib deja de compilar ese archivo y los
tests de vista dejan de existir sin que nada lo diga.

Mensaje: ``modules/item_catalog/browser.go: browser.go must carry NO build tag — a `wasm` tag silently drops the stdlib view-test rail``

### R5 · `RuleTestOutsideTests`

Ningún `*_test.go` fuera del directorio `tests/` de la raíz. Se ignoran
`vendor/`, `.git/`, `.build/`, `web/public/` y `docs/`.

### R6 · `RuleUntypedResult`

Ninguna función declarada bajo `modules/` puede devolver `[]any` (ni
`[]interface{}`). Es la firma del bag sin tipo que este diseño elimina.

Detección: recorrer los `*ast.FuncDecl` y examinar `Type.Results`; un resultado
es violación cuando es un `*ast.ArrayType` sin `Len` cuyo `Elt` es el
identificador `any` o un `*ast.InterfaceType` vacío.

### R7 · `RuleRegisterArity`

**Solo cuando el proyecto NO tiene `web/server.go`** (es decir, usa el main
generado): la función `Register` de `routes/routes.go` debe tener exactamente
un parámetro.

Razón: el generador del main lee la aridad y renderiza `nil` para cada
parámetro posterior al router. Con dos parámetros, la app arranca **sin ningún
módulo montado** y sin ningún error — un fallo silencioso perfecto. Con
`web/server.go` presente el main generado no se usa y la regla no aplica.

`routescan` ya sabe encontrar esa función (`routerParam`); reutiliza esa lógica
en vez de reescribirla, importando `routescan` si hace falta exportar algo
mínimo — **pero no dupliques el parser**.

## 6. Comportamiento en los bordes

- `rootDir` sin directorio `modules/` → sin violaciones de R1–R4, R6. El
  proyecto puede no tener módulos.
- `rootDir` sin `routes/routes.go` → R7 no aplica.
- Un archivo que `go/parser` no puede leer → **una violación**, no un pánico ni
  un salto silencioso. Regla `RuleForbiddenFileName` no; añade el error de
  parseo como `Violation` con `Rule` igual al de la regla que se estaba
  comprobando y el mensaje del parser. Un archivo ilegible nunca se ignora.
- Directorios que se saltan siempre: `.git`, `.build`, `vendor`, `node_modules`,
  `web/public`, `docs`.

## 7. Los tests

Crear `layoutscan_test.go` con la misma forma que `routescan/routescan_test.go`
(que ya lleva `//go:build !wasm` — **este también lo lleva**, por la misma
razón: no hay árbol de archivos que leer bajo WASM).

Construir los árboles de prueba con `t.TempDir()` y escribir los archivos desde
el test. **No añadir directorios `testdata/` con proyectos de ejemplo**: el
árbol que prueba cada regla debe leerse en el propio test, junto a la
afirmación.

Un test por regla, cada uno con su caso positivo y su caso negativo:

| Test | Árbol | Esperado |
|---|---|---|
| R1 | `modules/m/init.go` | 1 violación, `RuleForbiddenFileName` |
| R1 negativo | `modules/m/{module,server,browser}.go` + `modules/m/staffpicker.go` | 0 violaciones |
| R2 | `modules/m/sub/x.go` | 1 violación, `RuleModuleSubdirectory` |
| R3 | `modules/m/server.go` sin tag | 1 violación, `RuleMissingBuildTag` |
| R3 falso positivo | `server.go` con `//go:build !wasm` correcto **y** la cadena `//go:build wasm` dentro de un comentario a mitad de archivo | 0 violaciones |
| R4 | `modules/m/browser.go` con `//go:build wasm` | 1 violación, `RuleUnexpectedBuildTag` |
| R5 | `modules/m/m_test.go` | 1 violación, `RuleTestOutsideTests` |
| R5 negativo | `tests/m_test.go` | 0 violaciones |
| R6 | `modules/m/browser.go` con `func F() []any` | 1 violación, `RuleUntypedResult` |
| R7 | `routes/routes.go` con `Register(r router.Router, m ...router.APIModule)` y sin `web/server.go` | 1 violación, `RuleRegisterArity` |
| R7 no aplica | el mismo árbol **con** `web/server.go` | 0 violaciones |
| Acumulación | árbol con violaciones de R1, R2 y R5 a la vez | **3** violaciones, no 1 |

El último es el que prueba el contrato de §4: `VerifyLayout` reporta todo, no
se detiene en la primera.

## 8. Criterios de aceptación

- [ ] `gotest ./...` verde en el repo.
- [ ] `router/layoutscan/` existe con su doc de paquete declarando que es build
      tooling y que no debe moverse a la raíz.
- [ ] `layoutscan_test.go` lleva `//go:build !wasm`.
- [ ] Las doce filas de la tabla de §7 están cubiertas y pasan.
- [ ] Las siete constantes `Rule*` están exportadas.
- [ ] `VerifyLayout` devuelve TODAS las violaciones (test de acumulación verde).
- [ ] Ningún `go/ast`, `go/parser`, `go/token`, `os` ni `path/filepath` aparece
      en la raíz del paquete `router` — solo dentro de `layoutscan/`.
- [ ] R7 reutiliza la detección de `routescan`; el parser de rutas no está
      duplicado.

## 9. Fuera de alcance

- **No integrar el guard en ningún sitio.** Este plan solo publica la función.
  Quién la ejecuta (el daemon de desarrollo, un test de la app) es otro plan en
  otro repositorio.
- No verificar `config/`, `web/` ni `edge/`. Solo las siete reglas de §5.
- No añadir una opción de configuración ni una lista de exclusiones. Una regla
  que se puede apagar no es un arnés.
- No tocar `routescan` salvo para exportar lo mínimo que R7 necesite.
