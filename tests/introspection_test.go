package router_test

import (
	"testing"

	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/router/mock"
)

type dummyPolicy struct {
	grants []model.RoleGrant
}

func (p dummyPolicy) Grants() []model.RoleGrant {
	return p.grants
}

type dummyArgs struct{ Name string }

func (a *dummyArgs) IsNil() bool                      { return a == nil }
func (a *dummyArgs) Schema() []model.Field            { return []model.Field{{Name: "name", NotNull: true}} }
func (a *dummyArgs) Pointers() []any                  { return []any{&a.Name} }
func (a *dummyArgs) EncodeFields(w model.FieldWriter) { w.String("name", a.Name) }
func (a *dummyArgs) DecodeFields(r model.FieldReader) {}

var _ model.Fielder = (*dummyArgs)(nil)

func TestMountIntrospection(t *testing.T) {
	t.Run("nil policy sets policy_known to false", func(t *testing.T) {
		r := &mock.Router{}
		r.Get("/api/test", func(ctx router.Context) {}).Requires("test", model.Read)
		router.MountIntrospection(r, router.IntrospectionPath, nil).Public()

		ctx := &mock.Context{InMethod: "GET", InPath: router.IntrospectionPath}
		r.Invoke("GET", router.IntrospectionPath, ctx)

		if ctx.Status != 200 {
			t.Fatalf("expected status 200, got %d", ctx.Status)
		}
		body := string(ctx.ResponseBody())
		if !contains(body, `"policy_known":false`) {
			t.Errorf("expected policy_known:false in response, got %s", body)
		}
	})

	t.Run("policy with no grants reports empty roles and policy_known true", func(t *testing.T) {
		r := &mock.Router{}
		r.Get("/api/test", func(ctx router.Context) {}).Requires("test", model.Read)
		policy := dummyPolicy{grants: nil}
		router.MountIntrospection(r, router.IntrospectionPath, policy).Public()

		ctx := &mock.Context{InMethod: "GET", InPath: router.IntrospectionPath}
		r.Invoke("GET", router.IntrospectionPath, ctx)

		if ctx.Status != 200 {
			t.Fatalf("expected status 200, got %d", ctx.Status)
		}
		body := string(ctx.ResponseBody())
		if !contains(body, `"policy_known":true`) || !contains(body, `"roles":[]`) {
			t.Errorf("expected policy_known:true and roles:[], got %s", body)
		}
	})

	t.Run("policy with grants reports matching role", func(t *testing.T) {
		r := &mock.Router{}
		r.Get("/api/test", func(ctx router.Context) {}).Requires("test", model.Read)
		policy := dummyPolicy{
			grants: []model.RoleGrant{
				{Role: "admin", Grant: model.Grant{Resource: "test", Actions: model.Read}},
			},
		}
		router.MountIntrospection(r, router.IntrospectionPath, policy).Public()

		ctx := &mock.Context{InMethod: "GET", InPath: router.IntrospectionPath}
		r.Invoke("GET", router.IntrospectionPath, ctx)

		if ctx.Status != 200 {
			t.Fatalf("expected status 200, got %d", ctx.Status)
		}
		body := string(ctx.ResponseBody())
		if !contains(body, `"roles":["admin"]`) {
			t.Errorf("expected roles:[\"admin\"], got %s", body)
		}
	})

	t.Run("public route reports access public", func(t *testing.T) {
		r := &mock.Router{}
		r.Get("/public", func(ctx router.Context) {}).Public()
		router.MountIntrospection(r, router.IntrospectionPath, nil).Public()

		ctx := &mock.Context{InMethod: "GET", InPath: router.IntrospectionPath}
		r.Invoke("GET", router.IntrospectionPath, ctx)

		if ctx.Status != 200 {
			t.Fatalf("expected status 200, got %d", ctx.Status)
		}
		body := string(ctx.ResponseBody())
		if !contains(body, `"access":"public"`) {
			t.Errorf("expected access:public, got %s", body)
		}
	})

	t.Run("args serialization", func(t *testing.T) {
		r := &mock.Router{}
		r.Post("/with-args", func(ctx router.Context) {}).Public().Accepts(&dummyArgs{})
		router.MountIntrospection(r, router.IntrospectionPath, nil).Public()

		ctx := &mock.Context{InMethod: "GET", InPath: router.IntrospectionPath}
		r.Invoke("GET", router.IntrospectionPath, ctx)

		if ctx.Status != 200 {
			t.Fatalf("expected status 200, got %d", ctx.Status)
		}
		body := string(ctx.ResponseBody())
		if !contains(body, `"args":[{"name":"name","kind":"","required":true}]`) {
			t.Errorf("expected args field in body, got %s", body)
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstr(s, substr)
}

func searchSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
