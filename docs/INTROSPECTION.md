# Router Introspection (`/_routes`)

Every router implementation in the `webtyp` ecosystem shares a common introspection endpoint exposed via `router.MountIntrospection`.

## Mounting the Endpoint

```go
// Mount in development:
router.MountIntrospection(r, router.IntrospectionPath, policy).Public()

// Or in production with required permissions:
router.MountIntrospection(r, router.IntrospectionPath, policy).Requires(ResourceRoutes, model.Read)
```

`policy` is an optional `model.PolicyDescriber` (e.g. your authorization policy manager). If `policy` is `nil`, the response reports every route's required permission and sets `policy_known: false`.

## Response JSON Format

`GET /_routes` returns a JSON object containing an array of route descriptions:

```json
{
  "routes": [
    {
      "method": "GET",
      "path": "/api/sites/{id}/content",
      "resource": "site_content",
      "action": "r",
      "access": "guarded",
      "policy_known": true,
      "roles": ["admin", "editor"]
    }
  ]
}
```

### Response Fields

- **`method`**: HTTP verb or logical operation (`GET`, `POST`, `OP`, etc.).
- **`path`**: Route path pattern (e.g. `/api/sites/{id}/content`).
- **`resource`**: Target RBAC resource name (`""` if public or authenticated-only).
- **`action`**: Verb set required (`r`, `c`, `u`, `d`, or combinations like `ru`).
- **`access`**: Access level (`public`, `authenticated`, or `guarded`).
- **`policy_known`**: `true` if the server provided a `model.PolicyDescriber`; `false` if `policy == nil`.
- **`roles`**: Array of role codes granted this route's `(resource, action)`.
- **`args`**: Optional array describing accepted body fields (only present if declared via `Route.Accepts`).

### Interpreting `policy_known` and `roles`

- `policy_known: true` and `roles: ["admin"]`: Role `admin` has been granted access.
- `policy_known: true` and `roles: []`: **Warning!** The route requires a permission that **no role currently holds**. Every request to this route will respond with `403 Forbidden`.
- `policy_known: false` and `roles: []`: Policy details were not provided to the introspection endpoint. Roles holding the permission are unknown.
