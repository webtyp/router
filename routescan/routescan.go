// Package routescan reads the application's route manifest without running it.
//
// Build tools — goflare when it deploys, sitec when it compiles — need the
// method and path of every route, and a reusable module's routes cannot be
// listed in the application's file: their paths come from another package's
// constants and which routes exist is decided at runtime. So Scan reads the
// one thing the application does declare — routes/routes.go plus the Mount
// prefixes it hands to modules — and reports every declaration in source order.
//
// This package is BUILD TOOLING, not WASM code. It uses go/ast, go/parser and
// go/token from the standard library, which is correct and deliberate: do not
// "fix" those imports, and do not move this code into the package root — the
// root must stay WASM-safe.
package routescan

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
)

// DefaultFile is the path, relative to the project root, that Scan reads.
const DefaultFile = "routes/routes.go"

// ErrPathNotLiteral is returned when a route path is neither a string literal
// nor an identifier bound to a const declared in the same file. A path a build
// tool cannot read must not build, so there is no fallback that skips it.
const ErrPathNotLiteral = "route path must be a string literal or a const declared in this file"

// ErrNoRouterFunc is returned when no function in the file takes router.Router
// as its first parameter.
const ErrNoRouterFunc = "no function takes router.Router as its first parameter"

// ErrParsePrefix prefixes every error that wraps the go/parser failure.
const ErrParsePrefix = "routescan: "

// RouterImportPath is the import path Scan recognises as the routing contract.
// It is resolved through the file's import block, never by literal selector
// text, so an aliased import still matches.
const RouterImportPath = "webtyp.com/router"

// RouterTypeName is the type name Scan looks for on that import.
const RouterTypeName = "Router"

// DefaultRouterName is the local name of the routing contract when the file
// imports it without an explicit alias.
const DefaultRouterName = "router"

// MountSuffix marks a Mount declaration's path as a prefix, not a leaf:
// run_worker_first takes prefixes, and that is all build tooling needs.
const MountSuffix = "*"

// Route selector names as written on the Router receiver.
const (
	MethodGet         = "Get"
	MethodPost        = "Post"
	MethodPut         = "Put"
	MethodDelete      = "Delete"
	MethodOptions     = "Options"
	MethodStream      = "Stream"
	MethodSocket      = "Socket"
	MethodPublicAsset = "PublicAsset"
	MethodPublicDir   = "PublicDir"
	MethodHandle      = "Handle"
	MethodMount       = "Mount"
)

// Decl.Method values reported for each selector.
const (
	VerbGet     = "GET"
	VerbPost    = "POST"
	VerbPut     = "PUT"
	VerbDelete  = "DELETE"
	VerbOptions = "OPTIONS"
	VerbStream  = "STREAM"
	VerbSocket  = "SOCKET"
	VerbMount   = "MOUNT"
)

// methodOf maps a Router selector to the Decl.Method it reports. Handle and
// Mount are absent: their method is read from the call's own arguments.
var methodOf = map[string]string{
	MethodGet:         VerbGet,
	MethodPost:        VerbPost,
	MethodPut:         VerbPut,
	MethodDelete:      VerbDelete,
	MethodOptions:     VerbOptions,
	MethodStream:      VerbStream,
	MethodSocket:      VerbSocket,
	MethodPublicAsset: VerbGet,
	MethodPublicDir:   VerbGet,
}

// Decl is one route declared in routes/routes.go.
type Decl struct {
	Method string // "GET", "POST", "PUT", "DELETE", "OPTIONS", "STREAM", "SOCKET", "MOUNT", or the literal passed to Handle
	Path   string // exactly as written: "/api/contacto" (a Mount path carries the MountSuffix)
	Line   int    // 1-based line in routes/routes.go, for error messages
}

// Scan parses <rootDir>/routes/routes.go and returns every route declared in it,
// in source order.
//
// It returns an empty slice and a nil error when the file does not exist: a
// project without routes is legal, not an error.
func Scan(rootDir string) ([]Decl, error) {
	src, err := os.ReadFile(filepath.Join(rootDir, DefaultFile))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf(ErrParsePrefix+"%w", err)
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, DefaultFile, src, 0)
	if err != nil {
		return nil, fmt.Errorf(ErrParsePrefix+"%w", err)
	}

	routerName := localRouterName(f)
	consts := fileConsts(f)

	var out []Decl
	found := false
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		param, ok := routerParam(fn, routerName)
		if !ok {
			continue
		}
		found = true
		if err := collectDecls(fset, fn.Body, param, consts, &out); err != nil {
			return nil, err
		}
	}
	if !found {
		return nil, errors.New(DefaultFile + ": " + ErrNoRouterFunc)
	}
	return out, nil
}

// localRouterName resolves the local identifier of the routing contract
// through the file's import block. "" when the file does not import it, in
// which case no function can take router.Router.
func localRouterName(f *ast.File) string {
	for _, imp := range f.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil || path != RouterImportPath {
			continue
		}
		if imp.Name != nil {
			return imp.Name.Name
		}
		return DefaultRouterName
	}
	return ""
}

// fileConsts maps every identifier bound to a string const in the file to its
// value. Only a const declared here makes a path readable to build tooling.
func fileConsts(f *ast.File) map[string]string {
	consts := map[string]string{}
	ast.Inspect(f, func(n ast.Node) bool {
		gen, ok := n.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			return true
		}
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, name := range vs.Names {
				if i >= len(vs.Values) {
					continue
				}
				lit, ok := vs.Values[i].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				if v, err := strconv.Unquote(lit.Value); err == nil {
					consts[name.Name] = v
				}
			}
		}
		return true
	})
	return consts
}

// routerParam reports the name of fn's first parameter when its type is the
// routing contract. A helper that takes anything else declares no routes.
func routerParam(fn *ast.FuncDecl, routerName string) (string, bool) {
	if routerName == "" || fn.Type.Params == nil || len(fn.Type.Params.List) == 0 {
		return "", false
	}
	field := fn.Type.Params.List[0]
	if len(field.Names) == 0 {
		return "", false
	}
	sel, ok := field.Type.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || pkg.Name != routerName || sel.Sel.Name != RouterTypeName {
		return "", false
	}
	return field.Names[0].Name, true
}

// collectDecls appends one Decl per route call on param inside body, in source
// order. Trailing chained calls carry no path: only a call whose receiver is
// the parameter's identifier directly is a declaration.
func collectDecls(fset *token.FileSet, body *ast.BlockStmt, param string, consts map[string]string, out *[]Decl) error {
	var scanErr error
	ast.Inspect(body, func(n ast.Node) bool {
		if scanErr != nil {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		recv, ok := sel.X.(*ast.Ident)
		if !ok || recv.Name != param {
			return true
		}
		line := fset.Position(call.Pos()).Line
		if sel.Sel.Name == MethodHandle {
			if len(call.Args) < 2 {
				return true
			}
			method, ok := resolveArg(call.Args[0], consts)
			if !ok {
				scanErr = pathError(line)
				return false
			}
			path, ok := resolveArg(call.Args[1], consts)
			if !ok {
				scanErr = pathError(line)
				return false
			}
			*out = append(*out, Decl{Method: method, Path: path, Line: line})
			return true
		}
		if sel.Sel.Name == MethodMount {
			if len(call.Args) < 1 {
				return true
			}
			prefix, ok := resolveArg(call.Args[0], consts)
			if !ok {
				scanErr = pathError(line)
				return false
			}
			*out = append(*out, Decl{Method: VerbMount, Path: prefix + MountSuffix, Line: line})
			return true
		}
		verb, ok := methodOf[sel.Sel.Name]
		if !ok || len(call.Args) < 1 {
			return true
		}
		path, ok := resolveArg(call.Args[0], consts)
		if !ok {
			scanErr = pathError(line)
			return false
		}
		*out = append(*out, Decl{Method: verb, Path: path, Line: line})
		return true
	})
	return scanErr
}

// resolveArg reads a path argument: a string literal, or an identifier bound
// to a const declared in the same file. Anything else is unreadable to build
// tooling and must not build.
func resolveArg(expr ast.Expr, consts map[string]string) (string, bool) {
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind != token.STRING {
			return "", false
		}
		v, err := strconv.Unquote(e.Value)
		if err != nil {
			return "", false
		}
		return v, true
	case *ast.Ident:
		v, ok := consts[e.Name]
		return v, ok
	default:
		return "", false
	}
}

// pathError reports where a route stopped being readable to build tooling.
func pathError(line int) error {
	return errors.New(DefaultFile + ":" + strconv.Itoa(line) + ": " + ErrPathNotLiteral)
}
