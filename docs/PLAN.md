---
PLAN: "fix(layoutscan): R2 exime `docs/` dentro de un módulo — la documentación vive junto a lo que documenta"
EXECUTOR: jules
REVIEWER: none
STATUS: review
SESSION: 5659281628285121565
PR: https://github.com/webtyp/router/pull/6
---

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.

# PLAN — `docs/` no es un subdirectorio prohibido

## 1. Por qué

`layoutscan.VerifyLayout` se estrenó contra el árbol real de
`veltylabs/mjosefa-cms` y encontró 42 violaciones. Las 42 son correctas salvo
una clase:

```
[module-subdirectory] modules/personal/docs
[module-subdirectory] modules/appointment_booking/docs
[module-subdirectory] modules/business_calendar/docs
[module-subdirectory] modules/item_catalog/docs
[module-subdirectory] modules/item_catalog/data
```

Los cuatro `docs/` contienen documentación real del módulo (y a su vez un
`docs/reference/`). La regla R2 los marca porque un módulo debe ser plano.

**R2 existe por una razón concreta: un subdirectorio dentro de un módulo es
casi siempre código escondiéndose** — un `internal/`, un paquete auxiliar, una
copia local de algo que debía vivir aguas arriba. Esa razón no alcanza a la
documentación. La doc de un módulo junto al módulo es lo correcto: se lee donde
se trabaja y se mueve con él.

`data/` **sigue prohibido**, deliberadamente. Datos junto al código sí son la
señal que R2 busca: algo con estado y sin dueño declarado.

## 2. El cambio

En `layoutscan/layoutscan.go`, función `checkModules`. La rama que hoy dice:

```go
			if mEntry.IsDir() {
				// R2: RuleModuleSubdirectory
				*violations = append(*violations, Violation{...})
				continue
			}
```

pasa a eximir exactamente un nombre:

```go
			if mEntry.IsDir() {
				// `docs` es la única excepción a la planitud de un módulo: la
				// razón de R2 es que un subdirectorio esconde código sin dueño
				// (un internal/, un paquete auxiliar, una copia local de algo
				// que debía estar aguas arriba). La documentación no es eso —
				// vive junto a lo que documenta y se mueve con ello. `data/`
				// NO se exime: datos junto al código sí son lo que R2 busca.
				if subName == "docs" {
					continue
				}
				// R2: RuleModuleSubdirectory
				*violations = append(*violations, Violation{...})
				continue
			}
```

**Es una exención por nombre exacto, no un prefijo ni un patrón.** `docs_old`,
`mydocs` y `docs2` siguen siendo violaciones. Y no se recorre el interior de
`docs/`: lo que haya dentro (incluido `docs/reference/`) queda fuera del
alcance de `layoutscan` por completo.

No hay flag, opción ni lista configurable. Una regla que se puede apagar no es
un arnés.

## 3. La documentación del paquete

El doc de `RuleModuleSubdirectory` — y cualquier comentario que enumere las
reglas — debe decir que `docs/` está exento y por qué. Quien lea la constante
tiene que saberlo sin abrir la implementación.

## 4. Los tests

En `layoutscan_test.go`, junto a `TestR2_Subdirectory` que ya existe:

| Test | Árbol | Esperado |
|---|---|---|
| `TestR2_DocsExempt` | `modules/m/docs/README.md` | **0** violaciones |
| `TestR2_DocsNestedExempt` | `modules/m/docs/reference/x.md` | **0** violaciones |
| `TestR2_DataNotExempt` | `modules/m/data/seed.sql` | 1 violación, `RuleModuleSubdirectory` |
| `TestR2_DocsPrefixNotExempt` | `modules/m/docs_old/x.md` | 1 violación, `RuleModuleSubdirectory` |

Cada árbol necesita además los archivos de módulo válidos (`module.go`,
`server.go`, `browser.go`) para que el conteo esperado no arrastre violaciones
de otras reglas — sigue el patrón que ya usan los tests existentes.

`TestR2_Subdirectory`, el que ya existe, **no debe cambiar de expectativa**: si
usa un subdirectorio llamado `docs`, renómbralo en el test a otro nombre para
que siga probando lo que probaba.

## 5. Criterios de aceptación

- [ ] `gotest ./...` verde en el repo.
- [ ] Los cuatro tests de §4 existen y pasan.
- [ ] `TestR2_Subdirectory` sigue existiendo y sigue detectando una violación.
- [ ] La exención es por nombre exacto `docs`, nunca por prefijo.
- [ ] Ningún flag ni opción de configuración.
- [ ] El doc de `RuleModuleSubdirectory` menciona la exención.
- [ ] Ninguna otra regla cambia de comportamiento.

## 6. Fuera de alcance

- No tocar R1, R3, R4, R5, R6 ni R7.
- No eximir `data/` ni ningún otro nombre.
- No inspeccionar el contenido de `docs/`.
