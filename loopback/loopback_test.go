package loopback

import (
	"testing"

	"webtyp.com/model"
	"webtyp.com/router"
)

type In struct {
	X string
}

func (i *In) EncodeFields(w model.FieldWriter) { w.String("x", i.X) }
func (i *In) DecodeFields(r model.FieldReader) { i.X, _ = r.String("x") }
func (i *In) IsNil() bool                      { return i == nil }

type Out struct {
	X string
}

func (o *Out) EncodeFields(w model.FieldWriter) { w.String("x", o.X) }
func (o *Out) DecodeFields(r model.FieldReader) { o.X, _ = r.String("x") }
func (o *Out) IsNil() bool                      { return o == nil }

// toy is an OperationModule that registers a couple of operations: echo (copy
// args→result) and fail (WriteStatus 404).
type toy struct{}

func (toy) ModelName() string { return "toy" }

func (toy) MountOperations(reg router.OperationRegistry) {
	reg.Operation("echo", func(ctx router.Context) {
		var a In
		if err := ctx.Decode(&a); err != nil {
			ctx.WriteStatus(400)
			return
		}
		_ = ctx.Encode(&Out{X: a.X})
	}).Requires("toy", model.Read).Accepts(nil)

	reg.Operation("fail", func(ctx router.Context) {
		ctx.WriteStatus(404)
		_, _ = ctx.Write([]byte("nope"))
	}).Requires("toy", model.Read).Accepts(nil)

	reg.Operation("touch", func(ctx router.Context) {
		ctx.SetValue("touched", "true")
		ctx.WriteStatus(200)
	}).Requires("toy", model.Create).Accepts(nil)
}

var _ router.OperationModule = toy{}

func TestCall_RoundTrips(t *testing.T) {
	out := &Out{}
	var got error
	New(toy{}).Call("echo", &In{X: "hi"}, out, func(err error) { got = err })
	if got != nil {
		t.Fatalf("done(err) = %v, want nil", got)
	}
	if out.X != "hi" {
		t.Errorf("Out.X = %q, want %q", out.X, "hi")
	}
}

func TestCall_UnknownOp(t *testing.T) {
	into := &Out{X: "keep"}
	var got error
	New(toy{}).Call("nope", &In{X: "x"}, into, func(err error) { got = err })
	if got == nil {
		t.Fatal("done(err) = nil, want non-nil error")
	}
	if into.X != "keep" {
		t.Errorf("into mutated on unknown op: X = %q, want keep", into.X)
	}
}

func TestCall_HandlerStatus4xx(t *testing.T) {
	var got error
	New(toy{}).Call("fail", &In{}, &Out{}, func(err error) { got = err })
	if got == nil {
		t.Fatal("done(err) = nil, want non-nil error")
	}
	if got.Error() != "nope" {
		t.Errorf("error message = %q, want body %q", got.Error(), "nope")
	}
}

func TestCall_NilInto(t *testing.T) {
	var got error
	New(toy{}).Call("touch", &In{}, nil, func(err error) { got = err })
	if got != nil {
		t.Fatalf("done(err) = %v, want nil", got)
	}
}

func TestDispatch_FireAndForget(t *testing.T) {
	var got error
	New(toy{}).Dispatch("echo", &In{X: "fire"})
	_ = got
	// Dispatch must not panic; a failing op must not break the caller either.
	New(toy{}).Dispatch("fail", &In{})
}

func TestMultiModule(t *testing.T) {
	// two more modules mounting distinct ops into the same registry
	type toyB struct{ toy }
	out := &Out{}
	var got error
	caller := New(toy{}, toyB{})
	caller.Call("echo", &In{X: "both"}, out, func(err error) { got = err })
	if got != nil {
		t.Fatalf("done(err) = %v, want nil", got)
	}
	if out.X != "both" {
		t.Errorf("Out.X = %q, want both", out.X)
	}
}
