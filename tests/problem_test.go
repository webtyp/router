package router_test

import (
	"testing"

	"webtyp.com/model"
	"webtyp.com/router"
)

// One table for both directions and every transport: the status an error kind
// travels as, and the kind a status carries back.
func TestStatusOf_KindOfStatus_RoundTrip(t *testing.T) {
	for _, k := range []model.ErrorKind{model.ErrorInvalid, model.ErrorNotFound, model.ErrorConflict, model.ErrorInternal, model.ErrorUnreachable} {
		if got := router.KindOfStatus(router.StatusOf(k)); got != k {
			t.Errorf("KindOfStatus(StatusOf(%v)) = %v", k, got)
		}
	}
	if router.StatusOf(model.ErrorInvalid) != 400 || router.StatusOf(model.ErrorNotFound) != 404 ||
		router.StatusOf(model.ErrorConflict) != 409 || router.StatusOf(model.ErrorInternal) != 500 {
		t.Error("StatusOf must keep 400/404/409/500")
	}
	if router.KindOfStatus(418) != model.ErrorInternal {
		t.Error("an unknown status is internal")
	}
	for _, status := range []int{0, 502, 503, 504} {
		if router.KindOfStatus(status) != model.ErrorUnreachable {
			t.Errorf("status %d: no answer or a server down must be unreachable", status)
		}
	}
}
