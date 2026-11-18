package router

import "webtyp.com/model"

// statusOfKind is the one table between error kinds and HTTP statuses: router.WriteError writes the status of a kind,
// rpc's client and the loopback read the kind back, so the two directions cannot drift apart. A slice, not a map:
// this code runs in the browser too.
var statusOfKind = []struct {
	kind   model.ErrorKind
	status int
}{
	{model.ErrorInvalid, 400},
	{model.ErrorNotFound, 404},
	{model.ErrorConflict, 409},
	{model.ErrorUnreachable, 503},
}

// unreachableStatuses also read back as unreachable: no HTTP answer at all (0), a bad gateway and a timeout.
var unreachableStatuses = []int{0, 502, 504}

// statusInternal is what a kind outside the table (internal) travels as.
const statusInternal = 500

// StatusOf is the HTTP status an error kind travels as.
//
//	ctx.WriteStatus(router.StatusOf(model.KindOf(err)))
func StatusOf(k model.ErrorKind) int {
	for _, e := range statusOfKind {
		if e.kind == k {
			return e.status
		}
	}
	return statusInternal
}

// KindOfStatus is the error kind an HTTP status carries back: the table above, unreachable for no answer, 502 and 504,
// and internal for any other status.
func KindOfStatus(status int) model.ErrorKind {
	for _, e := range statusOfKind {
		if e.status == status {
			return e.kind
		}
	}
	for _, u := range unreachableStatuses {
		if u == status {
			return model.ErrorUnreachable
		}
	}
	return model.ErrorInternal
}

// Problem is the body of an operation's error (RFC 9457 problem details):
// router.WriteError writes it; every caller reads it back.
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
