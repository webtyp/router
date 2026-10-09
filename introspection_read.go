package router

import "webtyp.com/model"

// RouteRecord is one entry of the /_routes table as a client reads it back.
type RouteRecord struct {
	Method      string
	Path        string
	Resource    string
	Action      string // CRUD letters as served: "r", "ru", "crud"; "" when none
	Access      string // "public" | "authenticated" | "guarded"
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
