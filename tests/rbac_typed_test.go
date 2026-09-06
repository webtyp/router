package router_test

import (
	"testing"

	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/router/mock"
)

// El vocabulario RBAC de una ruta es tipado, y eso compra dos garantías que un `string`
// pelado no daba:
//
//  1. Un recurso no es una acción. `Requires("read", "invoices")` —los argumentos al
//     revés— compilaba, y el fallo no era un error: era una DENEGACIÓN SILENCIOSA en
//     runtime, en el único sitio del sistema donde el silencio es inaceptable.
//  2. Los verbos son un conjunto CERRADO (CRUD). Antes se podía inventar uno ("write",
//     "raed") y nadie lo enforzaba nunca.
//
// Ninguna de las dos cosas compila hoy. Este test documenta la garantía y fija lo que sí
// queda comprobable en runtime.
func TestRouteRBACIsTyped(t *testing.T) {
	r := &mock.Router{}

	r.Post("/orders", func(ctx router.Context) {}).Requires("orders", model.Update)

	route := r.Routes()[0]
	if route.Resource != model.Resource("orders") {
		t.Errorf("Resource = %q; se esperaba orders", route.Resource)
	}
	if route.Action != model.Update {
		t.Errorf("Action = %d; se esperaba model.Update", route.Action)
	}
	if route.IsPublic() {
		t.Error("una ruta con Requires no puede ser pública")
	}
}

// Los tres estados de acceso son UNA declaración, no una combinación de banderas.
//
// Antes se codificaban por ausencia: `Public bool` junto a un `Resource` vacío o no. Eso
// hacía escribible un estado ilegal —`.Public().Requires(...)`— donde la verja se quedaba
// con `Public` y **descartaba el permiso en silencio**: una ruta que parecía protegida y no
// lo estaba. Un valor declarado no puede contradecirse a sí mismo.
func TestAccessIsOneDeclaration(t *testing.T) {
	r := &mock.Router{}

	r.Get("/assets", func(ctx router.Context) {}).Public()
	r.Get("/me", func(ctx router.Context) {}).Authenticated()
	r.Get("/orders", func(ctx router.Context) {}).Requires("orders", model.Read)

	want := []model.Access{model.AccessPublic, model.AccessAuthenticated, model.AccessGuarded}
	for i, route := range r.Routes() {
		if route.Access != want[i] {
			t.Errorf("%s: Access = %d; se esperaba %d", route.Path, route.Access, want[i])
		}
	}
}

// La otra mitad del contrato, que no se toca: lo que no se declara, queda cerrado.
func TestUnannotatedRouteStaysPrivate(t *testing.T) {
	r := &mock.Router{}

	r.Get("/api/orders", func(ctx router.Context) {})

	route := r.Routes()[0]
	// El zero value es AccessGuarded: exige identidad Y permiso. Algo que no declara nada
	// queda inalcanzable — no abierto a cualquiera que resulte estar logueado.
	if route.Access != model.AccessGuarded {
		t.Errorf("Access = %d; el default debe ser AccessGuarded", route.Access)
	}
	if route.IsPublic() {
		t.Error("un Get sin anotar NO puede ser público: el default es negar")
	}
	if route.Resource != "" {
		t.Errorf("Resource = %q; una ruta sin anotar no declara recurso", route.Resource)
	}
	// El zero value de Action no concede nada: no es "todas", es "ninguna".
	if route.Action != 0 {
		t.Errorf("Action = %d; una ruta sin anotar no declara acción", route.Action)
	}
}
