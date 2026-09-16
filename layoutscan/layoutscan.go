// Package layoutscan verifies the structural layout of a webtyp application.
//
// This package is BUILD TOOLING, not WASM code. It uses go/ast, go/parser and
// go/token from the standard library, which is correct and deliberate: do not
// "fix" those imports, and do not move this code into the package root — the
// root must stay WASM-safe.
package layoutscan

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"webtyp.com/router/routescan"
)

// Violation is one broken rule, located. Rule is the stable identifier a test
// asserts against; Message is what a human reads.
type Violation struct {
	File    string // ruta relativa a rootDir
	Line    int    // 0 cuando la violación es del archivo entero, no de una línea
	Rule    string // una de las constantes Rule* de abajo
	Message string
}

// Rule constants exported for stable test assertions.
const (
	RuleForbiddenFileName  = "forbidden-file-name"
	// RuleModuleSubdirectory flags subdirectories inside a module directory (R2).
	// Exact directory name "docs" is exempt because module documentation lives
	// alongside what it documents.
	RuleModuleSubdirectory = "module-subdirectory"
	RuleMissingBuildTag    = "missing-build-tag"
	RuleUnexpectedBuildTag = "unexpected-build-tag"
	RuleTestOutsideTests   = "test-outside-tests"
	RuleUntypedResult      = "untyped-result"
	RuleRegisterArity      = "register-arity"
)

// VerifyLayout reports EVERY structural violation under rootDir.
//
// It returns all of them, never just the first: a caller fixing one rule at a
// time re-runs the whole build for each, and an agent given a single error
// fixes it and stops. The empty slice is a conforming project.
//
// It reads source; it never builds or runs anything.
func VerifyLayout(rootDir string) []Violation {
	var violations []Violation

	checkTestsOutsideTests(rootDir, &violations)
	checkModules(rootDir, &violations)
	checkRegisterArity(rootDir, &violations)

	return violations
}

// isIgnoredDir reports whether a path relative to rootDir is in an ignored directory.
func isIgnoredDir(rel string) bool {
	parts := strings.Split(rel, "/")
	if len(parts) == 0 {
		return false
	}
	switch parts[0] {
	case ".git", ".build", "vendor", "node_modules", "docs":
		return true
	case "web":
		return len(parts) >= 2 && parts[1] == "public"
	}
	return false
}

// checkTestsOutsideTests enforces R5 (RuleTestOutsideTests).
func checkTestsOutsideTests(rootDir string, violations *[]Violation) {
	_ = filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		relPath, err := filepath.Rel(rootDir, path)
		if err != nil {
			return nil
		}
		rel := filepath.ToSlash(relPath)
		if rel == "." {
			return nil
		}

		if info.IsDir() {
			if rel == "tests" || strings.HasPrefix(rel, "tests/") {
				return filepath.SkipDir
			}
			if isIgnoredDir(rel) {
				return filepath.SkipDir
			}
			return nil
		}

		if isIgnoredDir(rel) {
			return nil
		}

		if strings.HasSuffix(info.Name(), "_test.go") {
			*violations = append(*violations, Violation{
				File:    rel,
				Line:    0,
				Rule:    RuleTestOutsideTests,
				Message: fmt.Sprintf("%s: test files must reside inside tests/", rel),
			})
		}
		return nil
	})
}

// isForbiddenFileName reports whether a file name inside a module subdirectory is forbidden (R1).
func isForbiddenFileName(name string) bool {
	if name == "init.go" || name == "backend.go" || name == "view.go" || name == "frontend.go" {
		return true
	}
	if strings.HasSuffix(name, "_wasm.go") {
		return true
	}
	return false
}

// checkModules enforces R1, R2, R3, R4, R6 under modules/.
func checkModules(rootDir string, violations *[]Violation) {
	modulesDir := filepath.Join(rootDir, "modules")
	entries, err := os.ReadDir(modulesDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		fileName := entry.Name()
		fullPath := filepath.Join(modulesDir, fileName)
		relFile := "modules/" + fileName

		if !entry.IsDir() {
			if strings.HasSuffix(fileName, ".go") {
				checkModuleFile(rootDir, relFile, fullPath, false, violations)
			}
			continue
		}

		// entry is a module directory: modules/<m>/
		m := fileName
		mDir := fullPath
		mEntries, err := os.ReadDir(mDir)
		if err != nil {
			continue
		}

		for _, mEntry := range mEntries {
			subName := mEntry.Name()
			relSub := "modules/" + m + "/" + subName
			fullSub := filepath.Join(mDir, subName)

			if mEntry.IsDir() {
				// `docs` es la única excepción a la planitud de un módulo: la
				// razón de R2 es que un subdirectorio esconde código sin dueño
				// (un internal/, un paquete auxiliar, una copia local de algo
				// que debía estar aguas arriba). La documentación no es eso —
				// vive junto a lo que documenta y se mueve con ello. `data/`
				// NO se exime: datos junto al código sí son lo que R2 busca.
				if subName == "docs" {
					continue
				}
				// R2: RuleModuleSubdirectory
				*violations = append(*violations, Violation{
					File:    relSub,
					Line:    0,
					Rule:    RuleModuleSubdirectory,
					Message: fmt.Sprintf("%s: subdirectories inside module directory modules/%s are forbidden", relSub, m),
				})
				continue
			}

			// R1: RuleForbiddenFileName
			if isForbiddenFileName(subName) {
				msg := fmt.Sprintf("%s: `%s` is forbidden in module directory", relSub, subName)
				if subName == "init.go" {
					msg = fmt.Sprintf("%s: `init.go` is not a module file — split it into `server.go` (!wasm) and `browser.go` (neutral)", relSub)
				}
				*violations = append(*violations, Violation{
					File:    relSub,
					Line:    0,
					Rule:    RuleForbiddenFileName,
					Message: msg,
				})
			}

			if strings.HasSuffix(subName, ".go") {
				checkModuleFile(rootDir, relSub, fullSub, true, violations)
			}
		}
	}
}

// checkModuleFile inspects a Go file in modules/ or modules/<m>/ for R3, R4, R6.
func checkModuleFile(rootDir, relFile, fullPath string, inSubdir bool, violations *[]Violation) {
	baseName := filepath.Base(relFile)

	mustHaveNotWasm := false
	mustHaveNoBuildTag := false

	if baseName == "server.go" || (inSubdir && (baseName == "svg.go" || baseName == "css.go")) {
		mustHaveNotWasm = true
	} else if baseName == "browser.go" || (inSubdir && baseName == "module.go") {
		mustHaveNoBuildTag = true
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, fullPath, nil, parser.ParseComments)
	if err != nil {
		rule := RuleUntypedResult
		if mustHaveNotWasm {
			rule = RuleMissingBuildTag
		} else if mustHaveNoBuildTag {
			rule = RuleUnexpectedBuildTag
		}
		*violations = append(*violations, Violation{
			File:    relFile,
			Line:    0,
			Rule:    rule,
			Message: err.Error(),
		})
		return
	}

	headerComments := parseHeaderComments(fset, f)

	if mustHaveNotWasm {
		hasNotWasm := false
		for _, line := range headerComments {
			if (strings.HasPrefix(line, "//go:build") || strings.HasPrefix(line, "/*go:build")) && strings.Contains(line, "!wasm") {
				hasNotWasm = true
				break
			}
		}
		if !hasNotWasm {
			*violations = append(*violations, Violation{
				File:    relFile,
				Line:    0,
				Rule:    RuleMissingBuildTag,
				Message: fmt.Sprintf("%s: server.go, svg.go and css.go must carry `//go:build !wasm`", relFile),
			})
		}
	}

	if mustHaveNoBuildTag {
		hasBuildTag := false
		for _, line := range headerComments {
			if strings.HasPrefix(line, "//go:build") || strings.HasPrefix(line, "/*go:build") {
				hasBuildTag = true
				break
			}
		}
		if hasBuildTag {
			msg := fmt.Sprintf("%s: must carry NO build tag", relFile)
			if baseName == "browser.go" {
				msg = fmt.Sprintf("%s: browser.go must carry NO build tag — a `wasm` tag silently drops the stdlib view-test rail", relFile)
			}
			*violations = append(*violations, Violation{
				File:    relFile,
				Line:    0,
				Rule:    RuleUnexpectedBuildTag,
				Message: msg,
			})
		}
	}

	// R6: RuleUntypedResult
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Type == nil || fn.Type.Results == nil {
			continue
		}
		for _, field := range fn.Type.Results.List {
			if isUntypedArray(field.Type) {
				line := fset.Position(fn.Pos()).Line
				*violations = append(*violations, Violation{
					File:    relFile,
					Line:    line,
					Rule:    RuleUntypedResult,
					Message: fmt.Sprintf("%s:%d: function %s returns []any or []interface{}", relFile, line, fn.Name.Name),
				})
			}
		}
	}
}

// parseHeaderComments extracts comment lines appearing before the package clause.
func parseHeaderComments(fset *token.FileSet, f *ast.File) []string {
	var lines []string
	pkgLine := fset.Position(f.Package).Line
	for _, cg := range f.Comments {
		if fset.Position(cg.Pos()).Line >= pkgLine {
			continue
		}
		for _, c := range cg.List {
			for _, l := range strings.Split(c.Text, "\n") {
				l = strings.TrimSpace(l)
				lines = append(lines, l)
			}
		}
	}
	return lines
}

// isUntypedArray reports whether expr represents []any or []interface{}.
func isUntypedArray(expr ast.Expr) bool {
	arr, ok := expr.(*ast.ArrayType)
	if !ok || arr.Len != nil {
		return false
	}
	switch elt := arr.Elt.(type) {
	case *ast.Ident:
		return elt.Name == "any"
	case *ast.InterfaceType:
		return elt.Methods == nil || len(elt.Methods.List) == 0
	}
	return false
}

// checkRegisterArity enforces R7 (RuleRegisterArity).
func checkRegisterArity(rootDir string, violations *[]Violation) {
	// Applies only when web/server.go does NOT exist
	webServerPath := filepath.Join(rootDir, "web", "server.go")
	if _, err := os.Stat(webServerPath); err == nil {
		return
	}

	routesPath := filepath.Join(rootDir, "routes", "routes.go")
	if _, err := os.Stat(routesPath); err != nil {
		return
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, routesPath, nil, 0)
	if err != nil {
		*violations = append(*violations, Violation{
			File:    "routes/routes.go",
			Line:    0,
			Rule:    RuleRegisterArity,
			Message: err.Error(),
		})
		return
	}

	routerName := routescan.LocalRouterName(f)

	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Type == nil {
			continue
		}
		_, isRouterParam := routescan.RouterParam(fn, routerName)
		if isRouterParam || fn.Name.Name == "Register" {
			numParams := countParams(fn.Type.Params)
			if numParams != 1 {
				line := fset.Position(fn.Pos()).Line
				*violations = append(*violations, Violation{
					File:    "routes/routes.go",
					Line:    line,
					Rule:    RuleRegisterArity,
					Message: fmt.Sprintf("routes/routes.go:%d: Register function must take exactly one parameter (router.Router)", line),
				})
			}
		}
	}
}

// countParams counts total parameter variables in a function parameter list.
func countParams(params *ast.FieldList) int {
	if params == nil {
		return 0
	}
	count := 0
	for _, field := range params.List {
		if len(field.Names) == 0 {
			count++
		} else {
			count += len(field.Names)
		}
	}
	return count
}
