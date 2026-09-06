package router_test

import (
	"errors"
	"testing"

	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/router/mock"
)

// callerProbe is a minimal model.Decodable so a test can prove Caller decodes the
// response into a typed target — the call-side mirror of Context.Decode.
type callerProbe struct{ Value string }

func (p *callerProbe) IsNil() bool { return p == nil }
func (p *callerProbe) DecodeFields(r model.FieldReader) {
	if v, ok := r.String("value"); ok {
		p.Value = v
	}
}

func TestCallerContract(t *testing.T) {
	var _ router.Caller = (*mock.Caller)(nil)
}

func TestMockCaller_Call(t *testing.T) {
	m := &mock.Caller{
		CannedResult: []byte(`{"value":"ok"}`),
	}

	var called bool
	var got callerProbe
	m.Call("test_op", nil, &got, func(err error) {
		called = true
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	if !called {
		t.Error("done was not called")
	}
	if got.Value != "ok" {
		t.Errorf("expected the response decoded into the target (value=ok), got %q", got.Value)
	}
	if len(m.Calls) != 1 || m.Calls[0].Op != "test_op" {
		t.Fatalf("expected one recorded call to 'test_op', got %+v", m.Calls)
	}
}

// TestMockCaller_NilInto: a caller that does not care about the response (a
// save/delete) passes into=nil and still gets its error/nil back.
func TestMockCaller_NilInto(t *testing.T) {
	m := &mock.Caller{CannedResult: []byte(`{"value":"ignored"}`)}

	var called bool
	m.Call("save_op", nil, nil, func(err error) {
		called = true
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})
	if !called {
		t.Error("done was not called")
	}
}

func TestMockCaller_Dispatch(t *testing.T) {
	m := &mock.Caller{}
	m.Dispatch("fire_and_forget", nil)

	if len(m.Calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(m.Calls))
	}
	if m.Calls[0].Op != "fire_and_forget" {
		t.Errorf("expected op 'fire_and_forget', got %q", m.Calls[0].Op)
	}
}

func TestMockCaller_Error(t *testing.T) {
	errTest := errors.New("test error")
	m := &mock.Caller{CannedError: errTest}

	var called bool
	var got callerProbe
	m.Call("fail_op", nil, &got, func(err error) {
		called = true
		if err != errTest {
			t.Errorf("expected %v, got %v", errTest, err)
		}
	})

	if !called {
		t.Error("done was not called")
	}
}
