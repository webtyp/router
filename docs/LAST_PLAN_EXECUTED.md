---
PLAN: "feat(router): Streamer.Done — the stream handler learns the client left"
TAG: v0.2.0
EXECUTOR: jules
REVIEWER: none
---

> Este plan se despacha con el flujo CodeJob. Ver skill: agents-workflow.
> Orden de la ola (tres repos): **1. `webtyp/router` (este)** → 2. `webtyp/server` (`httpd` lo implementa)
> y 3. `webtyp/sse` (lo usa). Los planes 2 y 3 esperan el tag de este.

# Plan — `router.Streamer` avisa cuando el cliente se fue

## 0. El problema (verificado)

`router.Streamer` (`router.go:75`) es `Context` + `Flush()`. No hay forma de saber que el cliente cerró la
conexión. Consecuencia real en `webtyp/sse` (`server.go`, `streamHandler`): el bucle

```go
for msg := range client.send {
	if _, err := st.Write(msg); err != nil { return }
	st.Flush()
}
```

solo termina cuando **una escritura falla**, y eso solo ocurre cuando llega un mensaje a ese canal. Con un
canal por usuario (el buzón del chat de `mjosefa-cms`, `chat_room.inbox.<userID>`), que recibe pocos
mensajes, cada pestaña cerrada deja una gorutina y un registro en el hub vivos hasta el próximo aviso a ese
usuario (o para siempre). Además `httptest.Server.Close()` se bloquea en cualquier test que abra un stream
autenticado: así se detectó, al integrar el chat en `mjosefa-cms` (el ejecutor quedó en bucle).

## Design gate

1. **Prior art.** Go `net/http`: `Request.Context().Done()`; ASP.NET Core: `HttpContext.RequestAborted`
   (un `CancellationToken`); Node: `req.on('close')`; Spring: `SseEmitter.onCompletion`. Todos exponen
   una señal de "el cliente se fue" en el objeto de la petición. Aquí el objeto es `Streamer`, y el idioma
   Go de esa señal es un canal `Done()`, igual que `context.Context`.
2. **Nombre.** `Done() <-chan struct{}` — cualquier dev de Go lo lee sin documentación: "se cierra cuando
   la conexión terminó".
3. **Libro de complejidad.** Conceptos +1 (un método) · archivos que toca quien escribe un stream: 0 ·
   formas de hacer lo mismo: hoy 0 (no existe) → 1.
4. **Dónde vive.** En `router.Streamer`, no en `router.Context`: solo un stream dura más que una petición.
   `httpd` lo implementa con `r.Context().Done()`; `sse` lo consume. Ningún consumidor declara una interfaz
   local para esto.
5. **Qué borra.** Nada en este repo; en `sse` desaparece el comportamiento "solo sale cuando falla una
   escritura".

Es un cambio incompatible para quien implemente `Streamer` fuera de este repo: implementaciones conocidas
= `server/httpd.httpStreamer` (plan 2) y dos dobles de test (`router/tests/router_test.go:fakeStreamer`,
`sse/tests/test_router_test.go:mockStreamer`). Por eso `TAG: v0.2.0`.

## Etapas

1. `router.go`: agregar a `Streamer`
   ```go
   // Done is closed when the client disconnects or the server shuts the
   // connection down. A handler that loops (a push stream) must select on it
   // and return, or it outlives the connection.
   Done() <-chan struct{}
   ```
2. `tests/router_test.go`: `fakeStreamer` implementa `Done()` devolviendo un canal que el test controla.
   Test nuevo `TestStreamer_DoneIsPartOfTheContract`: `var _ router.Streamer = (*fakeStreamer)(nil)` y un
   caso que cierra el canal y comprueba que un handler de ejemplo que hace `select` sobre `Done()` retorna.
3. Docs: `README.md`/`docs/` donde se describa `Streamer` → una línea sobre `Done()`.

## Criterios de aceptación

```bash
gotest                                  # verde
grep -n "Done() <-chan struct{}" router.go   # → 1
```
