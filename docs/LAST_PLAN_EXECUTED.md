---
PLAN: "feat(loopback): in-process router.Caller over an OperationRegistry"
TAG: v0.2.0
EXECUTOR: local
REVIEWER: none
---

# PLAN — `router` Caller in-proc (Etapa B del `DEMO_AGENDA_MASTER_PLAN`)

Orquestador: [`webtyp/docs/DEMO_AGENDA_MASTER_PLAN.md`](https://github.com/webtyp/app/blob/main/docs/DEMO_AGENDA_MASTER_PLAN.md) §4.1, §7 fila B.
(Copia local: `/home/cesar/Dev/Project/webtyp/docs/DEMO_AGENDA_MASTER_PLAN.md`.)

## Problema

No existe un `router.Caller` que despache **en el mismo proceso** contra las ops
que uno o más `router.OperationModule` registraron. Hoy el único `Caller` real
es `mcp.NewCaller` (red + SSE). La demo (`app-demo`) quiere importar los módulos
de dominio reales (`item_catalog`, `appointment_booking`, `work_schedule`) y
manejarlos sin levantar un servidor. Un `Caller` in-proc es además útil para
tests de integración de consumidores y para apps offline.

## Entrega

Un paquete nuevo **`webtyp.com/router/loopback`** (sub-paquete de `router`; si al
implementarlo resulta que necesita tipos no exportados de `router` que no
conviene exportar, mover a `webtyp.com/routerloop` y actualizar este encabezado).

```go
package loopback

// New construye un router.Caller que invoca, en proceso y de forma síncrona,
// las operaciones que los módulos dados registraron vía MountOperations.
// Codifica args/resultado con webtyp/json (WASM-safe, ya en el árbol del
// ecosistema) — el consumidor sigue trabajando en valores model tipados y
// nunca importa un codec.
func New(mods ...router.OperationModule) router.Caller
```

### Piezas internas (no exportadas)

1. **`registry`** — implementa `router.OperationRegistry` (una sola función:
   `Operation(name string, h HandlerFunc) Route`). Guarda `name → HandlerFunc`
   (ver nota WASM abajo sobre `map` vs slice) y **devuelve un `noopRoute`**: un
   stub de `router.Route` cuyos métodos encadenables (`.Requires(...)`,
   `.Accepts(...)`, `.Public()`, `.Authenticated()`, …) devuelven `self` y no
   hacen nada. Es obligatorio: los módulos reales encadenan
   `reg.Operation(...).Requires(...).Accepts(...)` y eso debe compilar y no
   panicar. Revisar la interfaz `router.Route` real y stubbear TODOS sus métodos.
2. **`inCtx`** — implementa `router.Context` (~18 métodos; casi todos triviales):
   - `Decode(v model.Decodable)` — deserializa en `v` el buffer JSON de los args
     (producido en `Call` con `json.Encode(args, &buf)`). Usar `webtyp/json` en
     ambas direcciones — así se ejercita el mismo camino de codec que producción
     y no se asume que `args` e `into` sean el mismo tipo Go.
   - `Encode(v model.Encodable)` — `json.Encode(v, &c.body)`; `Call` luego
     `json.Decode(c.body, into)`.
   - `WriteStatus(code)` / `Write(b) (int, error)` — guardan `c.status` / `c.body`;
     `Call` mapea `code >= 400` a un `error` (`fmt.Err`) con el cuerpo como
     mensaje, igual que el `writeError` de `appointment_booking`.
   - `Body() []byte` → los args crudos. `Method()`→`""`, `Path()`→`""`,
     `Param(name)`→`""`, `GetHeader`→`""`, `SetHeader`→no-op,
     `Cookie`→`(Cookie{}, false)`, `SetCookie`→no-op.
   - `SetValue(k,v)`/`Value(k)` y `SetUserID`/`UserID` — un `[]fmt.KeyValue`
     pequeño (no `map`), o dos strings si con uno basta. NO `panic` en ninguno.
3. **`caller`** — implementa `router.Caller`:
   - `Call(op, args, into, done)` — busca `op` en el `registry`; si no existe,
     `done(fmt.Err("loopback", "unknown", "op", op))`. Construye un `inCtx` con
     `args`, ejecuta el handler síncronamente, y según el estado: `done(nil)` +
     decodifica el cuerpo en `into` (si `into != nil`), o `done(err)`.
   - `Dispatch(op, args)` — igual pero sin cuerpo ni error (fire-and-forget);
     un handler que falla se registra con `dom.Log`/equivalente, no propaga.
   - Asíncrono en la firma (el contrato lo pide) pero la ejecución es síncrona:
     invocar `done` antes de retornar está permitido (el doc de `Caller` lo
     contempla: "works for wasm fetch and for in-process test doubles alike").

### Nota WASM

Este `Caller` se compila dentro del binario wasm de `app-demo`, así que aplica
la regla "cero `map` en WASM". El `registry` guarda `name → HandlerFunc` como
**`[]struct{ name string; h router.HandlerFunc }`** con scan lineal en `Call`
(decenas de entradas, poblado una vez al construir — el scan no cuesta nada
medible). No usar `map`. No introducir `sync` salvo que un test lo exija: el uso
es single-goroutine en el cliente wasm.

## Tests (`loopback_test.go`, backend, `testing` stdlib)

- `TestCall_RoundTrips` — un `OperationModule` de juguete con una op `echo` que
  copia args→resultado; `New(toy).Call("echo", &In{X:"hi"}, &Out{}, done)` deja
  `Out.X == "hi"` y `done(nil)`.
- `TestCall_UnknownOp` — `Call("nope", …)` → `done` con error no-nil, `into`
  intacto.
- `TestCall_HandlerStatus4xx` — op que hace `ctx.WriteStatus(404)` → `done` con
  error; el mensaje contiene el cuerpo escrito.
- `TestCall_NilInto` — op de solo-efecto (create/delete) con `into == nil` →
  `done(nil)` sin panic.
- `TestDispatch_FireAndForget` — `Dispatch` ejecuta el handler; un handler que
  hace `WriteStatus(500)` no rompe nada.
- `TestMultiModule` — `New(a, b)`; ops de ambos módulos resuelven.

## Criterios de aceptación

- `gotest ./...` verde en `router`.
- `GOOS=js GOARCH=wasm go build ./...` OK.
- `router/docs/` gana un `LOOPBACK.md` (o una sección en el doc que corresponda —
  hoy `docs/` tiene `INTROSPECTION.md` + `LAST_PLAN_EXECUTED.md`, no
  `ARCHITECTURE.md`) que describe `loopback` como el `Caller` in-proc de
  referencia, en contraste con `mcp.NewCaller`.
- README de `router` indexa el sub-paquete.
- Si `router/loopback` resulta que necesita símbolos no exportados de `router`
  que no conviene exportar, mover a `webtyp.com/routerloop` (paquete propio) y
  actualizar el encabezado de este plan y el master §4.1 / §7 fila B.

## Fuera de alcance

- RBAC / `.Requires(...)`: el `loopback` confía en el llamador (mismo proceso).
  Si más adelante se quiere aplicar política en proceso, es un añadido separado.
- Streaming / sockets: solo `Call` + `Dispatch`.
