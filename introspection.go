package router

import (
	"webtyp.com/model"
)

const (
	keyRoutes      = "routes"
	keyMethod      = "method"
	keyPath        = "path"
	keyResource    = "resource"
	keyAction      = "action"
	keyAccess      = "access"
	keyDescription = "description"
	keyMFA         = "mfa"
	keyPolicyKnown = "policy_known"
	keyRoles       = "roles"
	keyArgs        = "args"
	keyName        = "name"
	keyKind        = "kind"
	keyRequired    = "required"
)

// IntrospectionPath is where this ecosystem serves its route table.
const IntrospectionPath = "/_routes"

// ResourceIntrospection is the RBAC resource that guards the route table at
// IntrospectionPath and every screen that reads it (the API explorer). Mount it as
//
//	router.MountIntrospection(r, router.IntrospectionPath, policy).Requires(router.ResourceIntrospection, model.Read)
//
// The value is "api_explorer" because stored permission rows already use it: changing
// it would silently revoke every grant that names it.
const ResourceIntrospection model.Resource = "api_explorer"

// MountIntrospection registers a read-only endpoint at path reporting every
// route registered on r, plus what the policy says about each one.
//
// It returns the Route so the CALLER declares the access, which is never
// optional: a route that annotates nothing is AccessGuarded and refuses to
// start. Callers write either
//
// In production pass .Requires(resource, model.Read):
// the route map is reconnaissance for an attacker (OWASP API9). .Public()
// is acceptable only in a development-only registration.
//
// in development, or .Requires(resource, action) in a deployment where the
// permission map is not something to hand out anonymously.
//
// policy may be nil. The response then reports every route's required
// permission and marks the roles UNKNOWN — never as "nobody", which is a
// different and much more alarming fact (see model.RolesFor).
//
// The route table is read when a request arrives, not at mount time, so this
// may be called before or after the routes it reports.
func MountIntrospection(r Router, path string, policy model.PolicyDescriber) Route {
	return r.Get(path, func(ctx Context) {
		t := Table(r.Routes(), policy)
		ctx.WriteStatus(200)
		if err := ctx.Encode(&t); err != nil {
			ctx.WriteStatus(500)
			ctx.Write([]byte("encoding routes failed"))
		}
	})
}

// Table is the route table a client reads from IntrospectionPath, built in
// process: what MountIntrospection serves, and what a page with no server
// (a module's demo) shows from its own operations. A nil policy leaves every
// route's roles unknown (PolicyKnown false), which is not "nobody holds it".
//
// Example: t := router.Table(r.Routes(), policy)
func Table(routes []RouteInfo, policy model.PolicyDescriber) RouteTable {
	t := RouteTable{Routes: make([]RouteRecord, 0, len(routes))}
	for _, info := range routes {
		rec := RouteRecord{
			Method:      info.Method,
			Path:        info.Path,
			Resource:    string(info.Resource),
			Action:      info.Action.String(),
			Access:      info.Access.String(),
			MFA:         info.MFA,
			Description: info.Description,
			PolicyKnown: policy != nil,
		}
		if policy != nil {
			for _, role := range model.RolesFor(policy, info.Resource, info.Action) {
				rec.Roles = append(rec.Roles, string(role))
			}
		}
		if info.Args != nil {
			rec.HasArgs = true
			fields := info.Args.Schema()
			rec.Args = make([]ArgRecord, 0, len(fields))
			for i := range fields {
				kind := ""
				if fields[i].Type != nil {
					kind = fields[i].Type.Name()
				}
				rec.Args = append(rec.Args, ArgRecord{Name: fields[i].Name, Kind: kind, Required: fields[i].NotNull})
			}
		}
		t.Routes = append(t.Routes, rec)
	}
	return t
}

