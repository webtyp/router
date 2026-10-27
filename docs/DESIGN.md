# Router Design Decisions

## Native Route Introspection vs OpenAPI Specification

### Context

This document explains why the `webtyp` ecosystem uses a lightweight native route introspection mechanism (`router.MountIntrospection` serving `/_routes`, consumed directly by `apiexplorer`) rather than adopting the OpenAPI 3.x specification as its primary internal model.

You will encounter this when:
- Exploring or debugging registered API routes in local development.
- Deciding whether to annotate handlers with OpenAPI/Swagger comments or schemas.
- Using or extending the `apiexplorer` developer tool in the webtyp platform shell.

Without this document, developers might wonder why `webtyp` does not require Swagger YAML definitions or `// @Summary` annotations, and whether OpenAPI is supported.

---

### Prior Art & Industry Landscape

Modern web frameworks expose API route tables and developer tools in three major ways:

1. **OpenAPI / Swagger as First-Class Schema (FastAPI, NestJS, Spring Boot):**
   - The framework maintains a full OpenAPI 3.x specification model.
   - Generates `/openapi.json` and renders Swagger UI at `/docs`.
   - Strengths: Industry standard, wide ecosystem of external SDK generators (TypeScript, Python, Swift) and API testing tools (Postman, Insomnia).
   - Weaknesses: High specification complexity (JSON Schema 2020-12, polymorphism, recursive references), heavy runtime dependencies, and friction when maintaining schema annotations alongside compiled code.

2. **CLI & Native Metadata (Ruby on Rails, Phoenix, Laravel):**
   - Route tables are inspected via CLI (`bin/rails routes`, `mix phx.routes`, `php artisan route:list`).
   - Development servers render route maps upon 404 or inside framework developer dashboards (e.g. Phoenix LiveDashboard).
   - Strengths: Zero overhead, tight integration with framework internals.
   - Weaknesses: Requires terminal access; no interactive in-browser HTTP client built-in out of the box.

3. **In-Browser DevTools (Nuxt 3 DevTools, Astro):**
   - Development servers expose lightweight metadata endpoints consumed by an embedded developer overlay.
   - Strengths: Zero-configuration visual interface for developers, fast, tailored to the frontend framework.

---

### Trade-offs: Native `/_routes` vs OpenAPI 3.x

| Dimension | OpenAPI 3.x Specification | WebTyp Native (`/_routes` + `apiexplorer`) |
| :--- | :--- | :--- |
| **Source of Truth** | OpenAPI JSON/YAML documents or code annotations (`// @Param`). | Go source code and runtime registrations (`config/routes.go`). |
| **WebAssembly / TinyGo Footprint** | Heavy. OpenAPI parsers and JSON Schema validators add hundreds of kilobytes to WASM binaries. | Zero external dependencies. Uses `webtyp/json` and compiles directly into TinyGo/WASM with minimal binary size. |
| **RBAC & Security Integration** | Generic decorators (`securitySchemes: BearerAuth`). No first-class awareness of active session roles or permissions. | Deep native integration with `webtyp/model` and RBAC. Detects orphan routes (routes with permissions no role holds) and reflects active user capabilities in real time. |
| **Developer Experience (Novice)** | Requires learning OpenAPI keywords or maintaining docstrings that drift from Go types. | **Zero configuration.** Start `webtyp` and the interactive API Explorer is available in the platform shell. |
| **External Interoperability** | Universal (Postman, client code generators). | Specific to WebTyp, unless translated by an exporter. |

---

### Architectural Decision

1. **Internal Core & WebAssembly UI:**
   - WebTyp maintains its lightweight, typed `router.RouteTable` shape.
   - The server mounts it guarded by `router.ResourceIntrospection` (read) in every environment.
   - The `apiexplorer` component in `webtyp.com/layout/apiexplorer` consumes `/_routes` directly, providing an instant, interactive test harness inside the management panel without manual configuration.

2. **External Interoperability (Opt-in Exporter):**
   - When external clients (e.g. third-party API consumers, mobile apps, or Postman collections) require OpenAPI 3.x specifications, a standalone adapter function/tool translates the native `router.RouteTable` into an OpenAPI-compliant document.
   - This keeps the core router and WebAssembly client lean, fast, and free of heavy dependencies while retaining full compatibility with the broader API tooling ecosystem.
