package router

import (
	"webtyp.com/fmt"
	"webtyp.com/model"
)

// Operation identifies one operation: the module that serves it, the entity it acts on and exactly one of the four actions.
// It is a comparable value: transports key on it and nobody parses a name.
//
//	op := router.Operation{Module: "patient_directory", Entity: "patient", Action: model.Create}
type Operation struct {
	Module string       // the module's ModelName(), e.g. "patient_directory"
	Entity string       // the entity's ModelName(), e.g. "patient"
	Action model.Action // exactly one of model.Create, model.Read, model.Update, model.Delete
}

const (
	opSep     = "."
	opUnknown = "?"

	errBadOperation = "router: an operation is written <module>.<entity>.<create|read|update|delete>, got: "
)

// ParseOperation is the inverse of String, for a stored operation (an offline queue). It refuses anything that is not exactly
// <module>.<entity>.<create|read|update|delete> with non-empty parts.
//
//	op, err := router.ParseOperation("patient_directory.patient.create")
func ParseOperation(s string) (Operation, error) {
	parts := fmt.Split(s, opSep)
	if len(parts) != 3 {
		return Operation{}, fmt.Err(errBadOperation, s)
	}
	action, known := model.ParseVerb(parts[2])
	if !known {
		return Operation{}, fmt.Err(errBadOperation, s)
	}
	op := Operation{Module: parts[0], Entity: parts[1], Action: action}
	if !op.Valid() {
		return Operation{}, fmt.Err(errBadOperation, s)
	}
	return op, nil
}

// Valid reports whether Module and Entity are set and Action is exactly one of the four actions (the word comes from model.Action.Verb).
func (o Operation) Valid() bool { return o.Module != "" && o.Entity != "" && o.Action.Verb() != "" }

// String renders "<module>.<entity>.<verb>": for logs and error messages and for the ONE place that must persist an operation
// (an offline queue stores it as text and reads it back with ParseOperation). Everything else holds an Operation.
func (o Operation) String() string {
	verb := o.Action.Verb()
	if verb == "" {
		verb = opUnknown
	}
	return o.Module + opSep + o.Entity + opSep + verb
}
