package router_test

import (
	"testing"
	"webtyp.com/router"
	"webtyp.com/router/mock"
)

func TestFromInternet_Unmarked(t *testing.T) {
	ctx := &mock.Context{}
	if got := router.FromInternet(ctx); !got {
		t.Errorf("FromInternet() = %v, want true", got)
	}
}

func TestFromInternet_Local(t *testing.T) {
	ctx := &mock.Context{}
	ctx.SetValue(router.ContextKeyNetwork, router.NetworkLocal)
	if got := router.FromInternet(ctx); got {
		t.Errorf("FromInternet() = %v, want false", got)
	}
}

func TestFromInternet_Internet(t *testing.T) {
	ctx := &mock.Context{}
	ctx.SetValue(router.ContextKeyNetwork, router.NetworkInternet)
	if got := router.FromInternet(ctx); !got {
		t.Errorf("FromInternet() = %v, want true", got)
	}
}

func TestFromInternet_Unknown(t *testing.T) {
	ctx := &mock.Context{}
	ctx.SetValue(router.ContextKeyNetwork, "lan")
	if got := router.FromInternet(ctx); !got {
		t.Errorf("FromInternet() = %v, want true", got)
	}
}
