package router

import (
	"webtyp.com/fmt"
	"webtyp.com/model"
)

const (
	queryObjectSep = "."
	hexDigits      = "0123456789ABCDEF"
	errQueryArray  = "router: a query string cannot carry an array; send the arguments in a request body"
)

// QueryOf returns the query of a request path ("" when it has none), without the "?" and without any fragment.
//
//	args.DecodeFields(router.QueryReader(router.QueryOf(ctx.Path())))
func QueryOf(path string) string {
	if idx := fmt.Index(path, string(queryFragment)); idx != -1 {
		path = path[:idx]
	}
	idx := fmt.Index(path, string(queryStart))
	if idx == -1 {
		return ""
	}
	return path[idx+1:]
}

// QueryEscape percent-encodes s for a query: every byte except A-Z a-z 0-9 - _ . ~ becomes %XX.
func QueryEscape(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.' || c == '~' {
			out = append(out, c)
			continue
		}
		out = append(out, queryEscape, hexDigits[c>>4], hexDigits[c&15])
	}
	return string(out)
}

// QueryReader reads the fields of a record from a query ("a=b&c=d", without the "?"). A present key with no value is present and empty.
// An object field "filter" is read from the keys "filter.<field>". Arrays are not supported in a query: Array returns false.
func QueryReader(query string) model.FieldReader { return &queryReader{query: query} }

type queryReader struct {
	query  string
	prefix string // "filter." while reading the object "filter"
}

func (q *queryReader) String(name string) (string, bool) { return QueryParam("?"+q.query, q.prefix+name) }
func (q *queryReader) Raw(name string) (string, bool)    { return q.String(name) }

func (q *queryReader) Int(name string) (int64, bool) {
	s, ok := q.String(name)
	if !ok {
		return 0, false
	}
	n, err := fmt.Convert(s).Int64()
	if err != nil {
		return 0, false
	}
	return n, true
}

func (q *queryReader) Float(name string) (float64, bool) {
	s, ok := q.String(name)
	if !ok {
		return 0, false
	}
	f, err := fmt.Convert(s).Float64()
	if err != nil {
		return 0, false
	}
	return f, true
}

func (q *queryReader) Bool(name string) (bool, bool) {
	s, ok := q.String(name)
	if !ok {
		return false, false
	}
	switch fmt.ToLower(s) {
	case "true", "on", "1":
		return true, true
	case "false", "off", "0", "":
		return false, true
	}
	return false, false
}

func (q *queryReader) Bytes(name string) ([]byte, bool) {
	s, ok := q.String(name)
	return []byte(s), ok
}

func (q *queryReader) Object(name string, into model.Decodable) bool {
	prefix := q.prefix + name + queryObjectSep
	found := false
	for _, part := range fmt.Split(q.query, string(querySep)) {
		if len(part) > len(prefix) && QueryUnescape(part[:len(prefix)]) == prefix {
			found = true
			break
		}
	}
	if !found {
		return false
	}
	into.DecodeFields(&queryReader{query: q.query, prefix: prefix})
	return true
}

func (q *queryReader) Array(name string) (model.ArrayReader, bool) { return nil, false }

const methodGet = "GET"

// DecodeArgs reads the arguments of a request into into. The method decides where they travel: a GET carries them in its query
// and never in a body (a body on a GET is ignored); every other method carries them in its body, decoded by fromBody. It is the
// one place that policy lives; httpd and the edge both call it.
//
//	err := router.DecodeArgs(ctx.Method(), router.QueryOf(ctx.Path()), ctx.Body(), &args, decodeJSON)
func DecodeArgs(method, query string, body []byte, into model.Decodable, fromBody func(body []byte, into model.Decodable) error) error {
	if method == methodGet {
		into.DecodeFields(QueryReader(query))
		return nil
	}
	return fromBody(body, into)
}

// EncodeQuery writes the fields of v as a query ("a=b&c=d"), the counterpart of QueryReader. An object field is written as
// "name.<field>"; an array is an error (send a body instead). A Null field is left out.
//
//	query, err := router.EncodeQuery(&crud.ReadArgs{Search: "12.345"})
func EncodeQuery(v model.Encodable) (string, error) {
	w := &queryWriter{}
	v.EncodeFields(w)
	if w.failed {
		return "", fmt.Err(errQueryArray)
	}
	return string(w.out), nil
}

type queryWriter struct {
	out    []byte
	prefix string
	failed bool
}

func (w *queryWriter) put(name, val string) {
	if len(w.out) > 0 {
		w.out = append(w.out, querySep)
	}
	w.out = append(w.out, QueryEscape(w.prefix+name)...)
	w.out = append(w.out, queryEq)
	w.out = append(w.out, QueryEscape(val)...)
}

func (w *queryWriter) String(name, val string)        { w.put(name, val) }
func (w *queryWriter) Raw(name, val string)           { w.put(name, val) }
func (w *queryWriter) Int(name string, val int64)     { w.put(name, fmt.Convert(val).String()) }
func (w *queryWriter) Float(name string, val float64) { w.put(name, fmt.Convert(val).String()) }
func (w *queryWriter) Bool(name string, val bool) {
	if val {
		w.put(name, "true")
		return
	}
	w.put(name, "false")
}
func (w *queryWriter) Bytes(name string, val []byte) { w.put(name, string(val)) }
func (w *queryWriter) Null(name string)              {}
func (w *queryWriter) Object(name string, val model.Encodable) {
	inner := &queryWriter{out: w.out, prefix: w.prefix + name + queryObjectSep}
	val.EncodeFields(inner)
	w.out, w.failed = inner.out, w.failed || inner.failed
}
func (w *queryWriter) Array(name string, n int) model.ArrayWriter {
	w.failed = true
	return discardArray{}
}

// discardArray swallows the elements of an array the query refused.
type discardArray struct{}

func (discardArray) String(string)          {}
func (discardArray) Int(int64)              {}
func (discardArray) Float(float64)          {}
func (discardArray) Bool(bool)              {}
func (discardArray) Bytes([]byte)           {}
func (discardArray) Object(model.Encodable) {}
func (discardArray) Close()                 {}
