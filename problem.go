package router

import "webtyp.com/model"

// The statuses an error kind travels as over HTTP. Written once, read by every
// transport: rpc.WriteError writes them, rpc's client and the loopback read them
// back, so the directions cannot drift apart.
const (
	statusInvalid  = 400
	statusNotFound = 404
	statusConflict = 409
	statusInternal = 500
)

// StatusOf is the HTTP status an error kind travels as.
//
//	ctx.WriteStatus(router.StatusOf(model.KindOf(err)))
func StatusOf(k model.ErrorKind) int {
	switch k {
	case model.ErrorInvalid:
		return statusInvalid
	case model.ErrorNotFound:
		return statusNotFound
	case model.ErrorConflict:
		return statusConflict
	}
	return statusInternal
}

// KindOfStatus is the error kind an HTTP status carries back; any other status,
// or no answer at all (0), is internal.
func KindOfStatus(status int) model.ErrorKind {
	switch status {
	case statusInvalid:
		return model.ErrorInvalid
	case statusNotFound:
		return model.ErrorNotFound
	case statusConflict:
		return model.ErrorConflict
	}
	return model.ErrorInternal
}

// Problem is the body of an operation's error (RFC 9457 problem details):
// rpc.WriteError writes it; every caller reads it back.
type Problem struct {
	Status int64
	Title  string
	Detail string
	Field  string // the field that failed, "" when none
}

func (p *Problem) IsNil() bool { return p == nil }

func (p *Problem) EncodeFields(w model.FieldWriter) {
	w.Int("status", p.Status)
	w.String("title", p.Title)
	if p.Detail != "" {
		w.String("detail", p.Detail)
	}
	if p.Field != "" {
		w.String("field", p.Field)
	}
}

func (p *Problem) DecodeFields(r model.FieldReader) {
	if v, ok := r.Int("status"); ok {
		p.Status = v
	}
	if v, ok := r.String("title"); ok {
		p.Title = v
	}
	if v, ok := r.String("detail"); ok {
		p.Detail = v
	}
	if v, ok := r.String("field"); ok {
		p.Field = v
	}
}
