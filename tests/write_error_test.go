package router_test

import (
	"errors"
	"strings"
	"testing"

	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/router/mock"
)

func TestWriteError_InvalidIs400WithField(t *testing.T) {
	ctx := &mock.Context{}
	err := &model.FieldError{Field: "name", Err: errors.New("name required")}

	router.WriteError(ctx, err)

	if ctx.Status != 400 {
		t.Fatalf("expected status 400, got %d", ctx.Status)
	}
	if ct := ctx.GetHeader("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("expected Content-Type application/problem+json, got %q", ct)
	}
	body := string(ctx.ResponseBody())
	if !strings.Contains(body, `"status":400`) {
		t.Errorf("body missing status 400: %s", body)
	}
	if !strings.Contains(body, `"title":"invalid"`) {
		t.Errorf("body missing title invalid: %s", body)
	}
	if !strings.Contains(body, `"detail":"name required"`) {
		t.Errorf("body missing detail: %s", body)
	}
	if !strings.Contains(body, `"field":"name"`) {
		t.Errorf("body missing field name: %s", body)
	}
}

func TestWriteError_InternalHidesText(t *testing.T) {
	ctx := &mock.Context{}
	err := errors.New("db down")

	router.WriteError(ctx, err)

	if ctx.Status != 500 {
		t.Fatalf("expected status 500, got %d", ctx.Status)
	}
	body := string(ctx.ResponseBody())
	if strings.Contains(body, "db down") {
		t.Errorf("body leaked internal error text: %s", body)
	}
	if !strings.Contains(body, `"detail":"internal error"`) {
		t.Errorf("body missing detail internal error: %s", body)
	}
	if got := router.InternalError(ctx); got != "db down" {
		t.Errorf("expected InternalError 'db down', got %q", got)
	}
}

func TestWriteError_NotFound404(t *testing.T) {
	ctx := &mock.Context{}
	err := model.NotFound("service not found")

	router.WriteError(ctx, err)

	if ctx.Status != 404 {
		t.Fatalf("expected status 404, got %d", ctx.Status)
	}
	body := string(ctx.ResponseBody())
	if !strings.Contains(body, `"status":404`) {
		t.Errorf("body missing status 404: %s", body)
	}
	if !strings.Contains(body, `"title":"not found"`) {
		t.Errorf("body missing title not found: %s", body)
	}
	if !strings.Contains(body, `"detail":"service not found"`) {
		t.Errorf("body missing detail: %s", body)
	}
	if router.InternalError(ctx) != "" {
		t.Errorf("expected empty InternalError, got %q", router.InternalError(ctx))
	}
}

func TestWriteError_Conflict409(t *testing.T) {
	ctx := &mock.Context{}
	err := model.Conflict("in use")

	router.WriteError(ctx, err)

	if ctx.Status != 409 {
		t.Fatalf("expected status 409, got %d", ctx.Status)
	}
	body := string(ctx.ResponseBody())
	if !strings.Contains(body, `"status":409`) {
		t.Errorf("body missing status 409: %s", body)
	}
	if !strings.Contains(body, `"title":"conflict"`) {
		t.Errorf("body missing title conflict: %s", body)
	}
	if !strings.Contains(body, `"detail":"in use"`) {
		t.Errorf("body missing detail: %s", body)
	}
}
