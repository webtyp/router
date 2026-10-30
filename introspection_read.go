package router

import "webtyp.com/model"

// RouteRecord is one entry of the /_routes table as a client reads it back.
type RouteRecord struct {
	Method      string
	Path        string
	Resource    string
	Action      string // CRUD letters as served: "r", "ru", "crud"; "" when none
	Access      string // "public" | "authenticated" | "guarded"
	MFA         bool   // the route requires multi-factor authentication
	Description string
	PolicyKnown bool
	Roles       []string
	Args        []ArgRecord
	HasArgs     bool // false when the entry had no "args" key — different from an empty list
}

func (r *RouteRecord) IsNil() bool { return false }

func (r *RouteRecord) DecodeFields(rd model.FieldReader) {
	if s, ok := rd.String(keyMethod); ok {
		r.Method = s
	}
	if s, ok := rd.String(keyPath); ok {
		r.Path = s
	}
	if s, ok := rd.String(keyResource); ok {
		r.Resource = s
	}
	if s, ok := rd.String(keyAction); ok {
		r.Action = s
	}
	if s, ok := rd.String(keyAccess); ok {
		r.Access = s
	}
	if b, ok := rd.Bool(keyMFA); ok {
		r.MFA = b
	}
	if s, ok := rd.String(keyDescription); ok {
		r.Description = s
	}
	if b, ok := rd.Bool(keyPolicyKnown); ok {
		r.PolicyKnown = b
	}
	if arr, ok := rd.Array(keyRoles); ok {
		r.Roles = make([]string, arr.Len())
		for i := 0; i < arr.Len(); i++ {
			r.Roles[i] = arr.String(i)
		}
	}
	if arr, ok := rd.Array(keyArgs); ok {
		r.HasArgs = true
		r.Args = make([]ArgRecord, arr.Len())
		for i := 0; i < arr.Len(); i++ {
			arr.Object(i, &r.Args[i])
		}
	}
}

// Orphan reports a guarded route whose permission NO role holds: it answers 403 to everyone.
// False when PolicyKnown is false: "the server did not say" is not "nobody has it".
func (r RouteRecord) Orphan() bool {
	return r.Access == model.AccessGuarded.String() && r.PolicyKnown && len(r.Roles) == 0
}

// ArgRecord is one field of a route's declared body schema.
type ArgRecord struct {
	Name     string
	Kind     string
	Required bool
}

func (a *ArgRecord) IsNil() bool { return false }

func (a *ArgRecord) DecodeFields(rd model.FieldReader) {
	if s, ok := rd.String(keyName); ok {
		a.Name = s
	}
	if s, ok := rd.String(keyKind); ok {
		a.Kind = s
	}
	if b, ok := rd.Bool(keyRequired); ok {
		a.Required = b
	}
}

// RouteTable is the decoded /_routes response.
type RouteTable struct {
	Routes []RouteRecord
}

func (t *RouteTable) IsNil() bool { return false }

func (t *RouteTable) DecodeFields(rd model.FieldReader) {
	if arr, ok := rd.Array(keyRoutes); ok {
		t.Routes = make([]RouteRecord, arr.Len())
		for i := 0; i < arr.Len(); i++ {
			arr.Object(i, &t.Routes[i])
		}
	}
}

func (t *RouteTable) EncodeFields(w model.FieldWriter) {
	arr := w.Array(keyRoutes, len(t.Routes))
	for i := range t.Routes {
		arr.Object(&t.Routes[i])
	}
	arr.Close()
}

func (r *RouteRecord) EncodeFields(w model.FieldWriter) {
	w.String(keyMethod, r.Method)
	w.String(keyPath, r.Path)
	w.String(keyResource, r.Resource)
	w.String(keyAction, r.Action)
	w.String(keyAccess, r.Access)
	if r.MFA {
		w.Bool(keyMFA, true)
	}
	if r.Description != "" {
		w.String(keyDescription, r.Description)
	}
	w.Bool(keyPolicyKnown, r.PolicyKnown)
	roles := w.Array(keyRoles, len(r.Roles))
	for _, s := range r.Roles {
		roles.String(s)
	}
	roles.Close()
	// args are omitted entirely when the route declared none: an empty list
	// would claim the route takes an empty body, which is a different statement.
	if r.HasArgs {
		args := w.Array(keyArgs, len(r.Args))
		for i := range r.Args {
			args.Object(&r.Args[i])
		}
		args.Close()
	}
}

func (a *ArgRecord) EncodeFields(w model.FieldWriter) {
	w.String(keyName, a.Name)
	w.String(keyKind, a.Kind)
	w.Bool(keyRequired, a.Required)
}
