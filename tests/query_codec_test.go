package router_test

import (
	"errors"
	"strings"
	"testing"

	"webtyp.com/model"
	"webtyp.com/router"
)

type qfilter struct {
	Name string
}

func (f *qfilter) EncodeFields(w model.FieldWriter) {
	if f.Name != "" {
		w.String("name", f.Name)
	}
}

func (f *qfilter) DecodeFields(r model.FieldReader) {
	if v, ok := r.String("name"); ok {
		f.Name = v
	}
}

func (f *qfilter) IsNil() bool { return f == nil }

type qrec struct {
	ID     string
	Search string
	Limit  int64
	On     bool
	Filter qfilter
	HasArr bool
}

func (r *qrec) EncodeFields(w model.FieldWriter) {
	if r.ID != "" {
		w.String("id", r.ID)
	}
	if r.Search != "" {
		w.String("search", r.Search)
	}
	if r.Limit != 0 {
		w.Int("limit", r.Limit)
	}
	if r.On {
		w.Bool("on", r.On)
	}
	if r.Filter.Name != "" {
		w.Object("filter", &r.Filter)
	}
	if r.HasArr {
		arr := w.Array("ids", 1)
		arr.String("item")
		arr.Close()
	}
}

func (r *qrec) DecodeFields(rd model.FieldReader) {
	if v, ok := rd.String("id"); ok {
		r.ID = v
	}
	if v, ok := rd.String("search"); ok {
		r.Search = v
	}
	if v, ok := rd.Int("limit"); ok {
		r.Limit = v
	}
	if v, ok := rd.Bool("on"); ok {
		r.On = v
	}
	rd.Object("filter", &r.Filter)
}

func (r *qrec) IsNil() bool { return r == nil }

func TestPathOf_StripsQueryAndFragment(t *testing.T) {
	if got := router.PathOf("/a/b?x=1#f"); got != "/a/b" {
		t.Fatalf("PathOf(/a/b?x=1#f) = %q, want %q", got, "/a/b")
	}
	if got := router.PathOf("/a/b"); got != "/a/b" {
		t.Fatalf("PathOf(/a/b) = %q, want %q", got, "/a/b")
	}
	if got := router.PathOf("/a?"); got != "/a" {
		t.Fatalf("PathOf(/a?) = %q, want %q", got, "/a")
	}
	if got := router.PathOf(""); got != "" {
		t.Fatalf("PathOf(\"\") = %q, want %q", got, "")
	}
}

func TestQueryOf(t *testing.T) {
	if got := router.QueryOf("/api/m/e?id=1&x=2#frag"); got != "id=1&x=2" {
		t.Fatalf("QueryOf(/api/m/e?id=1&x=2#frag) = %q, want %q", got, "id=1&x=2")
	}
	if got := router.QueryOf("/api/m/e"); got != "" {
		t.Fatalf("QueryOf(/api/m/e) = %q, want %q", got, "")
	}
	if got := router.QueryOf("/p?"); got != "" {
		t.Fatalf("QueryOf(/p?) = %q, want %q", got, "")
	}
}

func TestQuery_RoundTrip(t *testing.T) {
	in := &qrec{ID: "a b", Search: "12.345-6", Limit: 50}
	encoded, err := router.EncodeQuery(in)
	if err != nil {
		t.Fatalf("EncodeQuery error: %v", err)
	}
	want := "id=a%20b&search=12.345-6&limit=50"
	if encoded != want {
		t.Fatalf("EncodeQuery = %q, want %q", encoded, want)
	}

	var out qrec
	out.DecodeFields(router.QueryReader(encoded))
	if out.ID != in.ID || out.Search != in.Search || out.Limit != in.Limit {
		t.Fatalf("decoded %+v != in %+v", out, in)
	}
}

func TestQuery_ObjectUsesPrefix(t *testing.T) {
	in := &qrec{Filter: qfilter{Name: "x"}}
	encoded, err := router.EncodeQuery(in)
	if err != nil {
		t.Fatalf("EncodeQuery error: %v", err)
	}
	if !strings.Contains(encoded, "filter.name=x") {
		t.Fatalf("EncodeQuery = %q, want to contain %q", encoded, "filter.name=x")
	}

	var out qrec
	out.DecodeFields(router.QueryReader(encoded))
	if out.Filter.Name != "x" {
		t.Fatalf("decoded filter.Name = %q, want %q", out.Filter.Name, "x")
	}
}

func TestQuery_ObjectAbsentIsNotDecoded(t *testing.T) {
	reader := router.QueryReader("id=1&search=test")
	var f qfilter
	if reader.Object("filter", &f) {
		t.Fatalf("expected Object(filter) to return false")
	}
}

func TestQuery_PresentButEmptyIsPresent(t *testing.T) {
	reader := router.QueryReader("search=")
	val, ok := reader.String("search")
	if !ok || val != "" {
		t.Fatalf("String(search) = (%q, %v), want (\"\", true)", val, ok)
	}

	missingVal, missingOk := reader.String("other")
	if missingOk || missingVal != "" {
		t.Fatalf("String(other) = (%q, %v), want (\"\", false)", missingVal, missingOk)
	}
}

func TestQuery_BoolForms(t *testing.T) {
	trueCases := []string{"on=true", "on=1", "on=on", "on=TRUE"}
	for _, q := range trueCases {
		reader := router.QueryReader(q)
		v, ok := reader.Bool("on")
		if !ok || !v {
			t.Fatalf("Bool(on) for %q = (%v, %v), want (true, true)", q, v, ok)
		}
	}

	falseCases := []string{"on=false", "on=0", "on=off", "on="}
	for _, q := range falseCases {
		reader := router.QueryReader(q)
		v, ok := reader.Bool("on")
		if !ok || v {
			t.Fatalf("Bool(on) for %q = (%v, %v), want (false, true)", q, v, ok)
		}
	}

	reader := router.QueryReader("on=maybe")
	if _, ok := reader.Bool("on"); ok {
		t.Fatalf("Bool(on) for on=maybe expected ok=false, got true")
	}
}

func TestQuery_IntRejectsGarbage(t *testing.T) {
	reader := router.QueryReader("limit=abc")
	val, ok := reader.Int("limit")
	if ok || val != 0 {
		t.Fatalf("Int(limit) = (%v, %v), want (0, false)", val, ok)
	}
}

func TestQuery_ArrayIsRefusedOnEncode(t *testing.T) {
	in := &qrec{HasArr: true}
	_, err := router.EncodeQuery(in)
	if err == nil {
		t.Fatalf("EncodeQuery expected error for array, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "array") {
		t.Fatalf("EncodeQuery error %q does not contain array", err.Error())
	}
}

type targetArgs struct {
	Code string
	Qty  int64
}

func (a *targetArgs) EncodeFields(w model.FieldWriter) {
	if a.Code != "" {
		w.String("code", a.Code)
	}
	if a.Qty != 0 {
		w.Int("qty", a.Qty)
	}
}

func (a *targetArgs) DecodeFields(r model.FieldReader) {
	if v, ok := r.String("code"); ok {
		a.Code = v
	}
	if v, ok := r.Int("qty"); ok {
		a.Qty = v
	}
}

func (a *targetArgs) IsNil() bool { return a == nil }

func TestDecodeArgs_GetUsesTheQuery(t *testing.T) {
	var target targetArgs
	fromBodyCalled := false
	fromBody := func(body []byte, into model.Decodable) error {
		fromBodyCalled = true
		return nil
	}
	err := router.DecodeArgs("GET", "code=abc&qty=3", nil, &target, fromBody)
	if err != nil {
		t.Fatalf("DecodeArgs error: %v", err)
	}
	if fromBodyCalled {
		t.Fatalf("fromBody was called on GET")
	}
	if target.Code != "abc" || target.Qty != 3 {
		t.Fatalf("target = %+v, want Code:abc Qty:3", target)
	}
}

func TestDecodeArgs_GetIgnoresTheBody(t *testing.T) {
	var target targetArgs
	fromBodyCalled := false
	fromBody := func(body []byte, into model.Decodable) error {
		fromBodyCalled = true
		return nil
	}
	err := router.DecodeArgs("GET", "code=q", []byte("{\"code\":\"body\"}"), &target, fromBody)
	if err != nil {
		t.Fatalf("DecodeArgs error: %v", err)
	}
	if fromBodyCalled {
		t.Fatalf("fromBody was called on GET")
	}
	if target.Code != "q" {
		t.Fatalf("target.Code = %q, want %q", target.Code, "q")
	}
}

func TestDecodeArgs_OtherMethodsUseTheBody(t *testing.T) {
	for _, m := range []string{"POST", "PATCH", "DELETE"} {
		var target targetArgs
		called := 0
		fromBody := func(body []byte, into model.Decodable) error {
			called++
			into.DecodeFields(router.QueryReader("code=frombody"))
			return nil
		}
		err := router.DecodeArgs(m, "code=fromquery", []byte("raw"), &target, fromBody)
		if err != nil {
			t.Fatalf("DecodeArgs error for %s: %v", m, err)
		}
		if called != 1 {
			t.Fatalf("fromBody called %d times for %s, want 1", called, m)
		}
		if target.Code != "frombody" {
			t.Fatalf("target.Code = %q, want %q for %s", target.Code, "frombody", m)
		}
	}
}

func TestDecodeArgs_BodyErrorIsReturned(t *testing.T) {
	var target targetArgs
	expectedErr := errors.New("body decode failure")
	fromBody := func(body []byte, into model.Decodable) error {
		return expectedErr
	}
	err := router.DecodeArgs("POST", "", []byte("bad"), &target, fromBody)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("DecodeArgs error = %v, want %v", err, expectedErr)
	}
}

func TestQuery_EscapeReservedBytes(t *testing.T) {
	s := "a&b=c d%é"
	escaped := router.QueryEscape(s)
	want := "a%26b%3Dc%20d%25%C3%A9"
	if escaped != want {
		t.Fatalf("QueryEscape(%q) = %q, want %q", s, escaped, want)
	}
	unescaped := router.QueryUnescape(escaped)
	if unescaped != s {
		t.Fatalf("QueryUnescape(%q) = %q, want %q", escaped, unescaped, s)
	}
}
