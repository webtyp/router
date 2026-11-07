package router

import "webtyp.com/model"

const (
	headerContentType  = "Content-Type"
	contentTypeProblem = "application/problem+json"
	msgInternal        = "internal error"
	ctxKeyInternal     = "router.internal"
)

// WriteError answers an operation's error. The status comes from model.KindOf(err):
// invalid 400, not found 404, conflict 409, anything unclassified 500. The body is an
// RFC 9457 problem (Content-Type application/problem+json):
//
//	{"status":400,"title":"invalid","detail":"name required","field":"name"}
//
// A 500 never carries err's text: its detail is "internal error" and the real error is
// kept for the transport's logger (InternalError).
//
//	if err != nil { router.WriteError(ctx, err); return }
func WriteError(ctx Context, err error) {
	kind := model.KindOf(err)
	status := StatusOf(kind)
	detail := err.Error()
	if kind == model.ErrorInternal {
		detail = msgInternal
		ctx.SetValue(ctxKeyInternal, err.Error())
	}
	p := Problem{Status: int64(status), Title: kind.String(), Detail: detail, Field: model.FieldOf(err)}
	ctx.SetHeader(headerContentType, contentTypeProblem)
	ctx.WriteStatus(status)
	ctx.Encode(&p)
}

// InternalError is the real text of the 500 WriteError answered on ctx, "" when none.
// A transport logs it; the client never sees it.
//
//	if d := router.InternalError(ctx); d != "" { log("internal error:", d) }
func InternalError(ctx Context) string { return ctx.Value(ctxKeyInternal) }
