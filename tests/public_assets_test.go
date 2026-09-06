package router_test

import (
	"testing"

	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/router/mock"
)

// El arnés: abrir un archivo al mundo es un acto tipado y explícito; cerrar es el
// default. Estos tests fijan las dos direcciones.
func TestPublicAssetIsPublicByConstruction(t *testing.T) {
	r := &mock.Router{}

	r.PublicAsset("/style.css", func(ctx router.Context) {})
	r.PublicAsset("/client.wasm", func(ctx router.Context) {})

	routes := r.Routes()
	if len(routes) != 2 {
		t.Fatalf("registradas %d rutas, se esperaban 2", len(routes))
	}
	for _, route := range routes {
		if !route.IsPublic() {
			t.Errorf("%q no es pública: el navegador recibiría 403 y la página saldría en blanco", route.Path)
		}
		if route.Resource != "" {
			t.Errorf("%q tiene RBAC (%q): un asset no debe poder quedar cerrado", route.Path, route.Resource)
		}
	}
}

func TestPublicDirIsPublicAndVisible(t *testing.T) {
	r := &mock.Router{}

	r.PublicDir("/", "web/public")

	routes := r.Routes()
	if len(routes) != 1 {
		t.Fatalf("registradas %d rutas, se esperaba 1", len(routes))
	}
	if !routes[0].IsPublic() {
		t.Error("el directorio servido debe ser público")
	}
	// Que el directorio sea visible en la introspección es el punto: antes se servía
	// con un FileServer colgado FUERA del router, invisible al sistema de permisos.
	if routes[0].Dir != "web/public" {
		t.Errorf("Dir = %q; se esperaba \"web/public\" — un directorio servido debe ser visible al router", routes[0].Dir)
	}
}

// La otra mitad del contrato: lo que no se declara, queda cerrado.
func TestGetIsPrivateByDefault(t *testing.T) {
	r := &mock.Router{}

	r.Get("/api/orders", func(ctx router.Context) {})

	route := r.Routes()[0]
	if route.IsPublic() {
		t.Error("un Get sin anotar NO puede ser público: el default es negar")
	}
	if route.Resource != "" {
		t.Error("un Get sin anotar no debe declarar RBAC")
	}
}

// Servir un archivo CON permisos ya está cubierto por la ruta normal, y falla cerrado
// si se olvida el Requires.
func TestGatedFileIsJustARouteWithRequires(t *testing.T) {
	r := &mock.Router{}

	r.Get("/invoice/:id", func(ctx router.Context) {}).Requires("invoices", model.Read)

	route := r.Routes()[0]
	if route.IsPublic() {
		t.Error("una ruta con Requires no puede ser pública")
	}
	if route.Resource != "invoices" || route.Action != model.Read {
		t.Errorf("RBAC = (%q,%q); se esperaba (invoices,read)", route.Resource, route.Action)
	}
}
