# Architecture

## The Local Door and the Internet Door

The application serves on two ports: a local one and an internet one (through Cloudflare Tunnel).
The door is identified by the listener (the port the transport listened on), and not the IP address.
Through the tunnel every request arrives from `127.0.0.1`, so the client IP cannot tell the doors
apart, and an `X-Forwarded-For` header can be forged. Only the transport knows the listener port,
and it writes `ContextKeyNetwork` into the request to declare it.

An unmarked request is always considered to be from the internet (strict mode). If a transport forgets
to say where the request came from, the strict rules will apply, ensuring fail-safe behavior.

## Introspection Route Table Reading

Tools and modules in the ecosystem can read back what `MountIntrospection` writes using the `router.RouteTable` shape:

```go
import "webtyp.com/router"
import "webtyp.com/json"

// fetch /_routes somehow
var table router.RouteTable
if err := json.Decode(body, &table); err == nil {
    for _, route := range table.Routes {
        if route.Orphan() {
            println("Guarded route with NO role authorized: ", route.Path)
        }
    }
}
```

This ensures consumers don't have to redefine the read shape and stays perfectly in sync with the `MountIntrospection` endpoint.

For the architectural justification and trade-off comparison between WebTyp native route introspection and the OpenAPI specification, see [DESIGN.md](DESIGN.md).


## layoutscan

For documentation regarding the `layoutscan` package rules and layout invariants, please refer to the README's `layoutscan` section.
