# Architecture

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
