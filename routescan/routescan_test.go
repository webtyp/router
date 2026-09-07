//go:build !wasm

// routescan reads the host filesystem, so its tests run on stdlib only: under
// WASM there is no routes/routes.go to read.
package routescan

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func writeRoutes(t *testing.T, src string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "routes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "routes", "routes.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// lineOf returns the 1-based line of the first line containing substr.
func lineOf(src, substr string) int {
	for i, line := range strings.Split(src, "\n") {
		if strings.Contains(line, substr) {
			return i + 1
		}
	}
	return -1
}

func pathErr(line int) string {
	return DefaultFile + ":" + strconv.Itoa(line) + ": " + ErrPathNotLiteral
}

// TestScanMethods covers every recognised call, asserting Method, Path and Line.
func TestScanMethods(t *testing.T) {
	src := `package routes

import "webtyp.com/router"

func Register(r router.Router) {
	r.Get("/get", h)
	r.Post("/post", h)
	r.Put("/put", h)
	r.Delete("/delete", h)
	r.Options("/options", h)
	r.Stream("/stream", h)
	r.Socket("/socket", h)
	r.PublicAsset("/asset.js", h)
	r.PublicDir("/static", "web/public")
	r.Handle("PATCH", "/patch", h)
	r.Mount("/api/auth", auth.Routes)
}
`
	decls, err := Scan(writeRoutes(t, src))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	want := []Decl{
		{Method: "GET", Path: "/get", Line: lineOf(src, `"/get"`)},
		{Method: "POST", Path: "/post", Line: lineOf(src, `"/post"`)},
		{Method: "PUT", Path: "/put", Line: lineOf(src, `"/put"`)},
		{Method: "DELETE", Path: "/delete", Line: lineOf(src, `"/delete"`)},
		{Method: "OPTIONS", Path: "/options", Line: lineOf(src, `"/options"`)},
		{Method: "STREAM", Path: "/stream", Line: lineOf(src, `"/stream"`)},
		{Method: "SOCKET", Path: "/socket", Line: lineOf(src, `"/socket"`)},
		{Method: "GET", Path: "/asset.js", Line: lineOf(src, `"/asset.js"`)},
		{Method: "GET", Path: "/static*", Line: lineOf(src, `"/static"`)},
		{Method: "PATCH", Path: "/patch", Line: lineOf(src, `"/patch"`)},
		{Method: "MOUNT", Path: "/api/auth*", Line: lineOf(src, `r.Mount`)},
	}
	if len(decls) != len(want) {
		t.Fatalf("Scan returned %d decls, want %d: %+v", len(decls), len(want), decls)
	}
	for i, w := range want {
		if decls[i] != w {
			t.Errorf("decl %d = %+v, want %+v", i, decls[i], w)
		}
	}
}

// TestScanPublicDirIsPrefix: PublicDir declares a directory subtree, so its
// path carries the MountSuffix — an adjacent exact Get on the same-shaped path
// stays a leaf. Regression proof: against a routescan that treats PublicDir as
// a plain GET, both come back as bare paths.
func TestScanPublicDirIsPrefix(t *testing.T) {
	src := `package routes

import "webtyp.com/router"

func Register(r router.Router) {
	r.Get("/static", h)
	r.PublicDir("/assets", "web/public")
}
`
	decls, err := Scan(writeRoutes(t, src))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	want := []Decl{
		{Method: "GET", Path: "/static", Line: lineOf(src, `"/static"`)},
		{Method: "GET", Path: "/assets*", Line: lineOf(src, `r.PublicDir`)},
	}
	if len(decls) != len(want) {
		t.Fatalf("Scan returned %+v, want %+v", decls, want)
	}
	for i, w := range want {
		if decls[i] != w {
			t.Errorf("decl %d = %+v, want %+v", i, decls[i], w)
		}
	}
}

// TestScanPublicDirNonLiteralPrefix: a PublicDir prefix that is not a literal
// or an in-file const fails with the same verbatim path error as every other
// selector.
func TestScanPublicDirNonLiteralPrefix(t *testing.T) {
	src := `package routes

import "webtyp.com/router"

func Register(r router.Router) {
	p := "/assets"
	r.PublicDir(p, "web/public")
}
`
	_, err := Scan(writeRoutes(t, src))
	if err == nil {
		t.Fatal("Scan succeeded, want path error for non-literal PublicDir prefix")
	}
	if want := pathErr(lineOf(src, "r.PublicDir(")); err.Error() != want {
		t.Errorf("Scan error = %q, want %q", err.Error(), want)
	}
}

// TestScanChainedCalls: trailing chains carry no path and parse identically.
func TestScanChainedCalls(t *testing.T) {
	src := `package routes

import "webtyp.com/router"

func Register(r router.Router) {
	r.Get("/x", h).Public()
	r.Post("/y", h).Requires("items", model.Read)
}
`
	decls, err := Scan(writeRoutes(t, src))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	want := []Decl{
		{Method: "GET", Path: "/x", Line: lineOf(src, `"/x"`)},
		{Method: "POST", Path: "/y", Line: lineOf(src, `"/y"`)},
	}
	if len(decls) != len(want) {
		t.Fatalf("Scan returned %+v, want %+v", decls, want)
	}
	for i, w := range want {
		if decls[i] != w {
			t.Errorf("decl %d = %+v, want %+v", i, decls[i], w)
		}
	}
}

// TestScanParamName: the parameter may be named anything; the method resolves
// through the call expression, never by matching text.
func TestScanParamName(t *testing.T) {
	src := `package routes

import "webtyp.com/router"

func Register(mux router.Router) {
	mux.Get("/x", h).Public()
}
`
	decls, err := Scan(writeRoutes(t, src))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	want := []Decl{{Method: "GET", Path: "/x", Line: lineOf(src, `"/x"`)}}
	if len(decls) != 1 || decls[0] != want[0] {
		t.Fatalf("Scan returned %+v, want %+v", decls, want)
	}
}

// TestScanAliasedImport: router.Router is identified through the import block,
// so an aliased import still matches.
func TestScanAliasedImport(t *testing.T) {
	src := `package routes

import rt "webtyp.com/router"

func Register(r rt.Router) {
	r.Get("/x", h).Public()
}
`
	decls, err := Scan(writeRoutes(t, src))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	want := []Decl{{Method: "GET", Path: "/x", Line: lineOf(src, `"/x"`)}}
	if len(decls) != 1 || decls[0] != want[0] {
		t.Fatalf("Scan returned %+v, want %+v", decls, want)
	}
}

// TestScanConstPath: a path bound to a const in the same file resolves.
func TestScanConstPath(t *testing.T) {
	src := `package routes

import "webtyp.com/router"

const ContactPath = "/api/contacto"

func Register(r router.Router) {
	r.Post(ContactPath, h).Public()
}
`
	decls, err := Scan(writeRoutes(t, src))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	want := []Decl{{Method: "POST", Path: "/api/contacto", Line: lineOf(src, `ContactPath, h`)}}
	if len(decls) != 1 || decls[0] != want[0] {
		t.Fatalf("Scan returned %+v, want %+v", decls, want)
	}
}

// TestScanPathErrors: a variable or a concatenation is unreadable to build
// tooling and fails with the verbatim error at the call's line.
func TestScanPathErrors(t *testing.T) {
	for _, src := range []string{
		`package routes

import "webtyp.com/router"

func Register(r router.Router) {
	path := "/x"
	r.Get(path, h).Public()
}
`,
		`package routes

import "webtyp.com/router"

func Register(r router.Router) {
	r.Get("/a"+"/b", h).Public()
}
`,
	} {
		_, err := Scan(writeRoutes(t, src))
		if err == nil {
			t.Fatalf("Scan(%q) succeeded, want path error", src)
		}
		line := lineOf(src, "r.Get(")
		if want := pathErr(line); err.Error() != want {
			t.Errorf("Scan error = %q, want %q", err.Error(), want)
		}
	}
}

// TestScanMissingFile: a project without routes is legal, not an error.
func TestScanMissingFile(t *testing.T) {
	decls, err := Scan(t.TempDir())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(decls) != 0 {
		t.Fatalf("Scan returned %+v, want empty", decls)
	}
}

// TestScanNoRouterFunc: a file with no router.Router function fails verbatim.
func TestScanNoRouterFunc(t *testing.T) {
	src := `package routes

func Helper() int { return 1 }
`
	_, err := Scan(writeRoutes(t, src))
	if err == nil {
		t.Fatal("Scan succeeded, want no-router-func error")
	}
	if want := DefaultFile + ": " + ErrNoRouterFunc; err.Error() != want {
		t.Errorf("Scan error = %q, want %q", err.Error(), want)
	}
}

// TestScanIgnoresHelpers: calls in a function that does not take
// router.Router declare no routes.
func TestScanIgnoresHelpers(t *testing.T) {
	src := `package routes

import "webtyp.com/router"

func Register(r router.Router) {
	r.Get("/kept", h).Public()
}

func Helper(m mux.Router) {
	m.Get("/ignored", h)
}
`
	decls, err := Scan(writeRoutes(t, src))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	want := []Decl{{Method: "GET", Path: "/kept", Line: lineOf(src, `"/kept"`)}}
	if len(decls) != 1 || decls[0] != want[0] {
		t.Fatalf("Scan returned %+v, want %+v", decls, want)
	}
}

// TestScanUnparseable: a file that does not parse wraps the parser error.
func TestScanUnparseable(t *testing.T) {
	src := `package routes

func Register( {
`
	_, err := Scan(writeRoutes(t, src))
	if err == nil {
		t.Fatal("Scan succeeded, want parse error")
	}
	if !strings.HasPrefix(err.Error(), ErrParsePrefix) {
		t.Errorf("Scan error = %q, want prefix %q", err.Error(), ErrParsePrefix)
	}
}
