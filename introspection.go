package router

import (
	"webtyp.com/model"
)

// IntrospectionPath is where this ecosystem serves its route table.
const IntrospectionPath = "/_routes"

// MountIntrospection registers a read-only endpoint at path reporting every
// route registered on r, plus what the policy says about each one.
//
// It returns the Route so the CALLER declares the access, which is never
// optional: a route that annotates nothing is AccessGuarded and refuses to
// start. Callers write either
//
//	router.MountIntrospection(r, router.IntrospectionPath, policy).Public()
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
		routes := r.Routes()
		views := make([]routeView, 0, len(routes))
		for _, info := range routes {
			v := routeView{
				info:        info,
				policyKnown: policy != nil,
			}
			if policy != nil {
				v.roles = model.RolesFor(policy, info.Resource, info.Action)
			}
			views = append(views, v)
		}
		resp := routesResponse{views: views}
		ctx.WriteStatus(200)
		if err := ctx.Encode(resp); err != nil {
			ctx.WriteStatus(500)
			ctx.Write([]byte("encoding routes failed"))
		}
	})
}

type routesResponse struct {
	views []routeView
}

func (r routesResponse) IsNil() bool { return false }

func (r routesResponse) EncodeFields(w model.FieldWriter) {
	arr := w.Array("routes", len(r.views))
	for _, v := range r.views {
		arr.Object(v)
	}
	arr.Close()
}

type routeView struct {
	info        RouteInfo
	roles       []model.RoleCode
	policyKnown bool
}

func (v routeView) IsNil() bool { return false }

func (v routeView) EncodeFields(w model.FieldWriter) {
	v.info.EncodeFields(w) // method, path, resource, action, access
	w.Bool("policy_known", v.policyKnown)
	arr := w.Array("roles", len(v.roles))
	for _, r := range v.roles {
		arr.String(string(r))
	}
	arr.Close()

	// args: the field names and kinds Route.Accepts declared; omitted entirely
	// when Args is nil ("no args", per Route.Accepts). An empty array would
	// claim the route takes an empty body, which is a different statement.
	if v.info.Args != nil {
		fields := v.info.Args.Schema()
		fa := w.Array("args", len(fields))
		for i := range fields {
			fa.Object(argField{f: &fields[i]})
		}
		fa.Close()
	}
}

type argField struct {
	f *model.Field
}

func (a argField) IsNil() bool { return a.f == nil }

func (a argField) EncodeFields(w model.FieldWriter) {
	w.String("name", a.f.Name)
	kindName := ""
	if a.f.Type != nil {
		kindName = a.f.Type.Name()
	}
	w.String("kind", kindName)
	w.Bool("required", a.f.NotNull)
}
