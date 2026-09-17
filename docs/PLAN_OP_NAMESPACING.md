---
PLAN: "feat!: el nombre de una operación lo cualifica su módulo — la colisión deja de ser representable"
EXECUTOR: jules
REVIEWER: none
---

> **PENDIENTE — NO DESPACHAR TODAVÍA.** Este plan cambia los nombres de
> operación en el cable, que es un cambio incompatible que atraviesa `router`,
> `mcp`, las librerías de dominio de `veltylabs/modules/*` y las tres apps a la
> vez. Requiere la aprobación explícita del dueño del ecosistema antes de
> despacharse. El contenido está completo: cuando se apruebe, se despacha tal
> cual.
>
> Dispatched via the CodeJob workflow. See skill: agents-workflow.

# PLAN — El nombre de una operación no tiene dueño

## 1. El defecto, con la evidencia

`mcp/harvest.go` cosecha las operaciones de varios módulos en un único
registro, y ante un nombre repetido entra en pánico:

```go
func (r *opRegistry) Operation(name string, h router.HandlerFunc) router.Route {
	for _, t := range r.tools {
		if t.Name == name {
			panic("mcp: duplicate tool name \"" + name + "\" — …")
		}
	}
	...
}
```

El nombre es un `string` plano que cada librería de dominio elige por su cuenta.
Nada lo relaciona con el módulo que lo publica. Los autores lo cualifican **a
mano**, y funciona hasta que dos no lo hacen igual:

| Librería | Constante | Valor en el cable |
|---|---|---|
| `veltylabs/item_catalog` | `OpListItems` | `"list_catalog_items"` ← cualificado a mano |
| `veltylabs/business_calendar` | `OpGetDayBounds` | `"get_day_bounds"` |
| `veltylabs/appointment_booking` | `OpGetDayBounds` | `"get_day_bounds"` ← **idéntico** |

Montar los dos últimos en el mismo servidor MCP hace estallar el arranque. Las
dos librerías son correctas por separado: ninguna sabe de la otra, y ninguna
hizo nada mal.

**Lo que provocó aguas abajo.** Una app en producción escribió 55 líneas —
`opFilter`, `filteringRegistry`, `noopRoute` — para interceptar el registro y
descartar una de las dos operaciones antes de que llegara al registro real. Eso
es un fork del registro de `router` viviendo en el `config/` de una aplicación:
exactamente lo que el
[CONSTRUCTION_HARNESS](https://github.com/webtyp/app-releases/blob/main/docs/CONSTRUCTION_HARNESS.md)
prohíbe (*«Never wrap a library to fix its behaviour»*).

## 2. Por qué no basta con diagnosticar mejor

La tentación es convertir el pánico en un error legible y que la app resuelva.
Es incorrecto por el principio 3 del harness — **illegal states
unrepresentable**: mientras el nombre sea un `string` libre, dos módulos pueden
seguir eligiendo el mismo, y cada app nueva vuelve a chocar. Un diagnóstico
mejor no elimina el estado ilegal; solo lo señala más tarde.

El módulo ya declara su identidad: `router.OperationModule` incorpora
`model.ModuleNaming`, o sea `ModelName() string` (`"item_catalog"`,
`"business_calendar"`). La información para cualificar el nombre **ya está en la
costura**; simplemente no se usa.

## 3. El cambio

`HarvestOps` ya recorre los módulos uno a uno, así que sabe de quién es cada
operación que se registra:

```go
func HarvestOps(modules ...router.OperationModule) ToolProvider {
	reg := &opRegistry{}
	for _, m := range modules {
		m.MountOperations(reg)   // ← aquí se conoce m.ModelName()
	}
	return staticProvider(reg.tools)
}
```

**El registro pasa a cualificar cada nombre con el módulo que lo está
montando.** `opRegistry` gana un campo con el `ModelName()` del módulo en curso,
que `HarvestOps` fija antes de cada `MountOperations` y limpia después. El
nombre que llega al `Tool` es `<modelName>.<name>`:

- `business_calendar.get_day_bounds`
- `appointment_booking.get_day_bounds`

Dos nombres distintos. La colisión deja de existir, y el pánico por duplicado
queda solo para el caso que de verdad es un error: el mismo módulo registrando
dos veces el mismo nombre, o el mismo módulo pasado dos veces a `HarvestOps`.

**Un módulo que no declara `ModelName()` no puede montar operaciones.** Si
`ModelName()` devuelve `""`, `HarvestOps` devuelve error — no cualifica con un
prefijo vacío ni deja pasar el nombre desnudo.

### 3.1 El lado del cliente no puede escribir el prefijo a mano

Si el cliente tuviera que componer `"business_calendar." + OpGetDayBounds`, se
habría movido el problema, no resuelto: un literal mal escrito falla en runtime
con «unknown tool». El nombre cualificado tiene que salir del mismo sitio en
los dos extremos.

`view.NewCallerLister` recibe hoy un `view.Ops{List, Save, Delete}` de strings
planos. Pasa a recibir además el módulo (o su `ModelName()`), y compone el
nombre cualificado internamente. Ningún autor de app vuelve a escribir un
nombre de operación completo.

**Definir esa firma exacta es parte de este plan**, y debe cumplir el principio
7: que el autocompletado baste. Si al escribirla hace falta que el consumidor
declare algo local para nombrar lo que cruza, la costura sigue rota y hay que
decirlo antes de implementar.

## 4. Radio de impacto — hay que contarlo antes de empezar

Es un cambio incompatible en el cable. Todo esto se mueve **en la misma ola**:

| Repositorio | Qué cambia |
|---|---|
| `webtyp/router` | `OperationRegistry`: el contrato de cualificación; el helper del lado cliente |
| `webtyp/mcp` | `opRegistry` cualifica; el pánico por duplicado se acota |
| `webtyp/view` | `NewCallerLister` / `view.Ops` toman la identidad del módulo |
| `veltylabs/modules/*` (10 repos) | Nada en sus constantes; sí en cómo construyen sus `Presenter` |
| `veltylabs/mjosefa-cms` | **Se borran** `opFilter`, `filteringRegistry`, `noopRoute` de `config/server.go` |
| `veltylabs/iam`, `veltylabs/misitio` | Adoptan el nuevo `view.Ops` |

Hacerlo ahora cuesta lo que cuesta. Con una sola app en producción es el momento
más barato que va a haber.

## 5. Criterios de aceptación

- [ ] `gotest ./...` verde en `router` y en `mcp`.
- [ ] Un test que cosecha **dos** módulos que registran el mismo nombre desnudo
      y afirma que ambas operaciones quedan registradas, con nombres distintos.
      Hoy ese test entra en pánico: escríbelo primero y compruébalo.
- [ ] Un test que afirma que el pánico por duplicado **sigue** disparándose
      cuando es el mismo módulo el que repite el nombre.
- [ ] Un test que afirma que `ModelName() == ""` es un error, no un prefijo vacío.
- [ ] Test de forma-consumidor: cliente y servidor construyen el nombre desde la
      misma fuente; ningún literal cualificado escrito a mano en el test.
- [ ] Ningún consumidor necesita declarar un tipo local para nombrar la
      operación.

## 6. Fuera de alcance

- No tocar `AddTool` ni la validación de `Access` (plan aparte, ya despachado).
- No renombrar las constantes `Op*` de las librerías de dominio: siguen siendo
  el nombre **dentro** del módulo; lo que cambia es cómo se publica.
- No introducir un mecanismo de alias ni de compatibilidad con los nombres
  viejos. Dos nombres para una operación es la puerta trasera que este plan
  cierra.
